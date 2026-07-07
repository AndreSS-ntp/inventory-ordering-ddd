package domain

import "testing"

func TestNewOrder_Valid(t *testing.T) {
	order, err := NewOrder("SKU-1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if order.Status != StatusPending {
		t.Errorf("expected new order to start pending, got %q", order.Status)
	}
	if order.SKU != "SKU-1" || order.Quantity != 3 {
		t.Errorf("unexpected order fields: %+v", order)
	}
}

func TestNewOrder_InvalidQuantity(t *testing.T) {
	for _, qty := range []int{0, -5} {
		if _, err := NewOrder("SKU-1", qty); err != ErrInvalidQuantity {
			t.Errorf("NewOrder(qty=%d): expected ErrInvalidQuantity, got %v", qty, err)
		}
	}
}

func TestOrder_MarkReserved(t *testing.T) {
	order := &Order{Status: StatusPending, FailureReason: "stale"}
	order.MarkReserved()
	if order.Status != StatusReserved {
		t.Errorf("expected status reserved, got %q", order.Status)
	}
	if order.FailureReason != "" {
		t.Errorf("expected failure reason cleared, got %q", order.FailureReason)
	}
}

func TestOrder_MarkFailed(t *testing.T) {
	order := &Order{Status: StatusPending}
	order.MarkFailed("insufficient_stock")
	if order.Status != StatusFailed {
		t.Errorf("expected status failed, got %q", order.Status)
	}
	if order.FailureReason != "insufficient_stock" {
		t.Errorf("expected failure reason set, got %q", order.FailureReason)
	}
}
