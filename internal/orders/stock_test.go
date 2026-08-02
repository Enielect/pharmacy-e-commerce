package orders

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestDecrementStockAtomic(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	// Create a test variant
	store := NewStore(db)

	t.Run("successful decrement", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		// Need a category and product first
		var catID int64
		err = tx.QueryRow("INSERT INTO categories (name, slug) VALUES ('test', 'test') ON CONFLICT (slug) DO UPDATE SET name = 'test' RETURNING id").Scan(&catID)
		if err != nil {
			t.Fatalf("insert category: %v", err)
		}

		var prodID int64
		err = tx.QueryRow("INSERT INTO products (slug, name, brand, category_id) VALUES ('test-product', 'Test Product', 'Test', $1) RETURNING id", catID).Scan(&prodID)
		if err != nil {
			t.Fatalf("insert product: %v", err)
		}

		var variantID int64
		err = tx.QueryRow("INSERT INTO product_variants (product_id, strength, pack_size, sku, price_cents, stock_qty) VALUES ($1, '10mg', '30 tabs', 'TEST-SKU', 1000, 10) RETURNING id",
			prodID).Scan(&variantID)
		if err != nil {
			t.Fatalf("insert variant: %v", err)
		}

		// Decrement by 3 (should succeed, stock was 10)
		err = store.DecrementStockAtomic(tx, variantID, 3)
		if err != nil {
			t.Fatalf("DecrementStockAtomic failed: %v", err)
		}

		// Verify stock is now 7
		var stock int32
		err = tx.QueryRow("SELECT stock_qty FROM product_variants WHERE id = $1", variantID).Scan(&stock)
		if err != nil {
			t.Fatalf("query stock: %v", err)
		}
		if stock != 7 {
			t.Errorf("expected stock 7, got %d", stock)
		}
	})

	t.Run("decrement below zero fails", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		var catID int64
		err = tx.QueryRow("INSERT INTO categories (name, slug) VALUES ('test2', 'test2') ON CONFLICT (slug) DO UPDATE SET name = 'test2' RETURNING id").Scan(&catID)
		if err != nil {
			t.Fatalf("insert category: %v", err)
		}

		var prodID int64
		err = tx.QueryRow("INSERT INTO products (slug, name, brand, category_id) VALUES ('test-product-2', 'Test Product 2', 'Test', $1) RETURNING id", catID).Scan(&prodID)
		if err != nil {
			t.Fatalf("insert product: %v", err)
		}

		var variantID int64
		err = tx.QueryRow("INSERT INTO product_variants (product_id, strength, pack_size, sku, price_cents, stock_qty) VALUES ($1, '10mg', '30 tabs', 'TEST-SKU-2', 1000, 5) RETURNING id",
			prodID).Scan(&variantID)
		if err != nil {
			t.Fatalf("insert variant: %v", err)
		}

		// Try to decrement by 10 (stock is only 5)
		err = store.DecrementStockAtomic(tx, variantID, 10)
		if err != ErrOutOfStock {
			t.Errorf("expected ErrOutOfStock, got %v", err)
		}

		// Verify stock unchanged (should still be 5)
		var stock int32
		err = tx.QueryRow("SELECT stock_qty FROM product_variants WHERE id = $1", variantID).Scan(&stock)
		if err != nil {
			t.Fatalf("query stock: %v", err)
		}
		if stock != 5 {
			t.Errorf("expected stock 5, got %d", stock)
		}
	})

	t.Run("exact stock decrement", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("begin tx: %v", err)
		}
		defer tx.Rollback()

		var catID int64
		err = tx.QueryRow("INSERT INTO categories (name, slug) VALUES ('test3', 'test3') ON CONFLICT (slug) DO UPDATE SET name = 'test3' RETURNING id").Scan(&catID)
		if err != nil {
			t.Fatalf("insert category: %v", err)
		}

		var prodID int64
		err = tx.QueryRow("INSERT INTO products (slug, name, brand, category_id) VALUES ('test-product-3', 'Test Product 3', 'Test', $1) RETURNING id", catID).Scan(&prodID)
		if err != nil {
			t.Fatalf("insert product: %v", err)
		}

		var variantID int64
		err = tx.QueryRow("INSERT INTO product_variants (product_id, strength, pack_size, sku, price_cents, stock_qty) VALUES ($1, '10mg', '30 tabs', 'TEST-SKU-3', 1000, 3) RETURNING id",
			prodID).Scan(&variantID)
		if err != nil {
			t.Fatalf("insert variant: %v", err)
		}

		// Decrement exactly to zero
		err = store.DecrementStockAtomic(tx, variantID, 3)
		if err != nil {
			t.Fatalf("DecrementStockAtomic failed: %v", err)
		}

		var stock int32
		err = tx.QueryRow("SELECT stock_qty FROM product_variants WHERE id = $1", variantID).Scan(&stock)
		if err != nil {
			t.Fatalf("query stock: %v", err)
		}
		if stock != 0 {
			t.Errorf("expected stock 0, got %d", stock)
		}
	})
}
