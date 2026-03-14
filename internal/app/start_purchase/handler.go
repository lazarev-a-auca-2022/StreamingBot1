package start_purchase

import (
	"context"
	"encoding/json"
	"errors"
	"streamingbot/internal/domain/content"
	"streamingbot/internal/domain/purchase"
	"time"
)

var (
	ErrContentNotFound = errors.New("content not found")
	ErrContentInactive = errors.New("content inactive")
)

type IDGenerator interface {
	NewID() (string, error)
}

type Handler struct {
	Purchases purchase.Repository
	Contents  content.Repository
	IDs       IDGenerator
	Now       func() time.Time
}

func (h Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	c, err := h.Contents.GetByID(ctx, cmd.ContentID)
	if err != nil || c == nil {
		return Result{}, ErrContentNotFound
	}
	if !c.CanBePurchased() {
		return Result{}, ErrContentInactive
	}

	purchaseID, err := h.IDs.NewID()
	if err != nil {
		return Result{}, err
	}

	payloadObj := map[string]any{
		"purchase_id": purchaseID,
		"user_id":     cmd.UserID,
		"content_id":  cmd.ContentID,
	}
	payloadBytes, err := json.Marshal(payloadObj)
	if err != nil {
		return Result{}, err
	}
	payload := string(payloadBytes)

	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}

	p := purchase.Purchase{
		ID:              purchaseID,
		UserID:          cmd.UserID,
		ContentID:       cmd.ContentID,
		Status:          purchase.StatusPending,
		TelegramPayload: payload,
		StarsAmount:     c.PriceStars,
		CreatedAt:       now,
	}
	if err := h.Purchases.Create(ctx, p); err != nil {
		return Result{}, err
	}

	return Result{
		PurchaseID:     purchaseID,
		InvoicePayload: payload,
		AmountStars:    c.PriceStars,
	}, nil
}
