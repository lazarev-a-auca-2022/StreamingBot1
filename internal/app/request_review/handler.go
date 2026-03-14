package request_review

import (
	"context"
	"errors"
	"streamingbot/internal/domain/purchase"
	"streamingbot/internal/domain/review"
	"time"
)

var (
	ErrPurchaseNotFound = errors.New("purchase not found")
)

type TelegramSender interface {
	SendReviewRequest(ctx context.Context, userID int64, purchaseID string) error
}

type Handler struct {
	Purchases purchase.Repository
	Reviews   review.Repository
	Sender    TelegramSender
	Now       func() time.Time
}

func (h Handler) Handle(ctx context.Context, cmd Command) error {
	p, err := h.Purchases.GetByID(ctx, cmd.PurchaseID)
	if err != nil || p == nil {
		return ErrPurchaseNotFound
	}

	existing, err := h.Reviews.GetByPurchaseID(ctx, p.ID)
	if err == nil && existing != nil {
		return nil
	}

	if err := h.Sender.SendReviewRequest(ctx, p.UserID, p.ID); err != nil {
		return err
	}

	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	p.ReviewRequestedAt = &now
	return h.Purchases.Update(ctx, *p)
}
