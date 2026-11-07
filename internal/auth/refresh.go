package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// CryptoRefreshTokenGenerator создаёт 256 бит энтропии. Это opaque credential,
// не JWT. В отличие от слабого человеческого password, его можно искать по SHA-256.
type CryptoRefreshTokenGenerator struct{}

func NewCryptoRefreshTokenGenerator() *CryptoRefreshTokenGenerator {
	return &CryptoRefreshTokenGenerator{}
}

func (g *CryptoRefreshTokenGenerator) Generate() (string, error) {
	var bytes [32]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}

func (g *CryptoRefreshTokenGenerator) Hash(raw string) string {
	digest := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(digest[:])
}
