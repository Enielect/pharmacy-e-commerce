package orders

import (
	"database/sql"
	"fmt"
	"time"
)

type Order struct {
	ID                   int64     `json:"id"`
	UserID               int64     `json:"user_id"`
	Status               string    `json:"status"`
	TotalCents           int32     `json:"total_cents"`
	ShippingLine1        string    `json:"shipping_line1"`
	ShippingCity         string    `json:"shipping_city"`
	ShippingState        string    `json:"shipping_state"`
	ShippingPhone        string    `json:"shipping_phone"`
	CreatedAt            time.Time `json:"created_at"`
}

type OrderItem struct {
	ID                 int64  `json:"id"`
	OrderID            int64  `json:"order_id"`
	VariantID          int64  `json:"variant_id"`
	ProductNameSnapshot string `json:"product_name_snapshot"`
	PriceCentsSnapshot int32  `json:"price_cents_snapshot"`
	Quantity           int32  `json:"quantity"`
}

type LineItem struct {
	VariantID int64
	ProductName string
	PriceCents int32
	Quantity   int32
}

type CheckoutInput struct {
	UserID        int64
	CartID        int64
	LineItems     []LineItem
	AddressLine1  string
	AddressCity   string
	AddressState  string
	AddressPhone  string
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *sql.DB { return s.db }

var ErrOutOfStock = fmt.Errorf("item out of stock")

func (s *Store) DecrementStockAtomic(tx *sql.Tx, variantID int64, quantity int32) error {
	result, err := tx.Exec(`
		UPDATE product_variants SET stock_qty = stock_qty - $1, updated_at = now()
		WHERE id = $2 AND stock_qty >= $1
	`, quantity, variantID)
	if err != nil {
		return fmt.Errorf("decrement stock: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return ErrOutOfStock
	}
	return nil
}

func (s *Store) Checkout(input CheckoutInput) (*Order, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Calculate total and decrement stock atomically
	var totalCents int32
	for _, item := range input.LineItems {
		if err := s.DecrementStockAtomic(tx, item.VariantID, item.Quantity); err != nil {
			return nil, fmt.Errorf("stock decrement for variant %d: %w", item.VariantID, err)
		}
		totalCents += item.PriceCents * item.Quantity
	}

	// Determine initial status
	initialStatus := "pending_payment"

	// Create order
	var orderID int64
	err = tx.QueryRow(`
		INSERT INTO orders (user_id, status, total_cents, shipping_address_line1, shipping_address_city, shipping_address_state, shipping_address_phone)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id
	`, input.UserID, initialStatus, totalCents, input.AddressLine1, input.AddressCity, input.AddressState, input.AddressPhone).Scan(&orderID)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	// Create order items with snapshots
	for _, item := range input.LineItems {
		_, err = tx.Exec(`
			INSERT INTO order_items (order_id, variant_id, product_name_snapshot, price_cents_snapshot, quantity)
			VALUES ($1, $2, $3, $4, $5)
		`, orderID, item.VariantID, item.ProductName, item.PriceCents, item.Quantity)
		if err != nil {
			return nil, fmt.Errorf("create order item: %w", err)
		}
	}

	// Clear cart
	_, err = tx.Exec(`DELETE FROM cart_items WHERE cart_id = $1`, input.CartID)
	if err != nil {
		return nil, fmt.Errorf("clear cart: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return s.GetOrder(orderID)
}

func (s *Store) GetOrder(orderID int64) (*Order, error) {
	o := &Order{}
	err := s.db.QueryRow(`
		SELECT id, user_id, status, total_cents,
		       shipping_address_line1, shipping_address_city, shipping_address_state, shipping_address_phone,
		       created_at
		FROM orders WHERE id = $1
	`, orderID).Scan(&o.ID, &o.UserID, &o.Status, &o.TotalCents,
		&o.ShippingLine1, &o.ShippingCity, &o.ShippingState, &o.ShippingPhone, &o.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("get order: %w", err)
	}
	return o, nil
}

func (s *Store) GetOrderItems(orderID int64) ([]OrderItem, error) {
	rows, err := s.db.Query(`
		SELECT id, order_id, variant_id, product_name_snapshot, price_cents_snapshot, quantity
		FROM order_items WHERE order_id = $1
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []OrderItem
	for rows.Next() {
		var i OrderItem
		if err := rows.Scan(&i.ID, &i.OrderID, &i.VariantID, &i.ProductNameSnapshot, &i.PriceCentsSnapshot, &i.Quantity); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	return items, nil
}

func (s *Store) ListOrdersByUser(userID int64) ([]Order, error) {
	rows, err := s.db.Query(`
		SELECT id, user_id, status, total_cents,
		       shipping_address_line1, shipping_address_city, shipping_address_state, shipping_address_phone,
		       created_at
		FROM orders WHERE user_id = $1 ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalCents,
			&o.ShippingLine1, &o.ShippingCity, &o.ShippingState, &o.ShippingPhone, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (s *Store) AllOrders(status string) ([]Order, error) {
	var rows *sql.Rows
	var err error
	if status != "" {
		rows, err = s.db.Query(`
			SELECT id, user_id, status, total_cents,
			       shipping_address_line1, shipping_address_city, shipping_address_state, shipping_address_phone,
			       created_at
			FROM orders WHERE status = $1 ORDER BY created_at DESC
		`, status)
	} else {
		rows, err = s.db.Query(`
			SELECT id, user_id, status, total_cents,
			       shipping_address_line1, shipping_address_city, shipping_address_state, shipping_address_phone,
			       created_at
			FROM orders ORDER BY created_at DESC
		`)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var orders []Order
	for rows.Next() {
		var o Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalCents,
			&o.ShippingLine1, &o.ShippingCity, &o.ShippingState, &o.ShippingPhone, &o.CreatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (s *Store) UpdateOrderStatus(orderID int64, status string) error {
	_, err := s.db.Exec(`UPDATE orders SET status = $1 WHERE id = $2`, status, orderID)
	return err
}

// Prescription management

type Prescription struct {
	ID              int64      `json:"id"`
	OrderID         int64      `json:"order_id"`
	FileKey         string     `json:"file_key"`
	VerifiedByUserID *int64    `json:"verified_by_user_id"`
	VerifiedAt      *time.Time `json:"verified_at"`
	Status          string     `json:"status"`
}

func (s *Store) CreatePrescription(orderID int64, fileKey string) (int64, error) {
	var id int64
	err := s.db.QueryRow(`
		INSERT INTO prescriptions (order_id, file_key, status) VALUES ($1, $2, 'pending') RETURNING id
	`, orderID, fileKey).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create prescription: %w", err)
	}
	// Set order status to pending pharmacist review
	s.db.Exec(`UPDATE orders SET status = 'pending_pharmacist_review' WHERE id = $1`, orderID)
	return id, nil
}

func (s *Store) GetPrescriptionsByOrder(orderID int64) ([]Prescription, error) {
	rows, err := s.db.Query(`
		SELECT id, order_id, file_key, verified_by_user_id, verified_at, status
		FROM prescriptions WHERE order_id = $1
	`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ps []Prescription
	for rows.Next() {
		var p Prescription
		if err := rows.Scan(&p.ID, &p.OrderID, &p.FileKey, &p.VerifiedByUserID, &p.VerifiedAt, &p.Status); err != nil {
			return nil, err
		}
		ps = append(ps, p)
	}
	return ps, nil
}

func (s *Store) VerifyPrescription(prescriptionID int64, verifiedByUserID int64, status string) error {
	_, err := s.db.Exec(`
		UPDATE prescriptions SET status = $1, verified_by_user_id = $2, verified_at = now()
		WHERE id = $3
	`, status, verifiedByUserID, prescriptionID)
	if err != nil {
		return fmt.Errorf("verify prescription: %w", err)
	}
	// Update order status based on prescription verification
	var orderID int64
	s.db.QueryRow(`SELECT order_id FROM prescriptions WHERE id = $1`, prescriptionID).Scan(&orderID)
	if status == "approved" {
		s.db.Exec(`UPDATE orders SET status = 'processing' WHERE id = $1`, orderID)
	} else {
		s.db.Exec(`UPDATE orders SET status = 'cancelled' WHERE id = $1`, orderID)
	}
	return nil
}

// Order total calculation (for testing)
func CalculateTotal(items []LineItem) int32 {
	var total int32
	for _, item := range items {
		total += item.PriceCents * item.Quantity
	}
	return total
}
