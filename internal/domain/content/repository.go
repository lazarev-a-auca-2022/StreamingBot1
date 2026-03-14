package content

import "context"

type Repository interface {
	GetByID(ctx context.Context, id string) (*Content, error)
	ListActive(ctx context.Context) ([]Content, error)
}
