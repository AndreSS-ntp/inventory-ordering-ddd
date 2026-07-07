package repository

import (
	"context"
	"errors"

	"coursework/inventory/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresStockRepository implements domain.StockRepository against a
// stock_items table (see migrations/0001_init.up.sql).
type PostgresStockRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresStockRepository(pool *pgxpool.Pool) *PostgresStockRepository {
	return &PostgresStockRepository{pool: pool}
}

func (r *PostgresStockRepository) GetBySKU(ctx context.Context, sku string) (*domain.StockItem, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, sku, total_quantity, reserved_quantity, version
		FROM stock_items
		WHERE sku = $1
	`, sku)

	var item domain.StockItem
	err := row.Scan(&item.ID, &item.SKU, &item.TotalQuantity, &item.ReservedQuantity, &item.Version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrStockItemNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *PostgresStockRepository) UpdateReserved(ctx context.Context, sku string, expectedVersion int, newReserved int) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE stock_items
		SET reserved_quantity = $1, version = version + 1
		WHERE sku = $2 AND version = $3
	`, newReserved, sku, expectedVersion)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}
