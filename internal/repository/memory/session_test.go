package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rinatkh/classroom18/internal/user"
)

func TestSessionAtomicRotationAndSnapshots(t *testing.T) {
	r := NewRefreshSessionRepository()
	ctx := context.Background()
	old, err := r.Create(ctx, user.RefreshSession{UserID: 1, TokenHash: "old", ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for _, hash := range []string{"new-A", "new-B"} {
		wg.Add(1)
		go func(hash string) {
			defer wg.Done()
			<-start
			results <- r.Rotate(ctx, "old", user.RefreshSession{UserID: 1, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)})
		}(hash)
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
	if success != 1 || len(r.byHash) != 2 {
		t.Fatal("double rotation")
	}
	stored, err := r.GetByTokenHash(ctx, "old")
	if err != nil {
		t.Fatal(err)
	}
	if stored.ID != old.ID || stored.RevokedAt == nil || stored.ReplacedBySessionID == nil {
		t.Fatal("missing old state")
	}
	*stored.RevokedAt = time.Time{}
	*stored.ReplacedBySessionID = 999
	copied, _ := r.GetByTokenHash(ctx, "old")
	if copied.RevokedAt.IsZero() || *copied.ReplacedBySessionID == 999 {
		t.Fatal("shared pointers escaped repository")
	}
	if err := r.Revoke(ctx, "missing"); err != nil {
		t.Fatal(err)
	}
}
