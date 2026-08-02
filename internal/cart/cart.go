package cart

import (
	"database/sql"
	"fmt"
)

type Cart struct {
	ID    int64       `json:"id"`
	Items []CartItem  `json:"items"`
}

func (c *Cart) TotalCents() int32 {
	var total int32
	for _, item := range c.Items {
		total += item.PriceCents * item.Quantity
	}
	return total
}

type CartItem struct {
	ID            int64  `json:"id"`
	CartID        int64  `json:"cart_id"`
	VariantID     int64  `json:"variant_id"`
	Quantity      int32  `json:"quantity"`
	ProductName   string `json:"product_name"`
	Strength      string `json:"strength"`
	PackSize      string `json:"pack_size"`
	PriceCents    int32  `json:"price_cents"`
	StockQty      int32  `json:"stock_qty"`
	RequiresPrescription bool `json:"requires_prescription"`
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) getOrCreateCartID(tx *sql.Tx, userID *int64, sessionToken string) (int64, error) {
	var cartID int64
	var err error
	q := func(query string, args ...interface{}) *sql.Row {
		if tx != nil {
			return tx.QueryRow(query, args...)
		}
		return s.db.QueryRow(query, args...)
	}
	if userID != nil {
		err = q(`SELECT id FROM carts WHERE user_id = $1`, *userID).Scan(&cartID)
	} else {
		err = q(`SELECT id FROM carts WHERE session_token = $1`, sessionToken).Scan(&cartID)
	}
	if err == sql.ErrNoRows {
		if userID != nil {
			err = q(`INSERT INTO carts (user_id) VALUES ($1) RETURNING id`, *userID).Scan(&cartID)
		} else {
			err = q(`INSERT INTO carts (session_token) VALUES ($1) RETURNING id`, sessionToken).Scan(&cartID)
		}
	}
	return cartID, err
}

func (s *Store) GetCart(userID *int64, sessionToken string) (*Cart, error) {
	cartID, err := s.getOrCreateCartID(nil, userID, sessionToken)
	if err != nil {
		return nil, fmt.Errorf("get cart id: %w", err)
	}

	cart := &Cart{ID: cartID}

	rows, err := s.db.Query(`
		SELECT ci.id, ci.cart_id, ci.variant_id, ci.quantity,
		       p.name, pv.strength, pv.pack_size, pv.price_cents, pv.stock_qty,
		       p.requires_prescription
		FROM cart_items ci
		JOIN product_variants pv ON ci.variant_id = pv.id
		JOIN products p ON pv.product_id = p.id
		WHERE ci.cart_id = $1
		ORDER BY ci.created_at
	`, cartID)
	if err != nil {
		return nil, fmt.Errorf("cart items: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var item CartItem
		if err := rows.Scan(&item.ID, &item.CartID, &item.VariantID, &item.Quantity,
			&item.ProductName, &item.Strength, &item.PackSize, &item.PriceCents, &item.StockQty,
			&item.RequiresPrescription); err != nil {
			return nil, fmt.Errorf("scan cart item: %w", err)
		}
		cart.Items = append(cart.Items, item)
	}
	return cart, nil
}

func (s *Store) AddItem(userID *int64, sessionToken string, variantID int64, quantity int32) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	cartID, err := s.getOrCreateCartID(tx, userID, sessionToken)
	if err != nil {
		return fmt.Errorf("get cart: %w", err)
	}

	_, err = tx.Exec(`
		INSERT INTO cart_items (cart_id, variant_id, quantity)
		VALUES ($1, $2, $3)
		ON CONFLICT (cart_id, variant_id) DO UPDATE SET quantity = cart_items.quantity + $3
	`, cartID, variantID, quantity)
	if err != nil {
		return fmt.Errorf("add item: %w", err)
	}

	return tx.Commit()
}

func (s *Store) UpdateQuantity(itemID int64, quantity int32) error {
	if quantity < 1 {
		return s.RemoveItem(itemID)
	}
	_, err := s.db.Exec(`UPDATE cart_items SET quantity = $1 WHERE id = $2`, quantity, itemID)
	return err
}

func (s *Store) RemoveItem(itemID int64) error {
	_, err := s.db.Exec(`DELETE FROM cart_items WHERE id = $1`, itemID)
	return err
}

func (s *Store) ClearCart(cartID int64, tx *sql.Tx) error {
	_, err := tx.Exec(`DELETE FROM cart_items WHERE cart_id = $1`, cartID)
	return err
}

func (s *Store) CartItemCount(userID *int64, sessionToken string) (int, error) {
	cartID, err := s.getOrCreateCartID(nil, userID, sessionToken)
	if err != nil {
		return 0, nil
	}
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM cart_items WHERE cart_id = $1`, cartID).Scan(&count)
	return count, nil
}
