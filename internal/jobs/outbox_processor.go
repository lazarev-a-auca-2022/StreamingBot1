package jobs

import (
	"context"
	"streamingbot/internal/app/issue_access"
)

type OutboxEvent struct {
	ID         string
	Type       string
	PurchaseID string
}

type OutboxRepository interface {
	Unpublished(ctx context.Context, limit int) ([]OutboxEvent, error)
	MarkPublished(ctx context.Context, id string) error
}

type OutboxProcessor struct {
	Outbox      OutboxRepository
	IssueAccess issue_access.Handler
}

func (p OutboxProcessor) RunOnce(ctx context.Context) error {
	events, err := p.Outbox.Unpublished(ctx, 20)
	if err != nil {
		return err
	}

	for _, e := range events {
		switch e.Type {
		case "purchase_confirmed":
			if err := p.IssueAccess.Handle(ctx, issue_access.Command{PurchaseID: e.PurchaseID}); err != nil {
				continue
			}
			_ = p.Outbox.MarkPublished(ctx, e.ID)
		}
	}
	return nil
}
