package postgres

import (
	"context"
	"streamingbot/internal/domain/content"
	"sync"
)

type ContentRepo struct {
	mu   sync.RWMutex
	byID map[string]content.Content
}

func NewContentRepo() *ContentRepo {
	return &ContentRepo{byID: map[string]content.Content{}}
}

func (r *ContentRepo) GetByID(ctx context.Context, id string) (*content.Content, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	copy := c
	return &copy, nil
}

func (r *ContentRepo) ListActive(ctx context.Context) ([]content.Content, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]content.Content, 0, len(r.byID))
	for _, c := range r.byID {
		if c.Active {
			result = append(result, c)
		}
	}
	return result, nil
}

func (r *ContentRepo) Seed(c content.Content) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[c.ID] = c
}
