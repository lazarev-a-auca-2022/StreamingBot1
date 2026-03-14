package redis

import (
	"context"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type TokenStore struct {
	client *goredis.Client
}

func NewTokenStore(redisURL string) (*TokenStore, error) {
	opts, err := goredis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := goredis.NewClient(opts)
	return &TokenStore{client: client}, nil
}

func (s *TokenStore) Ping(ctx context.Context) error {
	return s.client.Ping(ctx).Err()
}

func (s *TokenStore) Close() error {
	return s.client.Close()
}

func (s *TokenStore) Put(ctx context.Context, tokenHash string, purchaseID string, ttl time.Duration) error {
	return s.client.Set(ctx, s.key(tokenHash), purchaseID, ttl).Err()
}

func (s *TokenStore) Get(ctx context.Context, tokenHash string) (string, error) {
	v, err := s.client.Get(ctx, s.key(tokenHash)).Result()
	if errors.Is(err, goredis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v, nil
}

func (s *TokenStore) Delete(ctx context.Context, tokenHash string) error {
	return s.client.Del(ctx, s.key(tokenHash)).Err()
}

func (s *TokenStore) key(tokenHash string) string {
	return fmt.Sprintf("access:token_hash:%s", tokenHash)
}
