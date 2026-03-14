package user

import "context"

type Repository interface {
	GetByID(ctx context.Context, id int64) (*User, error)
	Upsert(ctx context.Context, u User) error
}
