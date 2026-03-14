package confirm_payment

import (
	"context"
	"errors"
	"streamingbot/internal/domain/payment"
	"streamingbot/internal/domain/purchase"
	"time"
)

var (
	ErrEmptyChargeID    = errors.New("empty charge id")
	ErrPurchaseNotFound = errors.New("purchase not found")
)

type EventLogRepository interface {
	SavePaymentEvent(ctx context.Context, event payment.Event) error
}

type OutboxPublisher interface {
	PublishPurchaseConfirmed(ctx context.Context, purchaseID string) error
}

type Handler struct {
	Purchases   purchase.Repository
	Idempotency payment.IdempotencyRepository
	EventLog    EventLogRepository
	Outbox      OutboxPublisher
	Now         func() time.Time
}

func (h Handler) Handle(ctx context.Context, cmd Command) error {
	if cmd.Event.ChargeID == "" {
		return ErrEmptyChargeID
	}

	alreadyProcessed, err := h.Idempotency.IsProcessed(ctx, cmd.Event.ChargeID)
	if err != nil {
		return err
	}
	if alreadyProcessed {
		return nil
	}

	p, err := h.Purchases.GetByPayload(ctx, cmd.Event.InvoicePayload)
	if err != nil || p == nil {
		return ErrPurchaseNotFound
	}

	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	if err := p.MarkPaid(cmd.Event.ChargeID, now); err != nil {
		return err
	}

	if err := h.Purchases.Update(ctx, *p); err != nil {
		return err
	}
	if err := h.EventLog.SavePaymentEvent(ctx, cmd.Event); err != nil {
		return err
	}
	if err := h.Outbox.PublishPurchaseConfirmed(ctx, p.ID); err != nil {
		return err
	}

	return h.Idempotency.MarkProcessed(ctx, cmd.Event.ChargeID)
}
