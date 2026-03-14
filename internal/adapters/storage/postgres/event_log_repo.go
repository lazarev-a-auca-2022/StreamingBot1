package postgres

import (
	"context"
	"streamingbot/internal/domain/payment"

	"github.com/jackc/pgx/v5/pgxpool"
)

type EventLogRepo struct {
	db *pgxpool.Pool
}

func NewEventLogRepo(db *pgxpool.Pool) *EventLogRepo {
	return &EventLogRepo{db: db}
}

func (r *EventLogRepo) SavePaymentEvent(ctx context.Context, event payment.Event) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO payment_events(charge_id, amount_stars, invoice_payload, raw_payload, occurred_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (charge_id) DO NOTHING
	`, event.ChargeID, event.AmountStars, event.InvoicePayload, event.RawPayload, event.OccurredAt)
	return err
}
