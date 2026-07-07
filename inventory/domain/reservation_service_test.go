package domain

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
)

// fakeRepo is a hand-rolled mock of StockRepository. failsBeforeSucceed lets
// tests simulate a fixed number of optimistic-lock conflicts before the
// UPDATE finally succeeds.
type fakeRepo struct {
	item               StockItem
	failsBeforeSucceed int
	getErr             error
	updateErr          error

	getCalls    int
	updateCalls int
}

func (f *fakeRepo) GetBySKU(_ context.Context, _ string) (*StockItem, error) {
	f.getCalls++
	if f.getErr != nil {
		return nil, f.getErr
	}
	itemCopy := f.item
	return &itemCopy, nil
}

func (f *fakeRepo) UpdateReserved(_ context.Context, _ string, expectedVersion, newReserved int) (bool, error) {
	f.updateCalls++
	if f.updateErr != nil {
		return false, f.updateErr
	}
	if f.updateCalls <= f.failsBeforeSucceed {
		return false, nil
	}
	if expectedVersion != f.item.Version {
		return false, nil
	}
	f.item.ReservedQuantity = newReserved
	f.item.Version++
	return true, nil
}

func newTestService(repo StockRepository) *ReservationService {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewReservationService(repo, logger)
	svc.sleep = func(int) {} // no delay in tests
	return svc
}

func TestReservationService_Reserve_Success(t *testing.T) {
	repo := &fakeRepo{item: StockItem{SKU: "SKU-1", TotalQuantity: 10, ReservedQuantity: 0, Version: 0}}
	svc := newTestService(repo)

	result, err := svc.Reserve(context.Background(), "SKU-1", 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got reason %q", result.Reason)
	}
	if repo.item.ReservedQuantity != 4 {
		t.Errorf("expected reserved_quantity 4, got %d", repo.item.ReservedQuantity)
	}
	if repo.updateCalls != 1 {
		t.Errorf("expected exactly 1 UpdateReserved call, got %d", repo.updateCalls)
	}
}

func TestReservationService_Reserve_InsufficientStock(t *testing.T) {
	repo := &fakeRepo{item: StockItem{SKU: "SKU-1", TotalQuantity: 10, ReservedQuantity: 8, Version: 0}}
	svc := newTestService(repo)

	result, err := svc.Reserve(context.Background(), "SKU-1", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected failure")
	}
	if result.Reason != ErrInsufficientStock.Error() {
		t.Errorf("expected reason %q, got %q", ErrInsufficientStock.Error(), result.Reason)
	}
	if repo.updateCalls != 0 {
		t.Errorf("expected no UpdateReserved attempts when stock is insufficient, got %d", repo.updateCalls)
	}
}

func TestReservationService_Reserve_RetriesOnVersionConflictThenSucceeds(t *testing.T) {
	repo := &fakeRepo{
		item:               StockItem{SKU: "SKU-1", TotalQuantity: 10, ReservedQuantity: 0, Version: 0},
		failsBeforeSucceed: 2, // first two UpdateReserved calls simulate a concurrent writer winning
	}
	svc := newTestService(repo)

	result, err := svc.Reserve(context.Background(), "SKU-1", 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected eventual success, got reason %q", result.Reason)
	}
	if repo.updateCalls != 3 {
		t.Errorf("expected 3 UpdateReserved attempts (2 conflicts + 1 success), got %d", repo.updateCalls)
	}
	if repo.getCalls != 3 {
		t.Errorf("expected a fresh GetBySKU read per attempt, got %d calls", repo.getCalls)
	}
}

func TestReservationService_Reserve_VersionConflictExhausted(t *testing.T) {
	repo := &fakeRepo{
		item:               StockItem{SKU: "SKU-1", TotalQuantity: 10, ReservedQuantity: 0, Version: 0},
		failsBeforeSucceed: 999, // always conflicts
	}
	svc := newTestService(repo)

	result, err := svc.Reserve(context.Background(), "SKU-1", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success {
		t.Fatalf("expected failure after exhausting retries")
	}
	if result.Reason != ErrVersionConflictExhausted.Error() {
		t.Errorf("expected reason %q, got %q", ErrVersionConflictExhausted.Error(), result.Reason)
	}
	if repo.updateCalls != MaxReserveAttempts {
		t.Errorf("expected exactly %d attempts, got %d", MaxReserveAttempts, repo.updateCalls)
	}
}

func TestReservationService_Reserve_StockItemNotFound(t *testing.T) {
	repo := &fakeRepo{getErr: ErrStockItemNotFound}
	svc := newTestService(repo)

	result, err := svc.Reserve(context.Background(), "SKU-MISSING", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Success || result.Reason != ErrStockItemNotFound.Error() {
		t.Errorf("expected not-found reason, got success=%v reason=%q", result.Success, result.Reason)
	}
}

func TestReservationService_Reserve_RepositoryErrorPropagates(t *testing.T) {
	boom := errors.New("boom")
	repo := &fakeRepo{getErr: boom}
	svc := newTestService(repo)

	_, err := svc.Reserve(context.Background(), "SKU-1", 1)
	if !errors.Is(err, boom) {
		t.Fatalf("expected underlying repository error to propagate, got %v", err)
	}
}

func TestReservationService_Release_Success(t *testing.T) {
	repo := &fakeRepo{item: StockItem{SKU: "SKU-1", TotalQuantity: 10, ReservedQuantity: 5, Version: 0}}
	svc := newTestService(repo)

	result, err := svc.Release(context.Background(), "SKU-1", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Success {
		t.Fatalf("expected success, got reason %q", result.Reason)
	}
	if repo.item.ReservedQuantity != 0 {
		t.Errorf("expected reserved_quantity 0 after release, got %d", repo.item.ReservedQuantity)
	}
}
