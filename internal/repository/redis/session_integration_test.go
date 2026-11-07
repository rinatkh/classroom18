//go:build integration

package redis

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	goredis "github.com/redis/go-redis/v9"
	"github.com/rinatkh/classroom18/internal/auth"
	"github.com/rinatkh/classroom18/internal/user"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func integrationRepo(t *testing.T) (*RefreshSessionRepository, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Fatal("TEST_REDIS_URL required")
	}
	options, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	options.ContextTimeoutEnabled = true
	client := goredis.NewClient(options)
	t.Cleanup(func() { _ = client.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	if err := client.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	repo := NewRefreshSessionRepository(client)
	repo.prefix = "test:" + hex.EncodeToString(suffix[:]) + ":{refresh}:"
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		keys, _ := client.Keys(ctx, repo.prefix+"*").Result()
		if len(keys) > 0 {
			_ = client.Del(ctx, keys...).Err()
		}
	})
	return repo, ctx
}
func session(hash string, ttl time.Duration) user.RefreshSession {
	return user.RefreshSession{UserID: 7, TokenHash: auth.NewCryptoRefreshTokenGenerator().Hash(hash), ExpiresAt: time.Now().Add(ttl), CreatedAt: time.Now()}
}
func TestIntegrationRedisLifecycleAndHashOnly(t *testing.T) {
	repo, ctx := integrationRepo(t)
	input := session("raw-secret-one", time.Minute)
	input.UserID = (1 << 54) + 1 // Redis hash strings preserve int64 IDs.
	saved, err := repo.Create(ctx, input)
	if err != nil || saved.ID <= 0 {
		t.Fatalf("create %v %v", saved, err)
	}
	got, err := repo.GetByTokenHash(ctx, input.TokenHash)
	if err != nil || got.UserID != input.UserID {
		t.Fatalf("get %v %v", got, err)
	}
	ttl, err := repo.client.PTTL(ctx, repo.key(input.TokenHash)).Result()
	if err != nil || ttl <= 0 || ttl > time.Minute {
		t.Fatal(ttl, err)
	}
	fields, _ := repo.client.HGetAll(ctx, repo.key(input.TokenHash)).Result()
	for _, v := range fields {
		if v == "raw-secret-one" {
			t.Fatal("raw token stored")
		}
	}
	if _, err := repo.Create(ctx, input); !errors.Is(err, user.ErrSessionAlreadyExists) {
		t.Fatal(err)
	}
	next := session("raw-next", time.Minute)
	next.UserID = input.UserID
	if err := repo.Rotate(ctx, input.TokenHash, next); err != nil {
		t.Fatal(err)
	}
	old, _ := repo.GetByTokenHash(ctx, input.TokenHash)
	child, err := repo.GetByTokenHash(ctx, next.TokenHash)
	if err != nil || old.RevokedAt == nil || old.ReplacedBySessionID == nil || *old.ReplacedBySessionID != child.ID {
		t.Fatal("rotation state")
	}
	if err := repo.Rotate(ctx, input.TokenHash, session("replay", time.Minute)); !errors.Is(err, user.ErrRefreshRevoked) {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := repo.Revoke(ctx, next.TokenHash); err != nil {
			t.Fatal(err)
		}
	}
	child, _ = repo.GetByTokenHash(ctx, next.TokenHash)
	if child.RevokedAt == nil {
		t.Fatal("not revoked")
	}
	if err := repo.Revoke(ctx, "unknown"); err != nil {
		t.Fatal(err)
	}
}
func TestIntegrationRedisExpiration(t *testing.T) {
	repo, ctx := integrationRepo(t)
	v := session("expiring", 120*time.Millisecond)
	if _, err := repo.Create(ctx, v); err != nil {
		t.Fatal(err)
	}
	// Настоящий TTL: ждём observable удаления ключа, а не точную длительность scheduler.
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		_, err := repo.GetByTokenHash(ctx, v.TokenHash)
		if errors.Is(err, user.ErrInvalidRefreshToken) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-tick.C:
		case <-ctx.Done():
			t.Fatal("TTL did not expire")
		}
	}
	if err := repo.Rotate(ctx, v.TokenHash, session("late", time.Minute)); !errors.Is(err, user.ErrInvalidRefreshToken) {
		t.Fatal(err)
	}
	if _, err := repo.Create(ctx, session("already-expired", -time.Second)); !errors.Is(err, user.ErrRefreshExpired) {
		t.Fatal(err)
	}
}
func TestIntegrationRedisConcurrentRotationOnlyOneSuccess(t *testing.T) {
	repo, ctx := integrationRepo(t)
	old := session("old", time.Minute)
	if _, err := repo.Create(ctx, old); err != nil {
		t.Fatal(err)
	}
	var successes atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 20; i++ {
		next := session(time.Now().String()+string(rune('a'+i)), time.Minute)
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := repo.Rotate(ctx, old.TokenHash, next)
			if err == nil {
				successes.Add(1)
			} else if !errors.Is(err, user.ErrRefreshRevoked) {
				t.Errorf("unexpected %v", err)
			}
		}()
	}
	close(start)
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("successes=%d", successes.Load())
	}
	keys, _ := repo.client.Keys(ctx, repo.prefix+"*").Result()
	if len(keys) != 3 {
		t.Fatalf("old + child + sequence required: %v", keys)
	}
}
func TestIntegrationRedisCollisionAndWrongOwnerAreAtomic(t *testing.T) {
	repo, ctx := integrationRepo(t)
	old := session("old", time.Minute)
	next := session("collision", time.Minute)
	_, _ = repo.Create(ctx, old)
	_, _ = repo.Create(ctx, next)
	if err := repo.Rotate(ctx, old.TokenHash, next); !errors.Is(err, user.ErrSessionAlreadyExists) {
		t.Fatal(err)
	}
	next = session("wrong-owner", time.Minute)
	next.UserID = 8
	if err := repo.Rotate(ctx, old.TokenHash, next); !errors.Is(err, user.ErrInvalidRefreshToken) {
		t.Fatal(err)
	}
	got, _ := repo.GetByTokenHash(ctx, old.TokenHash)
	if got.RevokedAt != nil {
		t.Fatal("failed rotation consumed valid token")
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := repo.Create(canceled, session("canceled", time.Minute)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
