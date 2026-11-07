package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type Claims struct {
	UserID int64 `json:"user_id"`
	jwt.RegisteredClaims
}

// JWTManager реализует два маленьких контракта: TokenIssuer для AuthUseCase и
// TokenVerifier для HTTP middleware.
type JWTManager struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewJWTManager(secret string, ttl time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), ttl: ttl, now: time.Now}
}

func (m *JWTManager) Issue(userID int64) (string, error) {
	if len(m.secret) < 32 || m.ttl <= 0 || userID <= 0 {
		return "", ErrInvalidToken
	}
	now := m.now()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *JWTManager) Verify(raw string) (int64, error) {
	if len(m.secret) < 32 {
		return 0, ErrInvalidToken
	}
	claims := new(Claims)
	token, err := jwt.ParseWithClaims(
		raw,
		claims,
		func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, ErrInvalidToken
			}
			return m.secret, nil
		},
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(m.now),
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil || !token.Valid || claims.UserID <= 0 {
		return 0, ErrInvalidToken
	}
	return claims.UserID, nil
}
