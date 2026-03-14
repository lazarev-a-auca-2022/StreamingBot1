package use_access

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"streamingbot/internal/domain/access"
	"time"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrTokenExpired = errors.New("token expired")
	ErrTokenUsed    = errors.New("token already used")
)

type Handler struct {
	Grants access.Repository
	Cache  TokenCache
	Now    func() time.Time
}

type TokenCache interface {
	Get(ctx context.Context, tokenHash string) (string, error)
	Delete(ctx context.Context, tokenHash string) error
}

func (h Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	digest := sha256.Sum256([]byte(cmd.Token))
	tokenHash := hex.EncodeToString(digest[:])

	if h.Cache != nil {
		_, _ = h.Cache.Get(ctx, tokenHash)
	}

	grant, err := h.Grants.GetByTokenHash(ctx, tokenHash)
	if err != nil || grant == nil {
		return Result{}, ErrInvalidToken
	}

	now := time.Now()
	if h.Now != nil {
		now = h.Now()
	}
	if grant.UsedAt != nil {
		return Result{}, ErrTokenUsed
	}
	if now.After(grant.ExpiresAt) {
		return Result{}, ErrTokenExpired
	}

	if err := h.Grants.MarkUsed(ctx, grant.ID); err != nil {
		return Result{}, err
	}
	if h.Cache != nil {
		_ = h.Cache.Delete(ctx, tokenHash)
	}

	return Result{PurchaseID: grant.PurchaseID, UserID: grant.UserID, Valid: true}, nil
}
