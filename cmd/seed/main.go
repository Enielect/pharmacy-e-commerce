package main

import (
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

type productSeed struct {
	Name                string
	Brand               string
	Category            string
	Description         string
	ActiveIngredient    string
	RequiresPrescription bool
	Variants            []variantSeed
}

type variantSeed struct {
	Strength  string
	PackSize  string
	PriceCents int32
	StockQty  int32
}

func main() {
	cfg := config.Load()
	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer database.Close()

	// Check if products already exist
	var count int
	database.QueryRow("SELECT COUNT(*) FROM products").Scan(&count)
	if count > 0 {
		log.Println("Products already seeded, skipping")
		return
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

	for _, p := range products {
		slug := strings.ToLower(strings.ReplaceAll(p.Name, " ", "-"))
		slug = strings.ReplaceAll(slug, "/", "-")
		slug = strings.ReplaceAll(slug, "(", "")
		slug = strings.ReplaceAll(slug, ")", "")

		catID := catIDs[p.Category]

		var productID int64
		err := database.QueryRow(`
			INSERT INTO products (slug, name, brand, category_id, description, active_ingredient, requires_prescription)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id
		`, slug, p.Name, p.Brand, catID, p.Description, p.ActiveIngredient, p.RequiresPrescription).Scan(&productID)
		if err != nil {
			log.Printf("insert product %s: %v", p.Name, err)
			continue
		}

		for i, v := range p.Variants {
			sku := fmt.Sprintf("%s-%d", strings.ToUpper(strings.ReplaceAll(p.Name, " ", "-")), i+1)
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

func generateProducts() []productSeed {
	return []productSeed{
		// Pain Relief
		{"Paracetamol 500mg", "HealthPlus", "pain-relief", "Effective pain relief and fever reduction", "Paracetamol", false, []variantSeed{
			{"500mg", "100 tablets", 500, 200},
			{"500mg", "200 tablets", 900, 150},
		}},
		{"Ibuprofen 400mg", "MediCare", "pain-relief", "Anti-inflammatory pain relief", "Ibuprofen", false, []variantSeed{
			{"400mg", "50 tablets", 800, 100},
			{"400mg", "100 tablets", 1500, 75},
		}},
		{"Aspirin 100mg", "HealthPlus", "pain-relief", "Blood thinning and pain relief medication", "Acetylsalicylic Acid", false, []variantSeed{
			{"100mg", "56 tablets", 600, 0},
		}},
		{"Diclofenac Gel 1%", "PharmaCare", "pain-relief", "Topical gel for joint and muscle pain", "Diclofenac Diethylamine", false, []variantSeed{
			{"1%", "30g tube", 1800, 80},
			{"1%", "50g tube", 2500, 45},
		}},
		{"Tramadol 50mg", "PharmaStrong", "pain-relief", "Strong pain relief for moderate to severe pain", "Tramadol Hydrochloride", true, []variantSeed{
			{"50mg", "30 capsules", 3500, 50},
			{"50mg", "60 capsules", 6500, 30},
		}},
		{"Codeine Phosphate 30mg", "PharmaStrong", "pain-relief", "Pain relief for moderate pain", "Codeine Phosphate", true, []variantSeed{
			{"30mg", "28 tablets", 4200, 40},
		}},

		// Antibiotics
		{"Amoxicillin 500mg", "MediCare", "antibiotics", "Broad-spectrum antibiotic for bacterial infections", "Amoxicillin Trihydrate", true, []variantSeed{
			{"500mg", "21 capsules", 2500, 100},
			{"500mg", "42 capsules", 4500, 60},
		}},
		{"Azithromycin 250mg", "MediCare", "antibiotics", "Macrolide antibiotic for respiratory infections", "Azithromycin Dihydrate", true, []variantSeed{
			{"250mg", "6 tablets", 3000, 80},
			{"250mg", "12 tablets", 5500, 40},
		}},
		{"Ciprofloxacin 500mg", "PharmaCare", "antibiotics", "Fluoroquinolone antibiotic", "Ciprofloxacin Hydrochloride", true, []variantSeed{
			{"500mg", "14 tablets", 3500, 90},
		}},
		{"Metronidazole 400mg", "HealthPlus", "antibiotics", "Antibiotic for anaerobic infections", "Metronidazole", true, []variantSeed{
			{"400mg", "21 tablets", 1200, 120},
			{"400mg", "42 tablets", 2200, 60},
		}},
		{"Doxycycline 100mg", "MediCare", "antibiotics", "Tetracycline antibiotic", "Doxycycline Hyclate", true, []variantSeed{
			{"100mg", "28 capsules", 2800, 70},
		}},

		// Vitamins & Supplements
		{"Vitamin C 1000mg", "NutriVite", "vitamins", "Immune system support supplement", "Ascorbic Acid", false, []variantSeed{
			{"1000mg", "60 tablets", 1500, 300},
			{"1000mg", "120 tablets", 2500, 200},
		}},
		{"Vitamin D3 2000IU", "NaturePlus", "vitamins", "Bone health and immune support", "Cholecalciferol", false, []variantSeed{
			{"2000IU", "90 softgels", 2200, 150},
			{"2000IU", "180 softgels", 3800, 100},
		}},
		{"Vitamin B Complex", "NutriVite", "vitamins", "Energy metabolism support", "B Vitamins", false, []variantSeed{
			{"", "60 tablets", 1800, 200},
			{"", "120 tablets", 3200, 100},
		}},
		{"Zinc 50mg", "MineralPlus", "vitamins", "Immune support and wound healing", "Zinc Gluconate", false, []variantSeed{
			{"50mg", "100 tablets", 1200, 250},
		}},
		{"Omega-3 Fish Oil 1000mg", "NaturePlus", "vitamins", "Heart and brain health supplement", "Fish Oil", false, []variantSeed{
			{"1000mg", "60 softgels", 2800, 180},
			{"1000mg", "120 softgels", 4800, 120},
		}},
		{"Iron 65mg", "MineralPlus", "vitamins", "Iron supplement for anemia prevention", "Ferrous Sulfate", false, []variantSeed{
			{"65mg", "100 tablets", 1400, 200},
		}},
		{"Multivitamin Daily", "NutriVite", "vitamins", "Complete daily multivitamin", "Multivitamin Blend", false, []variantSeed{
			{"", "30 tablets", 1000, 400},
			{"", "90 tablets", 2500, 300},
		}},

		// First Aid
		{"Adhesive Bandages Mixed", "SafeCare", "first-aid", "Assorted sizes for minor cuts and scrapes", "", false, []variantSeed{
			{"Mixed", "50 pack", 800, 500},
			{"Mixed", "100 pack", 1400, 300},
		}},
		{"Sterile Gauze Swabs", "SafeCare", "first-aid", "Sterile gauze for wound cleaning", "", false, []variantSeed{
			{"10x10cm", "100 pack", 1200, 400},
		}},
		{"Antiseptic Solution 200ml", "PharmaCare", "first-aid", "First aid antiseptic for wound cleaning", "Chlorhexidine Gluconate", false, []variantSeed{
			{"0.05%", "200ml", 1500, 200},
			{"0.05%", "500ml", 2800, 100},
		}},
		{"Elastic Bandage 4 inches", "SafeCare", "first-aid", "Elastic support bandage for sprains", "", false, []variantSeed{
			{"4in x 5yd", "1 bandage", 1200, 150},
		}},
		{"Instant Cold Pack", "SafeCare", "first-aid", "Instant cold therapy for injuries", "", false, []variantSeed{
			{"Standard", "1 pack", 800, 200},
		}},
		{"First Aid Kit", "SafeCare", "first-aid", "Complete home first aid kit", "", false, []variantSeed{
			{"30 pieces", "1 kit", 4500, 80},
			{"50 pieces", "1 kit", 6500, 50},
		}},

		// Skincare
		{"Moisturizing Cream 50g", "DermaCare", "skincare", "Daily moisturizer for dry skin", "", false, []variantSeed{
			{"50g", "1 tube", 2500, 100},
			{"100g", "1 jar", 4200, 60},
		}},
		{"Sunscreen SPF 50", "DermaCare", "skincare", "Broad spectrum sun protection", "", false, []variantSeed{
			{"SPF 50", "50ml", 3500, 120},
			{"SPF 50", "100ml", 5500, 80},
		}},
		{"Hydrocortisone Cream 1%", "PharmaCare", "skincare", "Anti-itch and anti-inflammatory cream", "Hydrocortisone", false, []variantSeed{
			{"1%", "15g", 2000, 90},
			{"1%", "30g", 3500, 60},
		}},
		{"Acne Treatment Gel", "DermaCare", "skincare", "Gel for acne-prone skin", "Benzoyl Peroxide", false, []variantSeed{
			{"5%", "30g", 2800, 100},
			{"10%", "30g", 3200, 80},
		}},
		{"Vitamin E Oil", "NaturePlus", "skincare", "Moisturizing and skin repair oil", "Vitamin E", false, []variantSeed{
			{"30ml", "1 bottle", 1800, 150},
			{"60ml", "1 bottle", 3000, 100},
		}},

		// Baby Care
		{"Baby Diapers Size 4", "BabySoft", "baby-care", "Ultra-absorbent disposable diapers", "", false, []variantSeed{
			{"Size 4", "40 pack", 3500, 200},
			{"Size 4", "80 pack", 6000, 150},
		}},
		{"Baby Wipes 80s", "BabySoft", "baby-care", "Gentle fragrance-free baby wipes", "", false, []variantSeed{
			{"80 sheets", "1 pack", 1500, 300},
			{"80 sheets", "3 pack", 3800, 200},
		}},
		{"Baby Oil 200ml", "BabySoft", "baby-care", "Gentle moisturizing baby oil", "Mineral Oil", false, []variantSeed{
			{"200ml", "1 bottle", 2200, 120},
		}},
		{"Baby Shampoo 250ml", "BabySoft", "baby-care", "Tear-free baby shampoo", "", false, []variantSeed{
			{"250ml", "1 bottle", 2500, 100},
		}},
		{"Baby Lotion 200ml", "BabySoft", "baby-care", "Daily gentle baby lotion", "", false, []variantSeed{
			{"200ml", "1 bottle", 2800, 90},
		}},
		{"Baby Powder 100g", "BabySoft", "baby-care", "Gentle talc-free baby powder", "", false, []variantSeed{
			{"100g", "1 bottle", 1200, 150},
		}},

		// More Pain Relief
		{"Naproxen 250mg", "MediCare", "pain-relief", "Anti-inflammatory pain relief", "Naproxen Sodium", false, []variantSeed{
			{"250mg", "30 tablets", 1800, 80},
			{"250mg", "60 tablets", 3200, 50},
		}},
		{"Pregabalin 75mg", "PharmaStrong", "pain-relief", "Nerve pain relief", "Pregabalin", true, []variantSeed{
			{"75mg", "56 capsules", 5500, 35},
		}},

		// More Antibiotics
		{"Clarithromycin 500mg", "PharmaCare", "antibiotics", "Macrolide antibiotic", "Clarithromycin", true, []variantSeed{
			{"500mg", "14 tablets", 4000, 50},
		}},
		{"Levofloxacin 500mg", "MediCare", "antibiotics", "Fluoroquinolone antibiotic", "Levofloxacin", true, []variantSeed{
			{"500mg", "10 tablets", 5000, 40},
		}},

		// More supplements
		{"Magnesium 400mg", "MineralPlus", "vitamins", "Muscle health and sleep support", "Magnesium Citrate", false, []variantSeed{
			{"400mg", "60 tablets", 2000, 180},
			{"400mg", "120 tablets", 3500, 120},
		}},
		{"Probiotic 50 Billion", "NutriVite", "vitamins", "Digestive health support", "Probiotic Blend", false, []variantSeed{
			{"50B CFU", "30 capsules", 4500, 100},
			{"50B CFU", "60 capsules", 7500, 60},
		}},

		// More first aid
		{"Surgical Tape", "SafeCare", "first-aid", "Medical grade adhesive tape", "", false, []variantSeed{
			{"1 inch", "1 roll", 600, 300},
			{"2 inch", "1 roll", 1000, 200},
		}},
		{"Scissors Medical", "SafeCare", "first-aid", "Stainless steel medical scissors", "", false, []variantSeed{
			{"14cm", "1 pair", 1800, 50},
		}},

		// More skincare
		{"Antifungal Cream 1%", "DermaCare", "skincare", "Treatment for fungal skin infections", "Clotrimazole", false, []variantSeed{
			{"1%", "20g", 2000, 80},
		}},
		{"Calamine Lotion 100ml", "DermaCare", "skincare", "Relief from itching and rashes", "Calamine", false, []variantSeed{
			{"100ml", "1 bottle", 1500, 100},
		}},

		// More baby care
		{"Baby Thermometer", "BabySoft", "baby-care", "Digital baby thermometer", "", false, []variantSeed{
			{"Digital", "1 unit", 3000, 60},
		}},
		{"Teething Gel", "BabySoft", "baby-care", "Soothing gel for teething babies", "Lidocaine", false, []variantSeed{
			{"10g", "1 tube", 1800, 80},
		}},
	}
}
