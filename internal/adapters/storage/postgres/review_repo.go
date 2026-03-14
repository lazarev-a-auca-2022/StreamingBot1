package postgres

import (
	"context"
	"streamingbot/internal/domain/review"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ReviewRepo struct {
	db *pgxpool.Pool
}

func NewReviewRepo(db *pgxpool.Pool) *ReviewRepo {
	return &ReviewRepo{db: db}
}

func (r *ReviewRepo) GetByPurchaseID(ctx context.Context, purchaseID string) (*review.Review, error) {
	var rv review.Review
	err := r.db.QueryRow(ctx, `SELECT id, user_id, purchase_id, rating, text, published, created_at FROM reviews WHERE purchase_id=$1`, purchaseID).
		Scan(&rv.ID, &rv.UserID, &rv.PurchaseID, &rv.Rating, &rv.Text, &rv.Published, &rv.CreatedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	return &rv, nil
}

func (r *ReviewRepo) Create(ctx context.Context, rv review.Review) error {
	if rv.CreatedAt.IsZero() {
		rv.CreatedAt = time.Now()
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO reviews(id, user_id, purchase_id, rating, text, published, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (purchase_id) DO NOTHING
	`, rv.ID, rv.UserID, rv.PurchaseID, rv.Rating, rv.Text, rv.Published, rv.CreatedAt)
	return err
}
