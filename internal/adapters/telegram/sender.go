package telegram

import (
	"context"
	"log"
)

type Sender struct{}

func NewSender() Sender {
	return Sender{}
}

func (s Sender) SendAccessLink(ctx context.Context, userID int64, link string) error {
	_ = ctx
	log.Printf("telegram_send_access user_id=%d link=%s", userID, link)
	return nil
}

func (s Sender) SendReviewRequest(ctx context.Context, userID int64, purchaseID string) error {
	_ = ctx
	log.Printf("telegram_send_review_request user_id=%d purchase_id=%s", userID, purchaseID)
	return nil
}
