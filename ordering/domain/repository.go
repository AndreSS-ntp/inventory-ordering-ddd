package domain

import "context"

type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	UpdateStatus(ctx context.Context, order *Order) error
}
