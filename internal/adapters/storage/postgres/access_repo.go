package postgres

import (
	"context"
	"streamingbot/internal/domain/access"
	"sync"
)

type AccessRepo struct {
	mu          sync.RWMutex
	byPurchase  map[string]access.Grant
	byTokenHash map[string]string
}

func NewAccessRepo() *AccessRepo {
	return &AccessRepo{
		byPurchase:  map[string]access.Grant{},
		byTokenHash: map[string]string{},
	}
}

func (r *AccessRepo) GetByPurchaseID(ctx context.Context, purchaseID string) (*access.Grant, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	g, ok := r.byPurchase[purchaseID]
	if !ok {
		return nil, nil
	}
	copy := g
	return &copy, nil
}

func (r *AccessRepo) GetByTokenHash(ctx context.Context, tokenHash string) (*access.Grant, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	purchaseID, ok := r.byTokenHash[tokenHash]
	if !ok {
		return nil, nil
	}
	g := r.byPurchase[purchaseID]
	copy := g
	return &copy, nil
}

func (r *AccessRepo) Create(ctx context.Context, g access.Grant) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byPurchase[g.PurchaseID] = g
	r.byTokenHash[g.TokenHash] = g.PurchaseID
	return nil
}

func (r *AccessRepo) MarkUsed(ctx context.Context, grantID string) error {
	_ = ctx
	_ = grantID
	return nil
}
