//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rinatkh/classroom18/internal/auth"
	"github.com/rinatkh/classroom18/internal/user"
	"golang.org/x/crypto/bcrypt"
)

func TestIntegrationRefreshSessionLifecycle(t *testing.T) {
	pool, ctx := integrationPool(t)
	owner, err := NewUserRepository(pool).Create(ctx, user.User{Email: "alice@example.test", PasswordHash: "bcrypt-placeholder"})
	if err != nil {
		t.Fatal(err)
	}
	r := NewRefreshSessionRepository(pool)
	generator := auth.NewCryptoRefreshTokenGenerator()
	raw, err := generator.Generate()
	if err != nil {
		t.Fatal(err)
	}
	hash := generator.Hash(raw)
	old, err := r.Create(ctx, user.RefreshSession{UserID: owner.ID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || old.ID <= 0 || old.CreatedAt.IsZero() || old.UserID != owner.ID || old.TokenHash == raw {
		t.Fatal("create session failed")
	}
	found, err := r.GetByTokenHash(ctx, hash)
	if err != nil || found.ID != old.ID || found.RevokedAt != nil {
		t.Fatal("find active session failed")
	}
	var stored string
	if err := pool.QueryRow(ctx, `SELECT token_hash FROM refresh_sessions WHERE id=$1`, old.ID).Scan(&stored); err != nil || stored != hash || stored == raw {
		t.Fatal("raw credential in DB")
	}
	if _, err := r.Create(ctx, user.RefreshSession{UserID: owner.ID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}); !errors.Is(err, user.ErrSessionAlreadyExists) {
		t.Fatalf("duplicate hash=%v", err)
	}
	if _, err := r.GetByTokenHash(ctx, "missing"); !errors.Is(err, user.ErrInvalidRefreshToken) {
		t.Fatalf("unknown=%v", err)
	}
	if err := r.Rotate(ctx, hash, user.RefreshSession{UserID: owner.ID, TokenHash: "next-hash", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	old, err = r.GetByTokenHash(ctx, hash)
	if err != nil {
		t.Fatal(err)
	}
	next, err := r.GetByTokenHash(ctx, "next-hash")
	if err != nil {
		t.Fatal(err)
	}
	if old.RevokedAt == nil || old.ReplacedBySessionID == nil || *old.ReplacedBySessionID != next.ID || next.RevokedAt != nil {
		t.Fatal("rotation state not persisted")
	}
	if err := r.Rotate(ctx, hash, user.RefreshSession{UserID: owner.ID, TokenHash: "illegal", ExpiresAt: time.Now().Add(time.Hour)}); !errors.Is(err, user.ErrRefreshRevoked) {
		t.Fatalf("reuse=%v", err)
	}
	if err := r.Revoke(ctx, "next-hash"); err != nil {
		t.Fatal(err)
	}
	if err := r.Revoke(ctx, "next-hash"); err != nil {
		t.Fatal(err)
	}
	if err := r.Revoke(ctx, "unknown"); err != nil {
		t.Fatal(err)
	}
	next, _ = r.GetByTokenHash(ctx, "next-hash")
	if next.RevokedAt == nil {
		t.Fatal("logout not persisted")
	}
	expired, err := r.Create(ctx, user.RefreshSession{UserID: owner.ID, TokenHash: "expired", ExpiresAt: time.Now().Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Rotate(ctx, expired.TokenHash, user.RefreshSession{UserID: owner.ID, TokenHash: "expired-next", ExpiresAt: time.Now().Add(time.Hour)}); !errors.Is(err, user.ErrRefreshExpired) {
		t.Fatalf("expired=%v", err)
	}
}

func TestIntegrationRefreshSessionConstraintsAndCascade(t *testing.T) {
	pool, ctx := integrationPool(t)
	r := NewRefreshSessionRepository(pool)
	if _, err := r.Create(ctx, user.RefreshSession{UserID: 999, TokenHash: "orphan", ExpiresAt: time.Now().Add(time.Hour)}); err == nil {
		t.Fatal("missing FK")
	} else {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
			t.Fatalf("FK=%v", err)
		}
	}
	owner, err := NewUserRepository(pool).Create(ctx, user.User{Email: "owner@example.test", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := r.Create(ctx, user.RefreshSession{UserID: owner.ID, TokenHash: "first", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Rotate(ctx, first.TokenHash, user.RefreshSession{UserID: owner.ID, TokenHash: "second", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, owner.ID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM refresh_sessions`).Scan(&count); err != nil || count != 0 {
		t.Fatal("user cascade must remove entire rotated chain")
	}
}

func TestIntegrationRotationRollbackAndIdentity(t *testing.T) {
	pool, ctx := integrationPool(t)
	r := NewRefreshSessionRepository(pool)
	owner, err := NewUserRepository(pool).Create(ctx, user.User{Email: "owner@example.test", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	for _, hash := range []string{"old", "existing"} {
		if _, err := r.Create(ctx, user.RefreshSession{UserID: owner.ID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	// INSERT нарушит UNIQUE ПОСЛЕ UPDATE revoked_at. Транзакция обязана откатить UPDATE.
	if err := r.Rotate(ctx, "old", user.RefreshSession{UserID: owner.ID, TokenHash: "existing", ExpiresAt: time.Now().Add(time.Hour)}); !errors.Is(err, user.ErrSessionAlreadyExists) {
		t.Fatalf("duplicate during rotation=%v", err)
	}
	old, err := r.GetByTokenHash(ctx, "old")
	if err != nil || old.RevokedAt != nil || old.ReplacedBySessionID != nil {
		t.Fatal("failed insert revoked old session")
	}
	if err := r.Rotate(ctx, "old", user.RefreshSession{UserID: owner.ID + 1, TokenHash: "other-owner", ExpiresAt: time.Now().Add(time.Hour)}); !errors.Is(err, user.ErrInvalidRefreshToken) {
		t.Fatalf("owner substitution=%v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM refresh_sessions`).Scan(&count); err != nil || count != 2 {
		t.Fatal("failed rotation left partial row")
	}
	// Старый credential по-прежнему работает после rollback.
	if err := r.Rotate(ctx, "old", user.RefreshSession{UserID: owner.ID, TokenHash: "retry-new", ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
}

func TestIntegrationConcurrentRefreshOnlyOneSuccess(t *testing.T) {
	pool, ctx := integrationPool(t)
	r := NewRefreshSessionRepository(pool)
	tokens := auth.NewJWTManager("0123456789abcdef0123456789abcdef", 15*time.Minute)
	uc := user.NewUseCase(NewUserRepository(pool), auth.NewBcryptHasher(bcrypt.MinCost), tokens, r, auth.NewCryptoRefreshTokenGenerator(), user.SessionConfig{AccessTTL: 15 * time.Minute, RefreshTTL: 7 * 24 * time.Hour})
	if _, err := uc.Register(ctx, user.RegisterInput{Email: "alice@example.test", Password: "secret123"}); err != nil {
		t.Fatal(err)
	}
	first, err := uc.Login(ctx, user.LoginInput{Email: "alice@example.test", Password: "secret123"})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; _, err := uc.Refresh(ctx, first.RefreshToken); results <- err }()
	}
	close(start)
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else if !errors.Is(err, user.ErrRefreshRevoked) {
			t.Fatalf("loser=%v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successes=%d want exactly one", successes)
	}
	var total, active int
	if err := pool.QueryRow(ctx, `SELECT count(*), count(*) FILTER (WHERE revoked_at IS NULL) FROM refresh_sessions`).Scan(&total, &active); err != nil || total != 2 || active != 1 {
		t.Fatalf("total=%d active=%d", total, active)
	}
}

func TestIntegrationConcurrentRepositoryRotateOnlyOneSuccess(t *testing.T) {
	pool, ctx := integrationPool(t)
	r := NewRefreshSessionRepository(pool)
	owner, err := NewUserRepository(pool).Create(ctx, user.User{Email: "owner@example.test", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	// Прямые repository calls исключают возможность пройти тест только за счёт
	// предварительного UseCase.Get: безопасность гарантирует именно SQL operation.
	for round := 0; round < 5; round++ {
		hash := fmt.Sprintf("old-%d", round)
		if _, err := r.Create(ctx, user.RefreshSession{UserID: owner.ID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		results := make(chan error, 2)
		var wg sync.WaitGroup
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				results <- r.Rotate(ctx, hash, user.RefreshSession{UserID: owner.ID, TokenHash: fmt.Sprintf("next-%d-%d", round, i), ExpiresAt: time.Now().Add(time.Hour)})
			}(i)
		}
		close(start)
		wg.Wait()
		close(results)
		success := 0
		for err := range results {
			if err == nil {
				success++
			} else if !errors.Is(err, user.ErrRefreshRevoked) {
				t.Fatal(err)
			}
		}
		if success != 1 {
			t.Fatal("double SQL rotation")
		}
	}
}

func TestIntegrationSessionContextCancellation(t *testing.T) {
	pool, ctx := integrationPool(t)
	r := NewRefreshSessionRepository(pool)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := r.Create(canceled, user.RefreshSession{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Create=%v", err)
	}
	if _, err := r.GetByTokenHash(canceled, "hash"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Get=%v", err)
	}
	if err := r.Rotate(canceled, "hash", user.RefreshSession{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Rotate=%v", err)
	}
	if err := r.Revoke(canceled, "hash"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Revoke=%v", err)
	}
}

// Уже принятое application-решение может устареть, пока SQL ждёт другой request.
// Держим row lock, отзываем session, запускаем Rotate и дожидаемся DB lock wait.
// После commit ожидающий Rotate обязан перечитать revoked и отказать.
func TestIntegrationRotationRechecksAfterWaitingForLock(t *testing.T) {
	pool, ctx := integrationPool(t)
	owner, err := NewUserRepository(pool).Create(ctx, user.User{Email: "owner@example.test", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	r := NewRefreshSessionRepository(pool)
	old, err := r.Create(ctx, user.RefreshSession{UserID: owner.ID, TokenHash: "old", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Rollback(ctx)
	if _, err := blocker.Exec(ctx, `UPDATE refresh_sessions SET revoked_at=clock_timestamp() WHERE id=$1`, old.ID); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		done <- r.Rotate(ctx, "old", user.RefreshSession{UserID: owner.ID, TokenHash: "new", ExpiresAt: time.Now().Add(time.Hour)})
	}()
	// Ожидаем реальную блокировку в БД, без sleep и привязки к истечению часов.
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity
    WHERE datname=current_database() AND pid<>pg_backend_pid()
    AND wait_event_type='Lock' AND query LIKE '%refresh_sessions%')`).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("Rotate did not wait for lock: %v", err)
		default:
		}
	}
	if err := blocker.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, user.ErrRefreshRevoked) {
		t.Fatalf("stale state accepted after lock wait: %v", err)
	}
	if _, err := r.GetByTokenHash(ctx, "new"); !errors.Is(err, user.ErrInvalidRefreshToken) {
		t.Fatal("unexpected descendant")
	}
}
