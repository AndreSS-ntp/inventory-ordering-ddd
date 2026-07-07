package domain

import (
	"errors"
	"time"
)

type Status string

const (
	StatusPending  Status = "pending"
	StatusReserved Status = "reserved"
	StatusFailed   Status = "failed"
)

var ErrInvalidQuantity = errors.New("invalid_quantity")

// Order is the Ordering aggregate. It starts pending, and moves to either
// reserved or failed once Inventory.Reserve has been called synchronously.
type Order struct {
	ID            string
	SKU           string
	Quantity      int
	Status        Status
	FailureReason string
	CreatedAt     time.Time
}

func NewOrder(sku string, quantity int) (*Order, error) {
	if quantity <= 0 {
		return nil, ErrInvalidQuantity
	}
	return &Order{
		SKU:      sku,
		Quantity: quantity,
		Status:   StatusPending,
	}, nil
}

func (o *Order) MarkReserved() {
	o.Status = StatusReserved
	o.FailureReason = ""
}

func (o *Order) MarkFailed(reason string) {
	o.Status = StatusFailed
	o.FailureReason = reason
}
