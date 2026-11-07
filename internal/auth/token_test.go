package auth

import (
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"testing"
	"time"
)

func TestJWTManagerRoundTrip(t *testing.T) {
	manager := NewJWTManager("0123456789abcdef0123456789abcdef", time.Hour)
	manager.now = func() time.Time { return time.Unix(1_700_000_000, 0) }
	token, err := manager.Issue(42)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	userID, err := manager.Verify(token)
	if err != nil || userID != 42 {
		t.Fatalf("Verify() = %d, %v", userID, err)
	}
}

func TestJWTManagerRejectsExpiredToken(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	manager := NewJWTManager("0123456789abcdef0123456789abcdef", time.Minute)
	manager.now = func() time.Time { return now }
	token, err := manager.Issue(42)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	manager.now = func() time.Time { return now.Add(2 * time.Minute) }
	_, err = manager.Verify(token)
	if !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("Verify() error = %v, want ErrInvalidToken", err)
	}
}

func TestJWTManagerRejectsInvalidTokens(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	now := time.Unix(1_700_000_000, 0)
	manager := NewJWTManager(secret, time.Hour)
	manager.now = func() time.Time { return now }
	claims := Claims{UserID: 42, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour))}}
	sign := func(method jwt.SigningMethod, key any, c Claims) string {
		t.Helper()
		raw, err := jwt.NewWithClaims(method, c).SignedString(key)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	for _, tt := range []struct{ name, raw string }{
		{"invalid signature", sign(jwt.SigningMethodHS256, []byte("different-secret-0123456789abcdef"), claims)},
		{"wrong algorithm", sign(jwt.SigningMethodHS384, []byte(secret), claims)},
		{"none algorithm", sign(jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType, claims)},
		{"missing expiration", sign(jwt.SigningMethodHS256, []byte(secret), Claims{UserID: 42})},
		{"invalid user ID", sign(jwt.SigningMethodHS256, []byte(secret), Claims{RegisteredClaims: claims.RegisteredClaims})},
		{"malformed", "not.a.jwt"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := manager.Verify(tt.raw); !errors.Is(err, ErrInvalidToken) {
				t.Fatalf("Verify=%v", err)
			}
		})
	}
}

func TestJWTManagerRejectsInvalidIssuingConfig(t *testing.T) {
	for _, m := range []*JWTManager{NewJWTManager("short", time.Hour), NewJWTManager("0123456789abcdef0123456789abcdef", 0)} {
		if _, err := m.Issue(1); !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("Issue=%v", err)
		}
	}
	if _, err := NewJWTManager("0123456789abcdef0123456789abcdef", time.Hour).Issue(0); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("zero user=%v", err)
	}
}
