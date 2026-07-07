package domain

import "context"

type StockRepository interface {
	GetBySKU(ctx context.Context, sku string) (*StockItem, error)
	UpdateReserved(ctx context.Context, sku string, expectedVersion int, newReserved int) (updated bool, err error)
}
