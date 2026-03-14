package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type IdempotencyRepo struct {
	db *pgxpool.Pool
}

func NewIdempotencyRepo(db *pgxpool.Pool) *IdempotencyRepo {
	return &IdempotencyRepo{db: db}
}

func (r *IdempotencyRepo) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	var found int
	err := r.db.QueryRow(ctx, `SELECT 1 FROM idempotency_keys WHERE event_id=$1`, eventID).Scan(&found)
	if err != nil {
		if isNoRows(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *IdempotencyRepo) MarkProcessed(ctx context.Context, eventID string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO idempotency_keys(event_id) VALUES ($1) ON CONFLICT (event_id) DO NOTHING`, eventID)
	return err
}
