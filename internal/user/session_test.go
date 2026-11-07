package user

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// Fake фиксирует application-взаимодействия. SQL/locking проверяются отдельно
// настоящими PostgreSQL integration tests; fake не пытается эмулировать БД.
type fakeSessions struct {
	values                                  map[string]RefreshSession
	createErr, rotateErr, revokeErr, getErr error
	created, rotated, revoked               int
}

func newFakeSessions() *fakeSessions { return &fakeSessions{values: make(map[string]RefreshSession)} }
func (r *fakeSessions) Create(_ context.Context, s RefreshSession) (RefreshSession, error) {
	if r.createErr != nil {
		return RefreshSession{}, r.createErr
	}
	s.ID = int64(len(r.values) + 1)
	r.values[s.TokenHash] = s
	r.created++
	return s, nil
}
func (r *fakeSessions) GetByTokenHash(_ context.Context, hash string) (RefreshSession, error) {
	if r.getErr != nil {
		return RefreshSession{}, r.getErr
	}
	s, ok := r.values[hash]
	if !ok {
		return RefreshSession{}, ErrInvalidRefreshToken
	}
	return s, nil
}
func (r *fakeSessions) Rotate(_ context.Context, hash string, next RefreshSession) error {
	if r.rotateErr != nil {
		return r.rotateErr
	}
	old := r.values[hash]
	now := time.Now()
	old.RevokedAt = &now
	next.ID = int64(len(r.values) + 1)
	old.ReplacedBySessionID = &next.ID
	r.values[hash] = old
	r.values[next.TokenHash] = next
	r.rotated++
	return nil
}
func (r *fakeSessions) Revoke(_ context.Context, hash string) error {
	if r.revokeErr != nil {
		return r.revokeErr
	}
	if old, ok := r.values[hash]; ok {
		now := time.Now()
		old.RevokedAt = &now
		r.values[hash] = old
	}
	r.revoked++
	return nil
}

type fakeRefreshTokens struct {
	count int
	err   error
}

func (g *fakeRefreshTokens) Generate() (string, error) {
	if g.err != nil {
		return "", g.err
	}
	g.count++
	return fmt.Sprintf("raw-refresh-%d", g.count), nil
}
func (g *fakeRefreshTokens) Hash(raw string) string { return "sha256:" + raw }

type failingAccess struct{}

func (failingAccess) Issue(int64) (string, error) { return "", errors.New("signer unavailable") }

func sessionUseCase() (*UseCase, *fakeSessions, *fakeRefreshTokens) {
	sessions := newFakeSessions()
	generator := &fakeRefreshTokens{}
	uc := NewUseCase(&fakeRepository{byEmail: map[string]User{"alice@example.test": {ID: 7, Email: "alice@example.test", PasswordHash: "hash:secret123"}}}, fakeHasher{}, fakeTokens{}, sessions, generator, SessionConfig{AccessTTL: 15 * time.Minute, RefreshTTL: 7 * 24 * time.Hour})
	return uc, sessions, generator
}
func loginInput() LoginInput {
	return LoginInput{Email: "  ALICE@example.test ", Password: "secret123"}
}

