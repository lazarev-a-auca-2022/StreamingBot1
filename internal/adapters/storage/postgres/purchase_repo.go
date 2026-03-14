package postgres

import (
	"context"
	"streamingbot/internal/domain/purchase"
	"sync"
)

type PurchaseRepo struct {
	mu        sync.RWMutex
	byID      map[string]purchase.Purchase
	byPayload map[string]string
	byCharge  map[string]string
}

func NewPurchaseRepo() *PurchaseRepo {
	return &PurchaseRepo{
		byID:      map[string]purchase.Purchase{},
		byPayload: map[string]string{},
		byCharge:  map[string]string{},
	}
}

func (r *PurchaseRepo) GetByID(ctx context.Context, id string) (*purchase.Purchase, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	copy := p
	return &copy, nil
}

func (r *PurchaseRepo) GetByPayload(ctx context.Context, payload string) (*purchase.Purchase, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byPayload[payload]
	if !ok {
		return nil, nil
	}
	p := r.byID[id]
	copy := p
	return &copy, nil
}

func (r *PurchaseRepo) GetByChargeID(ctx context.Context, chargeID string) (*purchase.Purchase, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.byCharge[chargeID]
	if !ok {
		return nil, nil
	}
	p := r.byID[id]
	copy := p
	return &copy, nil
}

func (r *PurchaseRepo) Create(ctx context.Context, p purchase.Purchase) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[p.ID] = p
	r.byPayload[p.TelegramPayload] = p.ID
	if p.TelegramChargeID != "" {
		r.byCharge[p.TelegramChargeID] = p.ID
	}
	return nil
}

func (r *PurchaseRepo) Update(ctx context.Context, p purchase.Purchase) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[p.ID] = p
	r.byPayload[p.TelegramPayload] = p.ID
	if p.TelegramChargeID != "" {
		r.byCharge[p.TelegramChargeID] = p.ID
	}
	return nil
}
