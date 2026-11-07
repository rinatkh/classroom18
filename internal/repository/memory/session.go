package memory

import (
	"context"
	"sync"
	"time"

	"github.com/rinatkh/classroom18/internal/user"
)

// MemoryRefreshSessionRepository нужен для быстрых application/HTTP tests.
// Production wiring использует PostgreSQL. Mutex делает Rotate атомарным.
type MemoryRefreshSessionRepository struct {
	mu     sync.RWMutex
	byHash map[string]user.RefreshSession
	nextID int64
	now    func() time.Time
}

var _ user.RefreshSessionRepository = (*MemoryRefreshSessionRepository)(nil)

func NewRefreshSessionRepository() *MemoryRefreshSessionRepository {
	return &MemoryRefreshSessionRepository{byHash: make(map[string]user.RefreshSession), nextID: 1, now: time.Now}
}

func copySession(s user.RefreshSession) user.RefreshSession {
	if s.RevokedAt != nil {
		value := *s.RevokedAt
		s.RevokedAt = &value
	}
	if s.ReplacedBySessionID != nil {
		value := *s.ReplacedBySessionID
		s.ReplacedBySessionID = &value
	}
	return s
}

func (r *MemoryRefreshSessionRepository) Create(ctx context.Context, s user.RefreshSession) (user.RefreshSession, error) {
	if err := ctx.Err(); err != nil {
		return user.RefreshSession{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byHash[s.TokenHash]; ok {
		return user.RefreshSession{}, user.ErrSessionAlreadyExists
	}
	s.ID = r.nextID
	r.nextID++
	r.byHash[s.TokenHash] = copySession(s)
	return copySession(s), nil
}

func (r *MemoryRefreshSessionRepository) GetByTokenHash(ctx context.Context, hash string) (user.RefreshSession, error) {
	if err := ctx.Err(); err != nil {
		return user.RefreshSession{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.byHash[hash]
	if !ok {
		return user.RefreshSession{}, user.ErrInvalidRefreshToken
	}
	return copySession(s), nil
}

func (r *MemoryRefreshSessionRepository) Rotate(ctx context.Context, hash string, next user.RefreshSession) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.byHash[hash]
	if !ok {
		return user.ErrInvalidRefreshToken
	}
	if old.RevokedAt != nil {
		return user.ErrRefreshRevoked
	}
	now := r.now().UTC()
	if !old.ExpiresAt.After(now) {
		return user.ErrRefreshExpired
	}
	if next.UserID != old.UserID || !next.ExpiresAt.After(now) {
		return user.ErrInvalidRefreshToken
	}
	if _, ok := r.byHash[next.TokenHash]; ok {
		return user.ErrSessionAlreadyExists
	}
	next.ID = r.nextID
	r.nextID++
	old.RevokedAt = &now
	old.ReplacedBySessionID = &next.ID
	r.byHash[hash] = copySession(old)
	r.byHash[next.TokenHash] = copySession(next)
	return nil
}

func (r *MemoryRefreshSessionRepository) Revoke(ctx context.Context, hash string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byHash[hash]
	if ok && s.RevokedAt == nil {
		now := r.now().UTC()
		s.RevokedAt = &now
		r.byHash[hash] = s
	}
	return nil
}
