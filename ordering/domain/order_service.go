package domain

import (
	"context"
	"log/slog"
)

// InventoryReserver is the outbound port to the Inventory service. It's
// defined here (not in the grpc client package) so OrderService depends
// only on an abstraction it owns.
type InventoryReserver interface {
	// attempts is the number of read-check-CAS cycles Inventory performed
	// (1 = no optimistic-lock conflicts). It's surfaced for the load-test
	// report, not part of the persisted Order aggregate.
	Reserve(ctx context.Context, sku string, quantity int) (success bool, reason string, attempts int, err error)
}

// OrderService implements the "create order" use case: persist a pending
// order, synchronously call Inventory.Reserve, then persist the resulting
// status. It's shared by both the HTTP and gRPC adapters.
type OrderService struct {
	repo      OrderRepository
	inventory InventoryReserver
	logger    *slog.Logger
}

func NewOrderService(repo OrderRepository, inventory InventoryReserver, logger *slog.Logger) *OrderService {
	return &OrderService{repo: repo, inventory: inventory, logger: logger}
}

// PlaceOrder returns the persisted order plus the number of Inventory
// read-check-CAS attempts made (0 if Inventory was never called, e.g. on
// validation failure) - the latter purely for load-test/report purposes.
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
		// The Inventory call itself failed (network/timeout/etc, as opposed
		// to a business rejection). Treat it as a failed order rather than
		// bubbling a 500 up to the caller, so the client still gets a clean
		// JSON response describing what happened.
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
