package payments

import (
	"database/sql"
	"fmt"
	"math/rand"
)

type PaymentProvider interface {
	InitiateCheckout(amountCents int32, orderID int64) (string, error)
	VerifyWebhook(reference string) (bool, error)
}

type StubProvider struct {
	db *sql.DB
}

func NewStubProvider(db *sql.DB) *StubProvider {
	return &StubProvider{db: db}
}

func (p *StubProvider) InitiateCheckout(amountCents int32, orderID int64) (string, error) {
	ref := fmt.Sprintf("STUB-%d-%d", orderID, rand.Int63())
	_, err := p.db.Exec(`
		INSERT INTO payments (order_id, provider_reference, status, amount_cents)
		VALUES ($1, $2, 'pending', $3)
	`, orderID, ref, amountCents)
	if err != nil {
		return "", fmt.Errorf("create payment: %w", err)
	}
	return ref, nil
}

func (p *StubProvider) VerifyWebhook(reference string) (bool, error) {
	var status string
	err := p.db.QueryRow(`SELECT status FROM payments WHERE provider_reference = $1`, reference).Scan(&status)
	if err != nil {
		return false, fmt.Errorf("get payment: %w", err)
	}
	if status == "completed" {
		return true, nil
	}
	// Simulate successful payment
	_, err = p.db.Exec(`UPDATE payments SET status = 'completed' WHERE provider_reference = $1`, reference)
	if err != nil {
		return false, err
	}
	return true, nil
}
