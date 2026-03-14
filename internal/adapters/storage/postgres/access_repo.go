package postgres

import (
	"context"
	"database/sql"
	"streamingbot/internal/domain/access"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AccessRepo struct {
	db *pgxpool.Pool
}

func NewAccessRepo(db *pgxpool.Pool) *AccessRepo {
	return &AccessRepo{db: db}
}

func (r *AccessRepo) GetByPurchaseID(ctx context.Context, purchaseID string) (*access.Grant, error) {
	return r.getOne(ctx, `SELECT id, purchase_id, user_id, token_hash, issued_at, expires_at, used_at FROM access_grants WHERE purchase_id=$1`, purchaseID)
}

func (r *AccessRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*access.Grant, error) {
	return r.getOne(ctx, `SELECT id, purchase_id, user_id, token_hash, issued_at, expires_at, used_at FROM access_grants WHERE token_hash=$1`, tokenHash)
}

func (r *AccessRepo) Create(ctx context.Context, g access.Grant) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO access_grants(id, purchase_id, user_id, token_hash, issued_at, expires_at, used_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
	`, g.ID, g.PurchaseID, g.UserID, g.TokenHash, g.IssuedAt, g.ExpiresAt, g.UsedAt)
	return err
}

func (r *AccessRepo) MarkUsed(ctx context.Context, grantID string) error {
	_, err := r.db.Exec(ctx, `UPDATE access_grants SET used_at=NOW() WHERE id=$1 AND used_at IS NULL`, grantID)
	return err
}

func (r *AccessRepo) getOne(ctx context.Context, q string, arg any) (*access.Grant, error) {
	var g access.Grant
	var usedAt sql.NullTime
	err := r.db.QueryRow(ctx, q, arg).Scan(&g.ID, &g.PurchaseID, &g.UserID, &g.TokenHash, &g.IssuedAt, &g.ExpiresAt, &usedAt)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	if usedAt.Valid {
		t := usedAt.Time
		g.UsedAt = &t
	}
	return &g, nil
}
