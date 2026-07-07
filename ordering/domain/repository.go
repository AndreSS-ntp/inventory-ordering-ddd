package domain

import "context"

// OrderRepository is implemented by the infrastructure layer (see
// repository/postgres.go).
type OrderRepository interface {
	// Create persists a new pending order and populates its generated ID
	// and CreatedAt.
	Create(ctx context.Context, order *Order) error

	// UpdateStatus persists the order's current Status and FailureReason.
	UpdateStatus(ctx context.Context, order *Order) error
}
