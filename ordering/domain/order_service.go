package domain

import (
	"context"
	"log/slog"
)

type InventoryReserver interface {
	Reserve(ctx context.Context, sku string, quantity int) (success bool, reason string, attempts int, err error)
}

type OrderService struct {
	repo      OrderRepository
	inventory InventoryReserver
	logger    *slog.Logger
}

func NewOrderService(repo OrderRepository, inventory InventoryReserver, logger *slog.Logger) *OrderService {
	return &OrderService{repo: repo, inventory: inventory, logger: logger}
}

func (s *OrderService) PlaceOrder(ctx context.Context, sku string, quantity int) (*Order, int, error) {
	order, err := NewOrder(sku, quantity)
	if err != nil {
		return nil, 0, err
	}

	if err := s.repo.Create(ctx, order); err != nil {
		return nil, 0, err
	}

	success, reason, attempts, err := s.inventory.Reserve(ctx, sku, quantity)
	switch {
	case err != nil:
		s.logger.ErrorContext(ctx, "inventory reserve call failed", "order_id", order.ID, "sku", sku, "error", err)
		order.MarkFailed("inventory_unavailable")
	case success:
		order.MarkReserved()
	default:
		order.MarkFailed(reason)
	}

	if err := s.repo.UpdateStatus(ctx, order); err != nil {
		return nil, 0, err
	}

	return order, attempts, nil
}
