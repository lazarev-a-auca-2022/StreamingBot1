package review

import "context"

type Repository interface {
	GetByPurchaseID(ctx context.Context, purchaseID string) (*Review, error)
	Create(ctx context.Context, r Review) error
}
