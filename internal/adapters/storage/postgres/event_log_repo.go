package postgres

import (
	"context"
	"streamingbot/internal/domain/payment"
)

type EventLogRepo struct{}

func NewEventLogRepo() EventLogRepo {
	return EventLogRepo{}
}

func (r EventLogRepo) SavePaymentEvent(ctx context.Context, event payment.Event) error {
	_ = ctx
	_ = event
	return nil
}
