package domain

import "errors"

var (
	ErrInvalidQuantity   = errors.New("invalid_quantity")
	ErrInsufficientStock = errors.New("insufficient_stock")
	ErrStockItemNotFound = errors.New("stock_item_not_found")
)

// StockItem is the Inventory aggregate. The invariant reserved_quantity <=
// total_quantity must hold at all times, including under concurrent access
// to the same SKU (enforced by ReservationService via optimistic locking on
// Version, not by this struct alone).
type StockItem struct {
	ID               string
	SKU              string
	TotalQuantity    int
	ReservedQuantity int
	Version          int
}

func (s *StockItem) Available() int {
	return s.TotalQuantity - s.ReservedQuantity
}

// Reserve mutates the in-memory aggregate state, enforcing the invariant.
// It does not persist anything - see ReservationService for the
// read-check-compare-and-swap orchestration against storage.
func (s *StockItem) Reserve(quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}
	if s.Available() < quantity {
		return ErrInsufficientStock
	}
	s.ReservedQuantity += quantity
	return nil
}

// Release mutates the in-memory aggregate state, decreasing reserved
// quantity. It clamps to zero rather than going negative, since it's a
// compensating action and should be safe to call defensively.
func (s *StockItem) Release(quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}
	if quantity > s.ReservedQuantity {
		quantity = s.ReservedQuantity
	}
	s.ReservedQuantity -= quantity
	return nil
}
