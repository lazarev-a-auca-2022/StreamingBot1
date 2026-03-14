package telegram

import "context"

type Sender struct{}

func NewSender() Sender {
	return Sender{}
}

func (s Sender) SendAccessLink(ctx context.Context, userID int64, link string) error {
	_ = ctx
	_ = userID
	_ = link
	return nil
}

func (s Sender) SendReviewRequest(ctx context.Context, userID int64, purchaseID string) error {
	_ = ctx
	_ = userID
	_ = purchaseID
	return nil
}
