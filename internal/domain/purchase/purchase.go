package purchase

import (
	"errors"
	"time"
)

var (
	ErrInvalidTransition = errors.New("invalid purchase status transition")
	ErrMissingChargeID   = errors.New("missing telegram charge id")
)

type Purchase struct {
	ID                 string
	UserID             int64
	ContentID          string
	Status             Status
	TelegramPayload    string
	TelegramChargeID   string
	StarsAmount        int
	CreatedAt          time.Time
	PaidAt             *time.Time
	IssueLinkAttempts  int
	LastIssueLinkError string
	LastIssueLinkAt    *time.Time
	ReviewRequestedAt  *time.Time
}

func (p *Purchase) MarkPaid(chargeID string, now time.Time) error {
	if p.Status != StatusPending {
		return ErrInvalidTransition
	}
	if chargeID == "" {
		return ErrMissingChargeID
	}
	p.Status = StatusPaid
	p.TelegramChargeID = chargeID
	p.PaidAt = &now
	return nil
}

func (p *Purchase) MarkAccessIssued() error {
	if p.Status != StatusPaid {
		return ErrInvalidTransition
	}
	p.Status = StatusAccessIssued
	return nil
}

func (p *Purchase) MarkExpired() error {
	if p.Status != StatusAccessIssued {
		return ErrInvalidTransition
	}
	p.Status = StatusExpired
	return nil
}

func (p *Purchase) RegisterIssueAccessFailure(errMsg string, now time.Time, maxAttempts int) {
	p.IssueLinkAttempts++
	p.LastIssueLinkError = errMsg
	p.LastIssueLinkAt = &now
	if p.IssueLinkAttempts >= maxAttempts {
		p.Status = StatusError
	}
}
