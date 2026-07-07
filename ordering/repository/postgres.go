package repository

import (
	"context"

	"coursework/ordering/domain"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresOrderRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOrderRepository(pool *pgxpool.Pool) *PostgresOrderRepository {
	return &PostgresOrderRepository{pool: pool}
}

func (r *PostgresOrderRepository) Create(ctx context.Context, order *domain.Order) error {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO orders (sku, quantity, status)
		VALUES ($1, $2, $3)
		RETURNING id, created_at
	`, order.SKU, order.Quantity, order.Status)
	return row.Scan(&order.ID, &order.CreatedAt)
}

func (r *PostgresOrderRepository) UpdateStatus(ctx context.Context, order *domain.Order) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE orders
		SET status = $1, failure_reason = $2
		WHERE id = $3
	`, string(order.Status), nullableString(order.FailureReason), order.ID)
	return err
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
