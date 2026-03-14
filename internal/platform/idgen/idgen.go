package idgen

import (
	"crypto/rand"
	"encoding/hex"
)

type Service struct{}

func NewService() Service {
	return Service{}
}

func (Service) NewID() (string, error) {
	return NewID()
}

func NewID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
