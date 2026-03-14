package jobs

import (
	"context"
	"time"
)

// Scheduler runs periodic background jobs (outbox publishing, expiry, review requests).
type Scheduler struct {
	Interval time.Duration
}

func NewScheduler(interval time.Duration) Scheduler {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return Scheduler{Interval: interval}
}

func (s Scheduler) Start(ctx context.Context, job func(context.Context) error) {
	ticker := time.NewTicker(s.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if job != nil {
				_ = job(ctx)
			}
		}
	}
}
