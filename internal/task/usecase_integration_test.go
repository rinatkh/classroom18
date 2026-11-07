package task_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/rinatkh/classroom18/internal/repository/memory"
	"github.com/rinatkh/classroom18/internal/task"
)

// Проверяем business invariant через один UseCase, как в composition root.
func TestUseCaseConcurrentCreateRespectsActiveLimit(t *testing.T) {
	repo := memory.NewTaskRepository()
	uc := task.NewUseCase(repo)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := uc.Create(context.Background(), 1, task.CreateInput{Title: "task"})
			if err != nil && !errors.Is(err, task.ErrActiveLimit) {
				t.Errorf("create: %v", err)
			}
		}()
	}
	wg.Wait()
	items, err := uc.List(context.Background(), 1)
	if err != nil || len(items) != 20 {
		t.Fatalf("active limit: %d, %v", len(items), err)
	}
}

func TestActiveLimitIsPerOwner(t *testing.T) {
	uc := task.NewUseCase(memory.NewTaskRepository())
	ctx := context.Background()
	for i := 0; i < 20; i++ {
		if _, err := uc.Create(ctx, 1, task.CreateInput{Title: "Alice"}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := uc.Create(ctx, 1, task.CreateInput{Title: "over limit"}); !errors.Is(err, task.ErrActiveLimit) {
		t.Fatalf("Alice limit=%v", err)
	}
	bob, err := uc.Create(ctx, 2, task.CreateInput{Title: "Bob"})
	if err != nil || bob.OwnerID != 2 {
		t.Fatalf("Bob should have his own limit: %+v, %v", bob, err)
	}
	values, err := uc.List(ctx, 2)
	if err != nil || len(values) != 1 || values[0].ID != bob.ID {
		t.Fatalf("Bob list=%+v, %v", values, err)
	}
}
