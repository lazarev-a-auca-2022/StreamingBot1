package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OutboxEvent struct {
	ID         string
	Type       string
	PurchaseID string
	CreatedAt  time.Time
	Published  bool
}

type OutboxRepo struct {
	db *pgxpool.Pool
}

func NewOutboxRepo(db *pgxpool.Pool) *OutboxRepo {
	return &OutboxRepo{db: db}
}

func (r *OutboxRepo) PublishPurchaseConfirmed(ctx context.Context, purchaseID string) error {
	id := fmt.Sprintf("%d-%s", time.Now().UnixNano(), purchaseID)
	_, err := r.db.Exec(ctx, `
		INSERT INTO outbox_events(id, event_type, purchase_id, published)
		VALUES ($1, 'purchase_confirmed', $2, FALSE)
	`, id, purchaseID)
	return err
}

func (r *OutboxRepo) Unpublished(ctx context.Context, limit int) ([]OutboxEvent, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.db.Query(ctx, `
		SELECT id, event_type, purchase_id, created_at, published
		FROM outbox_events
		WHERE published=FALSE
		ORDER BY created_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []OutboxEvent
	for rows.Next() {
		var e OutboxEvent
		if err := rows.Scan(&e.ID, &e.Type, &e.PurchaseID, &e.CreatedAt, &e.Published); err != nil {
			return nil, err
		}
		result = append(result, e)
	}
	return result, rows.Err()
}

func (r *OutboxRepo) MarkPublished(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE outbox_events SET published=TRUE WHERE id=$1`, id)
	return err
}
