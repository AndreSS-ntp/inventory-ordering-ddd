package domain

import "context"

// StockRepository is implemented by the infrastructure layer (see
// repository/postgres.go). It's defined here, in the domain package, so the
// domain services depend only on an abstraction they own (dependency
// inversion), not on a concrete storage technology.
type StockRepository interface {
	GetBySKU(ctx context.Context, sku string) (*StockItem, error)

	// UpdateReserved performs a compare-and-swap equivalent to
	// UPDATE ... SET reserved_quantity = $1, version = version + 1
	// WHERE sku = $2 AND version = $3.
	// It returns updated=false (with a nil error) when no row matched,
	// meaning a concurrent writer changed the version first.
	UpdateReserved(ctx context.Context, sku string, expectedVersion int, newReserved int) (updated bool, err error)
}
