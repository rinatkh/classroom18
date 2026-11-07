package user

import (
	"context"
	"time"
)

// RefreshSession хранит только hash credential. HTTP cookie здесь нет.
type RefreshSession struct {
	ID                  int64
	UserID              int64
	TokenHash           string
	ExpiresAt           time.Time
	CreatedAt           time.Time
	RevokedAt           *time.Time
	ReplacedBySessionID *int64
}

// Rotate обязан повторно проверить старую session под блокировкой и атомарно
// отозвать её + сохранить новую. Одно чтение GetByTokenHash этого не гарантирует.
// Revoke идемпотентен: неизвестная/уже отозванная session не является ошибкой.
type RefreshSessionRepository interface {
	Create(ctx context.Context, session RefreshSession) (RefreshSession, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (RefreshSession, error)
	Rotate(ctx context.Context, oldTokenHash string, newSession RefreshSession) error
	Revoke(ctx context.Context, tokenHash string) error
}

type RefreshTokenGenerator interface {
	Generate() (string, error)
	Hash(raw string) string
}

type SessionConfig struct {
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}
