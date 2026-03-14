package streaming

import (
	"context"
	"time"
)

type Client struct{}

func NewClient() Client {
	return Client{}
}

func (c Client) IssueAccessLink(ctx context.Context, externalRef []byte, userID int64, ttl time.Duration, idempotencyKey string) (string, time.Time, error) {
	_ = ctx
	_ = externalRef
	_ = userID
	_ = idempotencyKey
	expiresAt := time.Now().Add(ttl)
	return "https://example.invalid/watch/token", expiresAt, nil
}
