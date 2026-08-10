package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"

	"pharmacy/internal/config"
	"pharmacy/internal/db"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var categories = []struct {
	Name string
	Slug string
}{
	{"Pain Relief", "pain-relief"},
	{"Antibiotics", "antibiotics"},
	{"Vitamins & Supplements", "vitamins"},
	{"First Aid", "first-aid"},
	{"Skincare", "skincare"},
	{"Baby Care", "baby-care"},
}

// Single category backlink for the existing products table check
type productSeed struct {
	Name                string
	Brand               string
	Category            string
	Description         string
	ActiveIngredient    string
	RequiresPrescription bool
	Image               string // optional; derived from category when empty
	Variants            []variantSeed
}

type variantSeed struct {
	Strength   string
	PackSize   string
	PriceCents int32
	StockQty   int32
}

// Unsplash photo pools per category (stable CDN URLs). Any 404s fall back to
// the local placeholder via the onerror handler on <img> tags.
var catImages = map[string][]string{
	"pain-relief": {
		"https://images.unsplash.com/photo-1550572017-edd951b55104?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1584308666744-24d5c474f2ae?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1587854692152-cbe660dbde88?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1512069772995-ec65ed45afd6?auto=format&fit=crop&w=600&q=80",
	},
	"antibiotics": {
		"https://images.unsplash.com/photo-1471864190281-a93a3070b6de?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1579684385127-1ef15d508118?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1607619056574-7b8d3ee536b2?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1554475901-4538ddfbccc2?auto=format&fit=crop&w=600&q=80",
	},
	"vitamins": {
		"https://images.unsplash.com/photo-1584308666744-24d5c474f2ae?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1532187863486-abf9dbad1b69?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1576308111318-37d5e3a2d2a2?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1586971571630-4a1cf38a3263?auto=format&fit=crop&w=600&q=80",
	},
	"first-aid": {
		"https://images.unsplash.com/photo-1603398938378-e54eab446dde?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1615361200141-f45040f367be?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1583947581281-3e9c18d2e04a?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1584515933487-779824d29309?auto=format&fit=crop&w=600&q=80",
	},
	"skincare": {
		"https://images.unsplash.com/photo-1556228720-195a672e8a03?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1576602976047-174e57a47881?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1522335789203-aabd1fc54bc9?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1585487000160-6ebcfceb0d03?auto=format&fit=crop&w=600&q=80",
	},
	"baby-care": {
		"https://images.unsplash.com/photo-1544816155-12df9643f363?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1596464716127-f2a82984de30?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1562408590-e32931084e23?auto=format&fit=crop&w=600&q=80",
		"https://images.unsplash.com/photo-1519689680058-324335c77eba?auto=format&fit=crop&w=600&q=80",
	},
}

func imageFor(category string, index int) string {
	pool := catImages[category]
	if len(pool) == 0 {
		return ""
	}
	return pool[index%len(pool)]
}

func main() {
	reset := flag.Bool("reset", false, "wipe existing catalog data before seeding")
	flag.Parse()

	cfg := config.Load()
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer database.Close()

	// Check if products already exist
	var count int
	database.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)

	if count > 0 && !*reset {
		log.Printf("Products already seeded (%d). Use -reset to wipe and re-seed.", count)
		return
	}

	if *reset {
		log.Println("Resetting catalog data...")
		resetCatalog(database)
	}

	// Seed categories
	catIDs := make(map[string]int64)
	for _, c := range categories {
		var id int64
		err := database.QueryRow(
			"INSERT INTO categories (name, slug) VALUES ($1, $2) ON CONFLICT (slug) DO UPDATE SET name = $1 RETURNING id",
			c.Name, c.Slug,
		).Scan(&id)
		if err != nil {
			log.Fatalf("insert category %s: %v", c.Name, err)
		}
		catIDs[c.Slug] = id
	}

	products := generateProducts()

	for i, p := range products {
		slug := strings.ToLower(strings.ReplaceAll(p.Name, " ", "-"))
		slug = strings.ReplaceAll(slug, "/", "-")
		slug = strings.ReplaceAll(slug, "(", "")
		slug = strings.ReplaceAll(slug, ")", "")

		catID := catIDs[p.Category]

		imageKey := p.Image
		if imageKey == "" {
			imageKey = imageFor(p.Category, i)
		}

		var productID int64
		err := database.QueryRow(`
			INSERT INTO products (slug, name, brand, category_id, description, active_ingredient, requires_prescription, primary_image_key)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id
		`, slug, p.Name, p.Brand, catID, p.Description, p.ActiveIngredient, p.RequiresPrescription, imageKey).Scan(&productID)
		if err != nil {
			log.Printf("insert product %s: %v", p.Name, err)
			continue
		}

		for vi, v := range p.Variants {
			sku := fmt.Sprintf("%s-%d", strings.ToUpper(strings.ReplaceAll(p.Name, " ", "-")), vi+1)
			sku = strings.ReplaceAll(sku, "/", "-")
			sku = strings.ReplaceAll(sku, "(", "")
			sku = strings.ReplaceAll(sku, ")", "")

			_, err := database.Exec(`
				INSERT INTO product_variants (product_id, strength, pack_size, sku, price_cents, stock_qty)
				VALUES ($1, $2, $3, $4, $5, $6)
			`, productID, v.Strength, v.PackSize, sku, v.PriceCents, v.StockQty)
			if err != nil {
				log.Printf("insert variant for %s: %v", p.Name, err)
			}
		}
	}

	log.Printf("Seeded %d products", len(products))
}

