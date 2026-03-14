package postgres

import (
	"context"
	"database/sql"
	"streamingbot/internal/domain/purchase"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PurchaseRepo struct {
	db *pgxpool.Pool
}

func NewPurchaseRepo(db *pgxpool.Pool) *PurchaseRepo {
	return &PurchaseRepo{db: db}
}

func (r *PurchaseRepo) GetByID(ctx context.Context, id string) (*purchase.Purchase, error) {
	return r.getOne(ctx, `SELECT id, user_id, content_id, status, telegram_payload, telegram_charge_id, stars_amount, created_at, paid_at, issue_link_attempts, last_issue_link_error, last_issue_link_at, review_requested_at FROM purchases WHERE id=$1`, id)
}

func (r *PurchaseRepo) GetByPayload(ctx context.Context, payload string) (*purchase.Purchase, error) {
	return r.getOne(ctx, `SELECT id, user_id, content_id, status, telegram_payload, telegram_charge_id, stars_amount, created_at, paid_at, issue_link_attempts, last_issue_link_error, last_issue_link_at, review_requested_at FROM purchases WHERE telegram_payload=$1`, payload)
}

func (r *PurchaseRepo) GetByChargeID(ctx context.Context, chargeID string) (*purchase.Purchase, error) {
	return r.getOne(ctx, `SELECT id, user_id, content_id, status, telegram_payload, telegram_charge_id, stars_amount, created_at, paid_at, issue_link_attempts, last_issue_link_error, last_issue_link_at, review_requested_at FROM purchases WHERE telegram_charge_id=$1`, chargeID)
}

func (r *PurchaseRepo) Create(ctx context.Context, p purchase.Purchase) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO purchases(
			id, user_id, content_id, status, telegram_payload, telegram_charge_id,
			stars_amount, created_at, paid_at, issue_link_attempts, last_issue_link_error,
			last_issue_link_at, review_requested_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
	`, p.ID, p.UserID, p.ContentID, string(p.Status), p.TelegramPayload, nullableString(p.TelegramChargeID), p.StarsAmount, p.CreatedAt, p.PaidAt, p.IssueLinkAttempts, nullableString(p.LastIssueLinkError), p.LastIssueLinkAt, p.ReviewRequestedAt)
	return err
}

func (r *PurchaseRepo) Update(ctx context.Context, p purchase.Purchase) error {
	_, err := r.db.Exec(ctx, `
		UPDATE purchases SET
			user_id=$2,
			content_id=$3,
			status=$4,
			telegram_payload=$5,
			telegram_charge_id=$6,
			stars_amount=$7,
			created_at=$8,
			paid_at=$9,
			issue_link_attempts=$10,
			last_issue_link_error=$11,
			last_issue_link_at=$12,
			review_requested_at=$13
		WHERE id=$1
	`, p.ID, p.UserID, p.ContentID, string(p.Status), p.TelegramPayload, nullableString(p.TelegramChargeID), p.StarsAmount, p.CreatedAt, p.PaidAt, p.IssueLinkAttempts, nullableString(p.LastIssueLinkError), p.LastIssueLinkAt, p.ReviewRequestedAt)
	return err
}

func (r *PurchaseRepo) getOne(ctx context.Context, q string, arg any) (*purchase.Purchase, error) {
	var (
		p           purchase.Purchase
		status      string
		chargeID    sql.NullString
		paidAt      sql.NullTime
		lastErr     sql.NullString
		lastIssueAt sql.NullTime
		reviewReqAt sql.NullTime
	)
	err := r.db.QueryRow(ctx, q, arg).Scan(
		&p.ID,
		&p.UserID,
		&p.ContentID,
		&status,
		&p.TelegramPayload,
		&chargeID,
		&p.StarsAmount,
		&p.CreatedAt,
		&paidAt,
		&p.IssueLinkAttempts,
		&lastErr,
		&lastIssueAt,
		&reviewReqAt,
	)
	if err != nil {
		if isNoRows(err) {
			return nil, nil
		}
		return nil, err
	}
	p.Status = purchase.Status(status)
	if chargeID.Valid {
		p.TelegramChargeID = chargeID.String
	}
	if paidAt.Valid {
		t := paidAt.Time
		p.PaidAt = &t
	}
	if lastErr.Valid {
		p.LastIssueLinkError = lastErr.String
	}
	if lastIssueAt.Valid {
		t := lastIssueAt.Time
		p.LastIssueLinkAt = &t
	}
	if reviewReqAt.Valid {
		t := reviewReqAt.Time
		p.ReviewRequestedAt = &t
	}
	return &p, nil
}

func nullableString(v string) any {
	if v == "" {
		return nil
	}
	return v
}
