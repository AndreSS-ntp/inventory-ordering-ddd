package domain

import "errors"

var (
	ErrInvalidQuantity   = errors.New("invalid_quantity")
	ErrInsufficientStock = errors.New("insufficient_stock")
	ErrStockItemNotFound = errors.New("stock_item_not_found")
)

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
