package memory

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/rinatkh/classroom18/internal/user"
)

func TestUserRepositoryConcurrentDuplicate(t *testing.T) {
	repo := NewUserRepository()
	var created atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Create(context.Background(), user.User{Email: "alice@example.com", PasswordHash: "hash"})
			if err == nil {
				created.Add(1)
			} else if !errors.Is(err, user.ErrAlreadyExists) {
				t.Errorf("create=%v", err)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 {
		t.Fatalf("created=%d", created.Load())
	}
	value, err := repo.GetByEmail(context.Background(), "ALICE@example.com")
	if err != nil || value.ID != 1 || value.PasswordHash != "hash" {
		t.Fatalf("user=%+v, %v", value, err)
	}
}
