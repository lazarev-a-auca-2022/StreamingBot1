package postgres

import (
	"context"
	"streamingbot/internal/domain/payment"
	"sync"
)

type EventLogRepo struct {
	mu     sync.Mutex
	events []payment.Event
}

func NewEventLogRepo() *EventLogRepo {
	return &EventLogRepo{events: []payment.Event{}}
}

func (r *EventLogRepo) SavePaymentEvent(ctx context.Context, event payment.Event) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event)
	return nil
}
