package purchase

import "context"

type Repository interface {
	GetByID(ctx context.Context, id string) (*Purchase, error)
	GetByPayload(ctx context.Context, payload string) (*Purchase, error)
	GetByChargeID(ctx context.Context, chargeID string) (*Purchase, error)
	Create(ctx context.Context, p Purchase) error
	Update(ctx context.Context, p Purchase) error
}
