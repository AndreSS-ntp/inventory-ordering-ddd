package domain

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"
)

type fakeOrderRepo struct {
	createCalls int
	updateCalls int
	lastStatus  Status
}

func (f *fakeOrderRepo) Create(_ context.Context, order *Order) error {
	f.createCalls++
	order.ID = "order-1"
	order.CreatedAt = time.Now()
	return nil
}

func (f *fakeOrderRepo) UpdateStatus(_ context.Context, order *Order) error {
	f.updateCalls++
	f.lastStatus = order.Status
	return nil
}

type fakeInventory struct {
	success  bool
	reason   string
	attempts int
	err      error

	called bool
	sku    string
	qty    int
}

func (f *fakeInventory) Reserve(_ context.Context, sku string, quantity int) (bool, string, int, error) {
	f.called = true
	f.sku = sku
	f.qty = quantity
	return f.success, f.reason, f.attempts, f.err
}

func newTestOrderService(repo *fakeOrderRepo, inv *fakeInventory) *OrderService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return NewOrderService(repo, inv, logger)
}

func TestOrderService_PlaceOrder_Reserved(t *testing.T) {
	repo := &fakeOrderRepo{}
	inv := &fakeInventory{success: true}
	svc := newTestOrderService(repo, inv)

	order, _, err := svc.PlaceOrder(context.Background(), "SKU-1", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != StatusReserved {
		t.Errorf("expected status reserved, got %q", order.Status)
	}
	if !inv.called || inv.sku != "SKU-1" || inv.qty != 2 {
		t.Errorf("expected inventory.Reserve called with (SKU-1, 2), got called=%v sku=%q qty=%d", inv.called, inv.sku, inv.qty)
	}
	if repo.createCalls != 1 || repo.updateCalls != 1 {
		t.Errorf("expected 1 create + 1 update, got create=%d update=%d", repo.createCalls, repo.updateCalls)
	}
}

func TestOrderService_PlaceOrder_Failed(t *testing.T) {
	repo := &fakeOrderRepo{}
	inv := &fakeInventory{success: false, reason: "insufficient_stock"}
	svc := newTestOrderService(repo, inv)

	order, _, err := svc.PlaceOrder(context.Background(), "SKU-1", 999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != StatusFailed {
		t.Errorf("expected status failed, got %q", order.Status)
	}
	if order.FailureReason != "insufficient_stock" {
		t.Errorf("expected failure reason insufficient_stock, got %q", order.FailureReason)
	}
}

func TestOrderService_PlaceOrder_InventoryTransportError(t *testing.T) {
	repo := &fakeOrderRepo{}
	inv := &fakeInventory{err: errors.New("connection refused")}
	svc := newTestOrderService(repo, inv)

	order, _, err := svc.PlaceOrder(context.Background(), "SKU-1", 1)
	if err != nil {
		t.Fatalf("expected transport errors to be absorbed as a failed order, got error: %v", err)
	}
	if order.Status != StatusFailed || order.FailureReason != "inventory_unavailable" {
		t.Errorf("expected failed/inventory_unavailable, got status=%q reason=%q", order.Status, order.FailureReason)
	}
}

func TestOrderService_PlaceOrder_InvalidQuantity(t *testing.T) {
	repo := &fakeOrderRepo{}
	inv := &fakeInventory{success: true}
	svc := newTestOrderService(repo, inv)

	_, _, err := svc.PlaceOrder(context.Background(), "SKU-1", 0)
	if !errors.Is(err, ErrInvalidQuantity) {
		t.Fatalf("expected ErrInvalidQuantity, got %v", err)
	}
	if repo.createCalls != 0 {
		t.Errorf("expected no persistence attempt for an invalid order, got %d create calls", repo.createCalls)
	}
	if inv.called {
		t.Errorf("expected inventory not to be called for an invalid order")
	}
}
