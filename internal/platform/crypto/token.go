package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

type TokenService struct{}

func NewTokenService() TokenService {
	return TokenService{}
}

func (s TokenService) Generate() (raw string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(buf)
	digest := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(digest[:])
	return raw, hash, nil
}
