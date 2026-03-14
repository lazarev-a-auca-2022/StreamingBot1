package postgres

import "context"

type IdempotencyRepo struct {
	processed map[string]struct{}
}

func NewIdempotencyRepo() *IdempotencyRepo {
	return &IdempotencyRepo{processed: map[string]struct{}{}}
}

func (r *IdempotencyRepo) IsProcessed(ctx context.Context, eventID string) (bool, error) {
	_ = ctx
	_, ok := r.processed[eventID]
	return ok, nil
}

func (r *IdempotencyRepo) MarkProcessed(ctx context.Context, eventID string) error {
	_ = ctx
	r.processed[eventID] = struct{}{}
	return nil
}
