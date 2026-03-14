package access

import "context"

type Repository interface {
	GetByPurchaseID(ctx context.Context, purchaseID string) (*Grant, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*Grant, error)
	Create(ctx context.Context, g Grant) error
	MarkUsed(ctx context.Context, grantID string) error
}
