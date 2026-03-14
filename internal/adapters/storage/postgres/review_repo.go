package postgres

import (
	"context"
	"streamingbot/internal/domain/review"
	"sync"
)

type ReviewRepo struct {
	mu         sync.RWMutex
	byPurchase map[string]review.Review
}

func NewReviewRepo() *ReviewRepo {
	return &ReviewRepo{byPurchase: map[string]review.Review{}}
}

func (r *ReviewRepo) GetByPurchaseID(ctx context.Context, purchaseID string) (*review.Review, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	rev, ok := r.byPurchase[purchaseID]
	if !ok {
		return nil, nil
	}
	copy := rev
	return &copy, nil
}

func (r *ReviewRepo) Create(ctx context.Context, rev review.Review) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byPurchase[rev.PurchaseID] = rev
	return nil
}
