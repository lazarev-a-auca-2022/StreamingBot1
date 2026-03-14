package submit_review

import (
	"context"
	"errors"
	"streamingbot/internal/domain/purchase"
	"streamingbot/internal/domain/review"
	"time"
)

var (
	ErrInvalidRating    = errors.New("invalid rating")
	ErrPurchaseNotFound = errors.New("purchase not found")
)

type Handler struct {
	Purchases purchase.Repository
	Reviews   review.Repository
	Now       func() time.Time
}

func (h Handler) Handle(ctx context.Context, cmd Command) error {
	if cmd.Rating < 1 || cmd.Rating > 5 {
		return ErrInvalidRating
	}
	p, err := h.Purchases.GetByID(ctx, cmd.PurchaseID)
	if err != nil || p == nil {
		return ErrPurchaseNotFound
	}
	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	return h.Reviews.Create(ctx, review.Review{
		ID:         "review-" + cmd.PurchaseID,
		UserID:     cmd.UserID,
		PurchaseID: cmd.PurchaseID,
		Rating:     cmd.Rating,
		Text:       cmd.Text,
		Published:  false,
		CreatedAt:  now,
	})
}
