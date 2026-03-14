package access

import "time"

type Grant struct {
	ID         string
	PurchaseID string
	UserID     int64
	TokenHash  string
	IssuedAt   time.Time
	ExpiresAt  time.Time
	UsedAt     *time.Time
}

func (g Grant) IsUsable(now time.Time) bool {
	return g.UsedAt == nil && now.Before(g.ExpiresAt)
}
