package issue_access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"streamingbot/internal/domain/access"
	"streamingbot/internal/domain/content"
	"streamingbot/internal/domain/purchase"
	"time"
)

var (
	ErrPurchaseNotFound = errors.New("purchase not found")
	ErrContentNotFound  = errors.New("content not found")
	ErrContentInactive  = errors.New("content inactive")
)

type StreamingProvider interface {
	IssueAccessLink(ctx context.Context, externalRef []byte, userID int64, ttl time.Duration, idempotencyKey string) (string, time.Time, error)
}

type TokenService interface {
	Generate() (raw string, hash string, err error)
}

type TelegramSender interface {
	SendAccessLink(ctx context.Context, userID int64, link string) error
}

type TokenCache interface {
	Put(ctx context.Context, tokenHash string, purchaseID string, ttl time.Duration) error
}

type Handler struct {
	Purchases  purchase.Repository
	Contents   content.Repository
	Grants     access.Repository
	Provider   StreamingProvider
	Tokens     TokenService
	Sender     TelegramSender
	Cache      TokenCache
	Now        func() time.Time
	TTL        time.Duration
	MaxRetries int
}

func (h Handler) Handle(ctx context.Context, cmd Command) error {
	p, err := h.Purchases.GetByID(ctx, cmd.PurchaseID)
	if err != nil || p == nil {
		return ErrPurchaseNotFound
	}

	c, err := h.Contents.GetByID(ctx, p.ContentID)
	if err != nil || c == nil {
		return ErrContentNotFound
	}
	if !c.CanBePurchased() {
		return ErrContentInactive
	}

	link, expiresAt, err := h.Provider.IssueAccessLink(ctx, c.ExternalRef, p.UserID, h.TTL, p.ID)
	if err != nil {
		now := time.Now()
		if h.Now != nil {
			now = h.Now()
		}
		p.RegisterIssueAccessFailure(err.Error(), now, h.MaxRetries)
		_ = h.Purchases.Update(ctx, *p)
		return err
	}

	tokenRaw, tokenHash, err := h.Tokens.Generate()
	if err != nil {
		return err
	}
	if tokenHash == "" {
		digest := sha256.Sum256([]byte(tokenRaw))
		tokenHash = hex.EncodeToString(digest[:])
	}

	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	grant := access.Grant{
		ID:         fmt.Sprintf("grant-%s", p.ID),
		PurchaseID: p.ID,
		UserID:     p.UserID,
		TokenHash:  tokenHash,
		IssuedAt:   now,
		ExpiresAt:  expiresAt,
	}
	if err := h.Grants.Create(ctx, grant); err != nil {
		return err
	}
	if h.Cache != nil {
		_ = h.Cache.Put(ctx, tokenHash, p.ID, time.Until(expiresAt))
	}

	if err := p.MarkAccessIssued(); err != nil {
		return err
	}
	if err := h.Purchases.Update(ctx, *p); err != nil {
		return err
	}

	finalLink := fmt.Sprintf("%s?token=%s", link, tokenRaw)
	return h.Sender.SendAccessLink(ctx, p.UserID, finalLink)
}
