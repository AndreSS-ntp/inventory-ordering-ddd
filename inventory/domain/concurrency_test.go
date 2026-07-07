package domain

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
)

// concurrentInMemoryRepo is a real compare-and-swap implementation (mutex
// guarded) rather than a scripted mock, so this test exercises genuine races
// between goroutines the same way concurrent DB transactions would.
type concurrentInMemoryRepo struct {
	mu   sync.Mutex
	item StockItem
}

func (r *concurrentInMemoryRepo) GetBySKU(_ context.Context, _ string) (*StockItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	itemCopy := r.item
	return &itemCopy, nil
}

func (r *concurrentInMemoryRepo) UpdateReserved(_ context.Context, _ string, expectedVersion, newReserved int) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.item.Version != expectedVersion {
		return false, nil
	}
	r.item.ReservedQuantity = newReserved
	r.item.Version++
	return true, nil
}

// TestReservationService_Reserve_ConcurrencyRespectsInvariant fires 50
// goroutines at a SKU with 10 units of stock, each requesting 1 unit, and
// asserts reserved_quantity never exceeds total_quantity and that exactly as
// many requests succeed as there was stock available.
func TestReservationService_Reserve_ConcurrencyRespectsInvariant(t *testing.T) {
	const totalStock = 10
	const workers = 50

	repo := &concurrentInMemoryRepo{item: StockItem{SKU: "SKU-CONCURRENT", TotalQuantity: totalStock}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	svc := NewReservationService(repo, logger)

	var wg sync.WaitGroup
	results := make([]ReservationResult, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := svc.Reserve(context.Background(), "SKU-CONCURRENT", 1)
			if err != nil {
				t.Errorf("worker %d: unexpected error: %v", i, err)
				return
			}
			results[i] = result
		}(i)
	}
	wg.Wait()

	successCount := 0
	for _, r := range results {
		if r.Success {
			successCount++
		}
	}

	if successCount != totalStock {
		t.Errorf("expected exactly %d successful reservations (bounded by stock), got %d", totalStock, successCount)
	}

	repo.mu.Lock()
	finalReserved := repo.item.ReservedQuantity
	repo.mu.Unlock()

	if finalReserved != successCount {
		t.Errorf("reserved_quantity (%d) does not match count of successful reservations (%d)", finalReserved, successCount)
	}
	if finalReserved > totalStock {
		t.Errorf("invariant violated: reserved_quantity %d > total_quantity %d", finalReserved, totalStock)
	}
}
