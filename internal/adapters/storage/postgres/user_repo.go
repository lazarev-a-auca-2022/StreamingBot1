package postgres

import (
	"context"
	"streamingbot/internal/domain/user"
	"sync"
)

type UserRepo struct {
	mu   sync.RWMutex
	byID map[int64]user.User
}

func NewUserRepo() *UserRepo {
	return &UserRepo{byID: map[int64]user.User{}}
}

func (r *UserRepo) GetByID(ctx context.Context, id int64) (*user.User, error) {
	_ = ctx
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.byID[id]
	if !ok {
		return nil, nil
	}
	copy := u
	return &copy, nil
}

func (r *UserRepo) Upsert(ctx context.Context, u user.User) error {
	_ = ctx
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byID[u.ID] = u
	return nil
}
