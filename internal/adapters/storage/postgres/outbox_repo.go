package postgres

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type OutboxEvent struct {
	ID         string
	Type       string
	PurchaseID string
	CreatedAt  time.Time
	Published  bool
}

type OutboxRepo struct {
	mu     sync.Mutex
	events []OutboxEvent
	nextID int
}

func NewOutboxRepo() *OutboxRepo {
	return &OutboxRepo{events: []OutboxEvent{}, nextID: 1}
}

func (r *OutboxRepo) PublishPurchaseConfirmed(ctx context.Context, purchaseID string) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	e := OutboxEvent{
		ID:         r.newID(),
		Type:       "purchase_confirmed",
		PurchaseID: purchaseID,
		CreatedAt:  time.Now(),
	}
	r.events = append(r.events, e)
	return nil
}

func (r *OutboxRepo) Unpublished(ctx context.Context, limit int) ([]OutboxEvent, error) {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 {
		limit = 10
	}
	res := make([]OutboxEvent, 0, limit)
	for _, e := range r.events {
		if !e.Published {
			res = append(res, e)
			if len(res) >= limit {
				break
			}
		}
	}
	return res, nil
}

func (r *OutboxRepo) MarkPublished(ctx context.Context, id string) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := range r.events {
		if r.events[i].ID == id {
			r.events[i].Published = true
			break
		}
	}
	return nil
}

func (r *OutboxRepo) newID() string {
	id := r.nextID
	r.nextID++
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), id)
}
