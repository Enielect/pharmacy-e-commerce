package admin

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type ProductDraft struct {
	ID              int64           `json:"id"`
	ImageKey        string          `json:"image_key"`
	SuggestedFields json.RawMessage `json:"suggested_fields"`
	Status          string          `json:"status"`
	CreatedAt       time.Time       `json:"created_at"`
	ReviewedAt      *time.Time      `json:"reviewed_at"`
	ReviewedBy      *int64          `json:"reviewed_by"`
}

type SuggestedProduct struct {
	Name               string `json:"name"`
	Brand              string `json:"brand"`
	CategoryID         int64  `json:"category_id"`
	Description        string `json:"description"`
	ActiveIngredient   string `json:"active_ingredient"`
	Slug               string `json:"slug"`
	Strength           string `json:"strength"`
	PackSize           string `json:"pack_size"`
	SKU                string `json:"sku"`
	PriceCents         int32  `json:"price_cents"`
	StockQty           int32  `json:"stock_qty"`
	RequiresPrescription bool  `json:"requires_prescription"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) ListDrafts(status string) ([]ProductDraft, error) {
	var rows *sql.Rows
	var err error
	if status != "" {
		rows, err = s.db.Query(`
			SELECT id, image_key, suggested_fields, status, created_at, reviewed_at, reviewed_by
			FROM product_drafts WHERE status = $1 ORDER BY created_at DESC
		`, status)
	} else {
		rows, err = s.db.Query(`
			SELECT id, image_key, suggested_fields, status, created_at, reviewed_at, reviewed_by
			FROM product_drafts ORDER BY created_at DESC
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("list drafts: %w", err)
	}
	defer rows.Close()
	var drafts []ProductDraft
	for rows.Next() {
		var d ProductDraft
		if err := rows.Scan(&d.ID, &d.ImageKey, &d.SuggestedFields, &d.Status, &d.CreatedAt, &d.ReviewedAt, &d.ReviewedBy); err != nil {
			return nil, fmt.Errorf("scan draft: %w", err)
		}
		drafts = append(drafts, d)
	}
	return drafts, nil
}

func (s *Store) GetDraft(id int64) (*ProductDraft, error) {
	d := &ProductDraft{}
	err := s.db.QueryRow(`
		SELECT id, image_key, suggested_fields, status, created_at, reviewed_at, reviewed_by
		FROM product_drafts WHERE id = $1
	`, id).Scan(&d.ID, &d.ImageKey, &d.SuggestedFields, &d.Status, &d.CreatedAt, &d.ReviewedAt, &d.ReviewedBy)
	if err != nil {
		return nil, fmt.Errorf("get draft: %w", err)
	}
	return d, nil
}

func (s *Store) ApproveDraft(draftID int64, reviewedBy int64, fields SuggestedProduct) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback()

	// Create product
	var productID int64
	err = tx.QueryRow(`
		INSERT INTO products (slug, name, brand, category_id, description, active_ingredient, requires_prescription, primary_image_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id
	`, fields.Slug, fields.Name, fields.Brand, fields.CategoryID, fields.Description,
		fields.ActiveIngredient, fields.RequiresPrescription, "").Scan(&productID)
	if err != nil {
		return fmt.Errorf("create product: %w", err)
	}

	// Create variant
	_, err = tx.Exec(`
		INSERT INTO product_variants (product_id, strength, pack_size, sku, price_cents, stock_qty)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, productID, fields.Strength, fields.PackSize, fields.SKU, fields.PriceCents, fields.StockQty)
	if err != nil {
		return fmt.Errorf("create variant: %w", err)
	}

	// Update draft status
	_, err = tx.Exec(`
		UPDATE product_drafts SET status = 'approved', reviewed_at = now(), reviewed_by = $1 WHERE id = $2
	`, reviewedBy, draftID)
	if err != nil {
		return fmt.Errorf("update draft: %w", err)
	}

	return tx.Commit()
}

func (s *Store) RejectDraft(draftID int64, reviewedBy int64) error {
	_, err := s.db.Exec(`
		UPDATE product_drafts SET status = 'rejected', reviewed_at = now(), reviewed_by = $1 WHERE id = $2
	`, reviewedBy, draftID)
	return err
}