func resetCatalog(database *sql.DB) {
	statements := []string{
		"DELETE FROM cart_items",
		"DELETE FROM carts",
		"DELETE FROM order_items",
		"DELETE FROM payments",
		"DELETE FROM prescriptions",
		"DELETE FROM orders",
		"DELETE FROM product_variants",
		"DELETE FROM products",
		"DELETE FROM categories",
	}
	for _, stmt := range statements {
		if _, err := database.Exec(stmt); err != nil {
			log.Printf("reset %s: %v", stmt, err)
		}
	}
}

func generateProducts() []productSeed {
	return []productSeed{
		// Pain Relief
		{"Paracetamol 500mg", "HealthPlus", "pain-relief", "Effective pain relief and fever reduction", "Paracetamol", false, "", []variantSeed{
			{"500mg", "100 tablets", 150000, 200},
			{"500mg", "200 tablets", 250000, 150},
		}},
		{"Ibuprofen 400mg", "MediCare", "pain-relief", "Anti-inflammatory pain relief", "Ibuprofen", false, "", []variantSeed{
			{"400mg", "50 tablets", 180000, 100},
			{"400mg", "100 tablets", 320000, 75},
		}},
		{"Aspirin 100mg", "HealthPlus", "pain-relief", "Blood thinning and pain relief medication", "Acetylsalicylic Acid", false, "", []variantSeed{
			{"100mg", "56 tablets", 120000, 0},
		}},
		{"Diclofenac Gel 1%", "PharmaCare", "pain-relief", "Topical gel for joint and muscle pain", "Diclofenac Diethylamine", false, "", []variantSeed{
			{"1%", "30g tube", 450000, 80},
			{"1%", "50g tube", 650000, 45},
		}},
		{"Tramadol 50mg", "PharmaStrong", "pain-relief", "Strong pain relief for moderate to severe pain", "Tramadol Hydrochloride", true, "", []variantSeed{
			{"50mg", "30 capsules", 850000, 50},
			{"50mg", "60 capsules", 1550000, 30},
		}},
		{"Codeine Phosphate 30mg", "PharmaStrong", "pain-relief", "Pain relief for moderate pain", "Codeine Phosphate", true, "", []variantSeed{
			{"30mg", "28 tablets", 950000, 40},
		}},
		{"Naproxen 250mg", "MediCare", "pain-relief", "Anti-inflammatory pain relief", "Naproxen Sodium", false, "", []variantSeed{
			{"250mg", "30 tablets", 450000, 80},
			{"250mg", "60 tablets", 780000, 50},
		}},
		{"Pregabalin 75mg", "PharmaStrong", "pain-relief", "Nerve pain relief", "Pregabalin", true, "", []variantSeed{
			{"75mg", "56 capsules", 1250000, 35},
		}},

		// Antibiotics
		{"Amoxicillin 500mg", "MediCare", "antibiotics", "Broad-spectrum antibiotic for bacterial infections", "Amoxicillin Trihydrate", true, "", []variantSeed{
			{"500mg", "21 capsules", 650000, 100},
			{"500mg", "42 capsules", 1200000, 60},
		}},
		{"Azithromycin 250mg", "MediCare", "antibiotics", "Macrolide antibiotic for respiratory infections", "Azithromycin Dihydrate", true, "", []variantSeed{
			{"250mg", "6 tablets", 150000, 80},
			{"250mg", "12 tablets", 280000, 40},
		}},
		{"Ciprofloxacin 500mg", "PharmaCare", "antibiotics", "Fluoroquinolone antibiotic", "Ciprofloxacin Hydrochloride", true, "", []variantSeed{
			{"500mg", "14 tablets", 850000, 90},
		}},
		{"Metronidazole 400mg", "HealthPlus", "antibiotics", "Antibiotic for anaerobic infections", "Metronidazole", true, "", []variantSeed{
			{"400mg", "21 tablets", 350000, 120},
			{"400mg", "42 tablets", 650000, 60},
		}},
		{"Doxycycline 100mg", "MediCare", "antibiotics", "Tetracycline antibiotic", "Doxycycline Hyclate", true, "", []variantSeed{
			{"100mg", "28 capsules", 750000, 70},
		}},
		{"Clarithromycin 500mg", "PharmaCare", "antibiotics", "Macrolide antibiotic", "Clarithromycin", true, "", []variantSeed{
			{"500mg", "14 tablets", 950000, 50},
		}},
		{"Levofloxacin 500mg", "MediCare", "antibiotics", "Fluoroquinolone antibiotic", "Levofloxacin", true, "", []variantSeed{
			{"500mg", "10 tablets", 1200000, 40},
		}},

		// Vitamins & Supplements
		{"Vitamin C 1000mg", "NutriVite", "vitamins", "Immune system support supplement", "Ascorbic Acid", false, "", []variantSeed{
			{"1000mg", "60 tablets", 350000, 300},
			{"1000mg", "120 tablets", 550000, 200},
		}},
		{"Vitamin D3 2000IU", "NaturePlus", "vitamins", "Bone health and immune support", "Cholecalciferol", false, "", []variantSeed{
			{"2000IU", "90 softgels", 500000, 150},
			{"2000IU", "180 softgels", 850000, 100},
		}},
		{"Vitamin B Complex", "NutriVite", "vitamins", "Energy metabolism support", "B Vitamins", false, "", []variantSeed{
			{"", "60 tablets", 400000, 200},
			{"", "120 tablets", 700000, 100},
		}},
		{"Zinc 50mg", "MineralPlus", "vitamins", "Immune support and wound healing", "Zinc Gluconate", false, "", []variantSeed{
			{"50mg", "100 tablets", 280000, 250},
		}},
		{"Omega-3 Fish Oil 1000mg", "NaturePlus", "vitamins", "Heart and brain health supplement", "Fish Oil", false, "", []variantSeed{
			{"1000mg", "60 softgels", 650000, 180},
			{"1000mg", "120 softgels", 1100000, 120},
		}},
		{"Iron 65mg", "MineralPlus", "vitamins", "Iron supplement for anemia prevention", "Ferrous Sulfate", false, "", []variantSeed{
			{"65mg", "100 tablets", 300000, 200},
		}},
		{"Multivitamin Daily", "NutriVite", "vitamins", "Complete daily multivitamin", "Multivitamin Blend", false, "", []variantSeed{
			{"", "30 tablets", 250000, 400},
			{"", "90 tablets", 550000, 300},
		}},
		{"Magnesium 400mg", "MineralPlus", "vitamins", "Muscle health and sleep support", "Magnesium Citrate", false, "", []variantSeed{
			{"400mg", "60 tablets", 500000, 180},
			{"400mg", "120 tablets", 850000, 120},
		}},
		{"Probiotic 50 Billion", "NutriVite", "vitamins", "Digestive health support", "Probiotic Blend", false, "", []variantSeed{
			{"50B CFU", "30 capsules", 950000, 100},
			{"50B CFU", "60 capsules", 1650000, 60},
		}},

		// First Aid
		{"Adhesive Bandages Mixed", "SafeCare", "first-aid", "Assorted sizes for minor cuts and scrapes", "", false, "", []variantSeed{
			{"Mixed", "50 pack", 200000, 500},
			{"Mixed", "100 pack", 350000, 300},
		}},
		{"Sterile Gauze Swabs", "SafeCare", "first-aid", "Sterile gauze for wound cleaning", "", false, "", []variantSeed{
			{"10x10cm", "100 pack", 300000, 400},
		}},
		{"Antiseptic Solution 200ml", "PharmaCare", "first-aid", "First aid antiseptic for wound cleaning", "Chlorhexidine Gluconate", false, "", []variantSeed{
			{"0.05%", "200ml", 350000, 200},
			{"0.05%", "500ml", 650000, 100},
		}},
		{"Elastic Bandage 4 inches", "SafeCare", "first-aid", "Elastic support bandage for sprains", "", false, "", []variantSeed{
			{"4in x 5yd", "1 bandage", 250000, 150},
		}},
		{"Instant Cold Pack", "SafeCare", "first-aid", "Instant cold therapy for injuries", "", false, "", []variantSeed{
			{"Standard", "1 pack", 180000, 200},
		}},
		{"First Aid Kit", "SafeCare", "first-aid", "Complete home first aid kit", "", false, "", []variantSeed{
			{"30 pieces", "1 kit", 950000, 80},
			{"50 pieces", "1 kit", 1450000, 50},
		}},
		{"Surgical Tape", "SafeCare", "first-aid", "Medical grade adhesive tape", "", false, "", []variantSeed{
			{"1 inch", "1 roll", 120000, 300},
			{"2 inch", "1 roll", 180000, 200},
		}},
		{"Scissors Medical", "SafeCare", "first-aid", "Stainless steel medical scissors", "", false, "", []variantSeed{
			{"14cm", "1 pair", 450000, 50},
		}},

		// Skincare
		{"Moisturizing Cream 50g", "DermaCare", "skincare", "Daily moisturizer for dry skin", "", false, "", []variantSeed{
			{"50g", "1 tube", 650000, 100},
			{"100g", "1 jar", 1050000, 60},
		}},
		{"Sunscreen SPF 50", "DermaCare", "skincare", "Broad spectrum sun protection", "", false, "", []variantSeed{
			{"SPF 50", "50ml", 850000, 120},
			{"SPF 50", "100ml", 1450000, 80},
		}},
		{"Hydrocortisone Cream 1%", "PharmaCare", "skincare", "Anti-itch and anti-inflammatory cream", "Hydrocortisone", false, "", []variantSeed{
			{"1%", "15g", 500000, 90},
			{"1%", "30g", 850000, 60},
		}},
		{"Acne Treatment Gel", "DermaCare", "skincare", "Gel for acne-prone skin", "Benzoyl Peroxide", false, "", []variantSeed{
			{"5%", "30g", 750000, 100},
			{"10%", "30g", 850000, 80},
		}},
		{"Vitamin E Oil", "NaturePlus", "skincare", "Moisturizing and skin repair oil", "Vitamin E", false, "", []variantSeed{
			{"30ml", "1 bottle", 450000, 150},
			{"60ml", "1 bottle", 750000, 100},
		}},
		{"Antifungal Cream 1%", "DermaCare", "skincare", "Treatment for fungal skin infections", "Clotrimazole", false, "", []variantSeed{
			{"1%", "20g", 500000, 80},
		}},
		{"Calamine Lotion 100ml", "DermaCare", "skincare", "Relief from itching and rashes", "Calamine", false, "", []variantSeed{
			{"100ml", "1 bottle", 350000, 100},
		}},

		// Baby Care
		{"Baby Diapers Size 4", "BabySoft", "baby-care", "Ultra-absorbent disposable diapers", "", false, "", []variantSeed{
			{"Size 4", "40 pack", 850000, 200},
			{"Size 4", "80 pack", 1500000, 150},
		}},
		{"Baby Wipes 80s", "BabySoft", "baby-care", "Gentle fragrance-free baby wipes", "", false, "", []variantSeed{
			{"80 sheets", "1 pack", 350000, 300},
			{"80 sheets", "3 pack", 850000, 200},
		}},
		{"Baby Oil 200ml", "BabySoft", "baby-care", "Gentle moisturizing baby oil", "Mineral Oil", false, "", []variantSeed{
			{"200ml", "1 bottle", 500000, 120},
		}},
		{"Baby Shampoo 250ml", "BabySoft", "baby-care", "Tear-free baby shampoo", "", false, "", []variantSeed{
			{"250ml", "1 bottle", 550000, 100},
		}},
		{"Baby Lotion 200ml", "BabySoft", "baby-care", "Daily gentle baby lotion", "", false, "", []variantSeed{
			{"200ml", "1 bottle", 650000, 90},
		}},
		{"Baby Powder 100g", "BabySoft", "baby-care", "Gentle talc-free baby powder", "", false, "", []variantSeed{
			{"100g", "1 bottle", 250000, 150},
		}},
		{"Baby Thermometer", "BabySoft", "baby-care", "Digital baby thermometer", "", false, "", []variantSeed{
			{"Digital", "1 unit", 750000, 60},
		}},
		{"Teething Gel", "BabySoft", "baby-care", "Soothing gel for teething babies", "Lidocaine", false, "", []variantSeed{
			{"10g", "1 tube", 450000, 80},
		}},
	}
}