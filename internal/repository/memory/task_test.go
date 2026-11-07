package memory

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/rinatkh/classroom18/internal/task"
)

func TestTaskRepositoryLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := NewTaskRepository()
	empty, err := repo.List(ctx, 1)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty list: %#v, %v", empty, err)
	}
	first, err := repo.Create(ctx, 1, task.CreateInput{Title: "first"})
	if err != nil || first.ID != 1 || first.Done || first.CreatedAt.IsZero() {
		t.Fatalf("create: %+v, %v", first, err)
	}
	second, err := repo.Create(ctx, 1, task.CreateInput{Title: "second"})
	if err != nil || second.ID != 2 {
		t.Fatalf("second: %+v, %v", second, err)
	}
	done := true
	updated, err := repo.Update(ctx, first.ID, task.UpdateInput{Done: &done})
	if err != nil || !updated.Done || updated.Title != "first" || updated.CreatedAt != first.CreatedAt {
		t.Fatalf("update: %+v, %v", updated, err)
	}
	count, err := repo.CountActive(ctx, 1)
	if err != nil || count != 1 {
		t.Fatalf("count: %d, %v", count, err)
	}
	items, err := repo.List(ctx, 1)
	if err != nil || len(items) != 2 || items[0].ID != 1 || items[1].ID != 2 {
		t.Fatalf("list: %+v, %v", items, err)
	}
	items[0].Title = "changed outside repository"
	actual, err := repo.Get(ctx, 1)
	if err != nil || actual.Title != "first" {
		t.Fatalf("list leaked mutable state: %+v, %v", actual, err)
	}
	done = false
	updated, err = repo.Update(ctx, 1, task.UpdateInput{Done: &done})
	if err != nil || updated.Done || updated.Title != "first" {
		t.Fatalf("explicit false: %+v, %v", updated, err)
	}
	if err := repo.Delete(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, 2); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if _, err := repo.Update(ctx, 2, task.UpdateInput{Done: &done}); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("update missing: %v", err)
	}
	if err := repo.Delete(ctx, 2); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("delete missing: %v", err)
	}
}

func TestTaskRepositoryConcurrentCreate(t *testing.T) {
	repo := NewTaskRepository()
	const workers = 50
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.Create(context.Background(), 1, task.CreateInput{Title: "task"}); err != nil {
				t.Errorf("create: %v", err)
			}
		}()
	}
	wg.Wait()
	items, err := repo.List(context.Background(), 1)
	if err != nil || len(items) != workers {
		t.Fatalf("list: %d, %v", len(items), err)
	}
	for i, value := range items {
		if value.ID != int64(i+1) {
			t.Fatalf("duplicate or unordered ID: %+v", value)
		}
	}
}

func TestTaskRepositoryHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repo := NewTaskRepository()
	_, createErr := repo.Create(ctx, 1, task.CreateInput{Title: "task"})
	_, getErr := repo.Get(ctx, 1)
	_, listErr := repo.List(ctx, 1)
	_, updateErr := repo.Update(ctx, 1, task.UpdateInput{})
	deleteErr := repo.Delete(ctx, 1)
	_, countErr := repo.CountActive(ctx, 1)
	for _, err := range []error{createErr, getErr, listErr, updateErr, deleteErr, countErr} {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	}
}

func TestTaskRepositoryOwnerScope(t *testing.T) {
	repo := NewTaskRepository()
	ctx := context.Background()
	alice, err := repo.Create(ctx, 1, task.CreateInput{Title: "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	bob, err := repo.Create(ctx, 2, task.CreateInput{Title: "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	values, err := repo.List(ctx, 2)
	if err != nil || len(values) != 1 || values[0].ID != bob.ID {
		t.Fatalf("scope=%+v, %v", values, err)
	}
	done := true
	if _, err := repo.Update(ctx, alice.ID, task.UpdateInput{Done: &done}); err != nil {
		t.Fatal(err)
	}
	aliceActive, err := repo.CountActive(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	bobActive, err := repo.CountActive(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if aliceActive != 0 || bobActive != 1 {
		t.Fatalf("active Alice=%d Bob=%d", aliceActive, bobActive)
	}
}