func TestLoginPersistsOnlyRefreshHash(t *testing.T) {
	uc, sessions, generator := sessionUseCase()
	now := time.Date(2035, 1, 2, 3, 4, 5, 0, time.UTC)
	uc.now = func() time.Time { return now }
	pair, err := uc.Login(context.Background(), loginInput())
	if err != nil {
		t.Fatal(err)
	}
	saved := sessions.values[generator.Hash(pair.RefreshToken)]
	if pair.AccessToken == "" || pair.ExpiresIn != 900 || saved.UserID != 7 || saved.TokenHash == pair.RefreshToken || saved.TokenHash == "" {
		t.Fatal("login result/session contract")
	}
	if !saved.ExpiresAt.Equal(now.Add(7*24*time.Hour)) || !saved.ExpiresAt.Equal(pair.RefreshExpiresAt) || !saved.CreatedAt.Equal(now) {
		t.Fatal("session lifetime")
	}
	if saved.RevokedAt != nil || saved.ReplacedBySessionID != nil || sessions.created != 1 {
		t.Fatal("new session should be active")
	}
}
func TestRefreshRotatesAndLogoutRevokes(t *testing.T) {
	uc, sessions, generator := sessionUseCase()
	ctx := context.Background()
	first, err := uc.Login(ctx, loginInput())
	if err != nil {
		t.Fatal(err)
	}
	next, err := uc.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if next.RefreshToken == first.RefreshToken || next.AccessToken == "" || sessions.rotated != 1 {
		t.Fatal("missing new credentials")
	}
	old := sessions.values[generator.Hash(first.RefreshToken)]
	active := sessions.values[generator.Hash(next.RefreshToken)]
	if old.RevokedAt == nil || old.ReplacedBySessionID == nil || *old.ReplacedBySessionID != active.ID || active.RevokedAt != nil {
		t.Fatal("rotation state")
	}
	if _, err := uc.Refresh(ctx, first.RefreshToken); !errors.Is(err, ErrRefreshRevoked) {
		t.Fatalf("old refresh=%v", err)
	}
	if err := uc.Logout(ctx, next.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := uc.Refresh(ctx, next.RefreshToken); !errors.Is(err, ErrRefreshRevoked) {
		t.Fatalf("logout refresh=%v", err)
	}
	if err := uc.Logout(ctx, ""); err != nil {
		t.Fatal(err)
	}
	if err := uc.Logout(ctx, "unknown"); err != nil {
		t.Fatal(err)
	}
}
func TestRefreshRejectsUnknownExpiredRevoked(t *testing.T) {
	for _, kind := range []string{"missing", "unknown", "expired", "revoked"} {
		t.Run(kind, func(t *testing.T) {
			uc, sessions, generator := sessionUseCase()
			raw := "known"
			s := RefreshSession{UserID: 7, TokenHash: generator.Hash(raw), ExpiresAt: time.Now().Add(time.Hour)}
			want := ErrInvalidRefreshToken
			switch kind {
			case "missing":
				raw = ""
			case "expired":
				s.ExpiresAt = time.Now().Add(-time.Second)
				sessions.values[s.TokenHash] = s
				want = ErrRefreshExpired
			case "revoked":
				at := time.Now()
				s.RevokedAt = &at
				sessions.values[s.TokenHash] = s
				want = ErrRefreshRevoked
			}
			if _, err := uc.Refresh(context.Background(), raw); !errors.Is(err, want) {
				t.Fatalf("err=%v want=%v", err, want)
			}
			if generator.count != 0 || sessions.rotated != 0 {
				t.Fatal("invalid session must not issue/rotate")
			}
		})
	}
}
func TestSessionFailuresDoNotReturnCredentials(t *testing.T) {
	sentinel := errors.New("persistence unavailable")
	for _, kind := range []string{"create", "generator", "access"} {
		t.Run(kind, func(t *testing.T) {
			uc, sessions, generator := sessionUseCase()
			switch kind {
			case "create":
				sessions.createErr = sentinel
			case "generator":
				generator.err = sentinel
			case "access":
				uc.tokens = failingAccess{}
			}
			pair, err := uc.Login(context.Background(), loginInput())
			if err == nil || pair.AccessToken != "" || pair.RefreshToken != "" || sessions.created != 0 {
				t.Fatal("failed login returned credentials or saved session")
			}
		})
	}
	uc, sessions, generator := sessionUseCase()
	first, err := uc.Login(context.Background(), loginInput())
	if err != nil {
		t.Fatal(err)
	}
	sessions.rotateErr = ErrRefreshRevoked // another request won after GetByTokenHash
	result, err := uc.Refresh(context.Background(), first.RefreshToken)
	if !errors.Is(err, ErrRefreshRevoked) || result.AccessToken != "" || result.RefreshToken != "" || sessions.values[generator.Hash(first.RefreshToken)].RevokedAt != nil {
		t.Fatal("failed rotation returned credentials or changed old session")
	}
	sessions.revokeErr = sentinel
	if !errors.Is(uc.Logout(context.Background(), first.RefreshToken), sentinel) {
		t.Fatal("logout must propagate DB error")
	}
	sessions.getErr = sentinel
	if _, err := uc.Refresh(context.Background(), first.RefreshToken); !errors.Is(err, sentinel) {
		t.Fatal("lookup error mapping")
	}
}
