package domain

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"time"
)

// MaxReserveAttempts bounds the read-check-CAS retry loop on optimistic
// lock conflicts, per the spec: 5 attempts, small jittered delay between.
const MaxReserveAttempts = 5

var ErrVersionConflictExhausted = errors.New("version_conflict_exhausted")

// ReservationResult mirrors the gRPC response shape (success + reason)
// without coupling the domain layer to generated protobuf types.
type ReservationResult struct {
	Success bool
	Reason  string
	// Attempts is the number of read-check-CAS cycles performed (1 means
	// no optimistic-lock conflicts were hit).
	Attempts int
}

// ReservationService orchestrates reservation/release against a
// StockRepository, retrying on optimistic-lock conflicts.
type ReservationService struct {
	repo   StockRepository
	logger *slog.Logger
	sleep  func(attempt int)
}

func NewReservationService(repo StockRepository, logger *slog.Logger) *ReservationService {
	return &ReservationService{repo: repo, logger: logger, sleep: jitterSleep}
}

func jitterSleep(attempt int) {
	base := time.Duration(attempt) * 5 * time.Millisecond
	jitter := time.Duration(rand.Intn(10)) * time.Millisecond
	time.Sleep(base + jitter)
}

// Reserve implements: read StockItem by SKU, check invariant, attempt
// UPDATE ... WHERE sku = $1 AND version = $2. On a version conflict (no
// rows affected) it re-reads and retries, up to MaxReserveAttempts, with a
// jittered delay between attempts, then gives up with
// "version_conflict_exhausted".
func (s *ReservationService) Reserve(ctx context.Context, sku string, quantity int) (ReservationResult, error) {
	return s.retryMutate(ctx, sku, quantity, (*StockItem).Reserve, "reserve")
}

// Release is the compensating counterpart to Reserve, using the same
// optimistic-locking retry strategy since it's also a concurrent write to
// reserved_quantity.
func (s *ReservationService) Release(ctx context.Context, sku string, quantity int) (ReservationResult, error) {
	return s.retryMutate(ctx, sku, quantity, (*StockItem).Release, "release")
}

func (s *ReservationService) retryMutate(
	ctx context.Context,
	sku string,
	quantity int,
	mutate func(*StockItem, int) error,
	opName string,
) (ReservationResult, error) {
	if quantity <= 0 {
		return ReservationResult{Success: false, Reason: ErrInvalidQuantity.Error()}, nil
	}

	for attempt := 1; attempt <= MaxReserveAttempts; attempt++ {
		item, err := s.repo.GetBySKU(ctx, sku)
		if err != nil {
			if errors.Is(err, ErrStockItemNotFound) {
				return ReservationResult{Success: false, Reason: ErrStockItemNotFound.Error(), Attempts: attempt}, nil
			}
			return ReservationResult{}, err
		}

		if err := mutate(item, quantity); err != nil {
			if errors.Is(err, ErrInsufficientStock) {
				return ReservationResult{Success: false, Reason: ErrInsufficientStock.Error(), Attempts: attempt}, nil
			}
			return ReservationResult{}, err
		}

		ok, err := s.repo.UpdateReserved(ctx, sku, item.Version, item.ReservedQuantity)
		if err != nil {
			return ReservationResult{}, err
		}
		if ok {
			return ReservationResult{Success: true, Attempts: attempt}, nil
		}

		s.logger.WarnContext(ctx, "optimistic lock conflict, retrying",
			"op", opName, "sku", sku, "quantity", quantity,
			"attempt", attempt, "max_attempts", MaxReserveAttempts)

		if attempt < MaxReserveAttempts {
			s.sleep(attempt)
		}
	}

	s.logger.ErrorContext(ctx, "retries exhausted on version conflict",
		"op", opName, "sku", sku, "quantity", quantity, "max_attempts", MaxReserveAttempts)
	return ReservationResult{Success: false, Reason: ErrVersionConflictExhausted.Error(), Attempts: MaxReserveAttempts}, nil
}
