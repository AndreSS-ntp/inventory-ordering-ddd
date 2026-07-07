package domain

import "testing"

func TestStockItem_Reserve_Success(t *testing.T) {
	item := &StockItem{SKU: "SKU-1", TotalQuantity: 10, ReservedQuantity: 3, Version: 0}

	if err := item.Reserve(5); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if item.ReservedQuantity != 8 {
		t.Errorf("expected reserved_quantity 8, got %d", item.ReservedQuantity)
	}
}

func TestStockItem_Reserve_InsufficientStock(t *testing.T) {
	item := &StockItem{SKU: "SKU-1", TotalQuantity: 10, ReservedQuantity: 8, Version: 0}

	err := item.Reserve(5)
	if err != ErrInsufficientStock {
		t.Fatalf("expected ErrInsufficientStock, got %v", err)
	}
	if item.ReservedQuantity != 8 {
		t.Errorf("state must not change on failed reservation, got reserved_quantity %d", item.ReservedQuantity)
	}
}

func TestStockItem_Reserve_ExactAvailableSucceeds(t *testing.T) {
	item := &StockItem{SKU: "SKU-1", TotalQuantity: 10, ReservedQuantity: 0, Version: 0}

	if err := item.Reserve(10); err != nil {
		t.Fatalf("expected no error reserving exactly the available amount, got %v", err)
	}
	if item.Available() != 0 {
		t.Errorf("expected 0 available after reserving all stock, got %d", item.Available())
	}
}

func TestStockItem_Reserve_InvalidQuantity(t *testing.T) {
	item := &StockItem{SKU: "SKU-1", TotalQuantity: 10, ReservedQuantity: 0, Version: 0}

	for _, qty := range []int{0, -1} {
		if err := item.Reserve(qty); err != ErrInvalidQuantity {
			t.Errorf("Reserve(%d): expected ErrInvalidQuantity, got %v", qty, err)
		}
	}
}

func TestStockItem_Release_ClampsToZero(t *testing.T) {
	item := &StockItem{SKU: "SKU-1", TotalQuantity: 10, ReservedQuantity: 3, Version: 0}

	if err := item.Release(100); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if item.ReservedQuantity != 0 {
		t.Errorf("expected reserved_quantity clamped to 0, got %d", item.ReservedQuantity)
	}
}
