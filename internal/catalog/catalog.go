package catalog

import (
	"database/sql"
	"fmt"
	"time"
)

type Category struct {
	ID       int64  `json:"id"`
	ParentID *int64 `json:"parent_id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
}

type Product struct {
	ID                  int64     `json:"id"`
	Slug                string    `json:"slug"`
	Name                string    `json:"name"`
	Brand               string    `json:"brand"`
	CategoryID          int64     `json:"category_id"`
	Description         string    `json:"description"`
	ActiveIngredient    string    `json:"active_ingredient"`
	NafdacRegNumber     *string   `json:"nafdac_reg_number"`
	RequiresPrescription bool     `json:"requires_prescription"`
	PrimaryImageKey     string    `json:"primary_image_key"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type Variant struct {
	ID        int64     `json:"id"`
	ProductID int64     `json:"product_id"`
	Strength  string    `json:"strength"`
	PackSize  string    `json:"pack_size"`
	SKU       string    `json:"sku"`
	PriceCents int32    `json:"price_cents"`
	StockQty  int32     `json:"stock_qty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProductWithVariants struct {
	Product
	Variants []Variant `json:"variants"`
	Category Category  `json:"category"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Categories() ([]Category, error) {
	rows, err := s.db.Query(`SELECT id, parent_id, name, slug FROM categories ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("categories: %w", err)
	}
	defer rows.Close()
	var cats []Category
	for rows.Next() {
		var c Category
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.Slug); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		cats = append(cats, c)
	}
	return cats, nil
}

type ProductListParams struct {
	CategorySlug string
	Search       string
	Page         int
	PerPage      int
}

type ProductListResult struct {
	Products   []ProductWithVariants
	TotalCount int
	TotalPages int
	Page       int
	PerPage    int
}

func (s *Store) ListProducts(params ProductListParams) (*ProductListResult, error) {
	perPage := params.PerPage
	if perPage < 1 {
		perPage = 12
	}
	offset := (params.Page - 1) * perPage
	if offset < 0 {
		offset = 0
	}

	where := ""
	args := []interface{}{}
	argIdx := 1

	if params.CategorySlug != "" {
		where += fmt.Sprintf(" WHERE c.slug = $%d", argIdx)
		args = append(args, params.CategorySlug)
		argIdx++
	}
	if params.Search != "" {
		if where == "" {
			where = " WHERE"
		} else {
			where += " AND"
		}
		where += fmt.Sprintf(" (p.name ILIKE $%d OR p.brand ILIKE $%d OR p.active_ingredient ILIKE $%d)", argIdx, argIdx, argIdx)
		args = append(args, "%"+params.Search+"%")
		argIdx++
	}

	countQuery := `SELECT COUNT(*) FROM products p JOIN categories c ON p.category_id = c.id` + where
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count: %w", err)
	}

	query := fmt.Sprintf(`
		SELECT p.id, p.slug, p.name, p.brand, p.category_id, p.description, 
		       p.active_ingredient, p.nafdac_reg_number, p.requires_prescription,
		       p.primary_image_key, p.created_at, p.updated_at,
		       c.id, c.parent_id, c.name, c.slug
		FROM products p
		JOIN categories c ON p.category_id = c.id
		%s
		ORDER BY p.name
		LIMIT $%d OFFSET $%d
	`, where, argIdx, argIdx+1)
	args = append(args, perPage, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	var results []ProductWithVariants
	for rows.Next() {
		var pwv ProductWithVariants
		err := rows.Scan(
			&pwv.ID, &pwv.Slug, &pwv.Name, &pwv.Brand, &pwv.CategoryID, &pwv.Description,
			&pwv.ActiveIngredient, &pwv.NafdacRegNumber, &pwv.RequiresPrescription,
			&pwv.PrimaryImageKey, &pwv.CreatedAt, &pwv.UpdatedAt,
			&pwv.Category.ID, &pwv.Category.ParentID, &pwv.Category.Name, &pwv.Category.Slug,
		)
		if err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		pwv.Variants, err = s.variantsByProduct(pwv.ID)
		if err != nil {
			return nil, fmt.Errorf("variants for product %d: %w", pwv.ID, err)
		}
		results = append(results, pwv)
	}

	totalPages := total / perPage
	if total%perPage > 0 {
		totalPages++
	}
	return &ProductListResult{
		Products:   results,
		TotalCount: total,
		TotalPages: totalPages,
		Page:       params.Page,
		PerPage:    perPage,
	}, nil
}

func (s *Store) GetProductBySlug(slug string) (*ProductWithVariants, error) {
	pwv := &ProductWithVariants{}
	err := s.db.QueryRow(`
		SELECT p.id, p.slug, p.name, p.brand, p.category_id, p.description,
		       p.active_ingredient, p.nafdac_reg_number, p.requires_prescription,
		       p.primary_image_key, p.created_at, p.updated_at,
		       c.id, c.parent_id, c.name, c.slug
		FROM products p
		JOIN categories c ON p.category_id = c.id
		WHERE p.slug = $1
	`, slug).Scan(
		&pwv.ID, &pwv.Slug, &pwv.Name, &pwv.Brand, &pwv.CategoryID, &pwv.Description,
		&pwv.ActiveIngredient, &pwv.NafdacRegNumber, &pwv.RequiresPrescription,
		&pwv.PrimaryImageKey, &pwv.CreatedAt, &pwv.UpdatedAt,
		&pwv.Category.ID, &pwv.Category.ParentID, &pwv.Category.Name, &pwv.Category.Slug,
	)
	if err != nil {
		return nil, fmt.Errorf("get product %s: %w", slug, err)
	}
	pwv.Variants, err = s.variantsByProduct(pwv.ID)
	if err != nil {
		return nil, fmt.Errorf("variants: %w", err)
	}
	return pwv, nil
}

func (s *Store) GetVariant(id int64) (*Variant, error) {
	v := &Variant{}
	err := s.db.QueryRow(`
		SELECT id, product_id, strength, pack_size, sku, price_cents, stock_qty, created_at, updated_at
		FROM product_variants WHERE id = $1
	`, id).Scan(&v.ID, &v.ProductID, &v.Strength, &v.PackSize, &v.SKU, &v.PriceCents, &v.StockQty, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get variant %d: %w", id, err)
	}
	return v, nil
}

func (s *Store) variantsByProduct(productID int64) ([]Variant, error) {
	rows, err := s.db.Query(`
		SELECT id, product_id, strength, pack_size, sku, price_cents, stock_qty, created_at, updated_at
		FROM product_variants WHERE product_id = $1 ORDER BY price_cents
	`, productID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var variants []Variant
	for rows.Next() {
		var v Variant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.Strength, &v.PackSize, &v.SKU, &v.PriceCents, &v.StockQty, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	return variants, nil
}

func (s *Store) UpdateVariantStock(variantID int64, stockQty int32) error {
	_, err := s.db.Exec(`UPDATE product_variants SET stock_qty = $1, updated_at = now() WHERE id = $2`, stockQty, variantID)
	return err
}

func (s *Store) UpdateVariantPrice(variantID int64, priceCents int32) error {
	_, err := s.db.Exec(`UPDATE product_variants SET price_cents = $1, updated_at = now() WHERE id = $2`, priceCents, variantID)
	return err
}
