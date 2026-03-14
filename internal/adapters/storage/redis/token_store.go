package redis

import "context"

type TokenStore struct{}

func NewTokenStore() TokenStore {
	return TokenStore{}
}

func (s TokenStore) Put(ctx context.Context, tokenHash string, purchaseID string) error {
	_ = ctx
	_ = tokenHash
	_ = purchaseID
	return nil
}

func (s TokenStore) Delete(ctx context.Context, tokenHash string) error {
	_ = ctx
	_ = tokenHash
	return nil
}
