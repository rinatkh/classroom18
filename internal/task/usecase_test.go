package task

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	createFn      func(context.Context, int64, CreateInput) (Task, error)
	getFn         func(context.Context, int64) (Task, error)
	listFn        func(context.Context, int64) ([]Task, error)
	updateFn      func(context.Context, int64, UpdateInput) (Task, error)
	deleteFn      func(context.Context, int64) error
	countActiveFn func(context.Context, int64) (int, error)
}

func (f *fakeRepository) Create(ctx context.Context, ownerID int64, input CreateInput) (Task, error) {
	return f.createFn(ctx, ownerID, input)
}

func (f *fakeRepository) Get(ctx context.Context, id int64) (Task, error) {
	return f.getFn(ctx, id)
}

func (f *fakeRepository) List(ctx context.Context, ownerID int64) ([]Task, error) {
	return f.listFn(ctx, ownerID)
}

func (f *fakeRepository) Update(
	ctx context.Context,
	id int64,
	input UpdateInput,
) (Task, error) {
	return f.updateFn(ctx, id, input)
}

func (f *fakeRepository) Delete(ctx context.Context, id int64) error {
	return f.deleteFn(ctx, id)
}

func (f *fakeRepository) CountActive(ctx context.Context, ownerID int64) (int, error) {
	return f.countActiveFn(ctx, ownerID)
}

func TestUseCaseCreateRejectsEmptyTitleWithoutRepositoryCall(t *testing.T) {
	repositoryCalled := false
	repository := &fakeRepository{
		countActiveFn: func(context.Context, int64) (int, error) {
			repositoryCalled = true
			return 0, nil
		},
	}

	_, err := NewUseCase(repository).Create(context.Background(), 1, CreateInput{Title: "   "})
	if !errors.Is(err, ErrEmptyTitle) {
		t.Fatalf("error=%v, want ErrEmptyTitle", err)
	}
	if repositoryCalled {
		t.Fatal("repository не должен вызываться после ошибки business validation")
	}
}

func TestUseCaseCreateSuccess(t *testing.T) {
	repository := &fakeRepository{
		countActiveFn: func(context.Context, int64) (int, error) { return 3, nil },
		createFn: func(_ context.Context, ownerID int64, input CreateInput) (Task, error) {
			if ownerID != 1 || input.Title != "Learn architecture" {
				t.Fatalf("title=%q", input.Title)
			}
			return Task{OwnerID: 1, ID: 42, Title: input.Title}, nil
		},
	}

	result, err := NewUseCase(repository).Create(
		context.Background(), 1,
		CreateInput{Title: "  Learn architecture  "},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.ID != 42 || result.Title != "Learn architecture" {
		t.Fatalf("result=%+v", result)
	}
}

func TestUseCaseCreateWrapsRepositoryError(t *testing.T) {
	storageErr := errors.New("storage unavailable")
	repository := &fakeRepository{
		countActiveFn: func(context.Context, int64) (int, error) { return 0, nil },
		createFn: func(context.Context, int64, CreateInput) (Task, error) {
			return Task{}, storageErr
		},
	}

	_, err := NewUseCase(repository).Create(
		context.Background(), 1,
		CreateInput{Title: "Task"},
	)
	if !errors.Is(err, storageErr) {
		t.Fatalf("error=%v, want wrapped storage error", err)
	}
}

func TestUseCaseCreateRejectsActiveLimit(t *testing.T) {
	createCalled := false
	repository := &fakeRepository{
		countActiveFn: func(context.Context, int64) (int, error) { return maxActiveTasks, nil },
		createFn: func(context.Context, int64, CreateInput) (Task, error) {
			createCalled = true
			return Task{}, nil
		},
	}

	_, err := NewUseCase(repository).Create(
		context.Background(), 1,
		CreateInput{Title: "One more task"},
	)
	if !errors.Is(err, ErrActiveLimit) {
		t.Fatalf("error=%v, want ErrActiveLimit", err)
	}
	if createCalled {
		t.Fatal("Create repository не должен вызываться при превышении лимита")
	}
}

func TestUseCaseUpdateRejectsAlreadyCompleted(t *testing.T) {
	updateCalled := false
	done := true
	repository := &fakeRepository{
		getFn: func(context.Context, int64) (Task, error) {
			return Task{OwnerID: 1, ID: 7, Done: true}, nil
		},
		updateFn: func(context.Context, int64, UpdateInput) (Task, error) {
			updateCalled = true
			return Task{}, nil
		},
	}

	_, err := NewUseCase(repository).Update(
		context.Background(), 1,
		7,
		UpdateInput{Done: &done},
	)
	if !errors.Is(err, ErrAlreadyComplete) {
		t.Fatalf("error=%v, want ErrAlreadyComplete", err)
	}
	if updateCalled {
		t.Fatal("Update repository не должен вызываться после business conflict")
	}
}

func TestUseCaseUpdateRejectsNoChangesWithoutRepositoryCall(t *testing.T) {
	_, err := NewUseCase(&fakeRepository{}).Update(context.Background(), 1, 1, UpdateInput{})
	if !errors.Is(err, ErrNoChanges) {
		t.Fatalf("error=%v", err)
	}
}

func TestUseCaseUpdateRejectsEmptyTitleWithoutWrite(t *testing.T) {
	title := "   "
	repo := &fakeRepository{getFn: func(context.Context, int64) (Task, error) { return Task{OwnerID: 1, ID: 1}, nil }}
	_, err := NewUseCase(repo).Update(context.Background(), 1, 1, UpdateInput{Title: &title})
	if !errors.Is(err, ErrEmptyTitle) {
		t.Fatalf("error=%v", err)
	}
}

func TestUseCaseUpdatePreservesPartialChangesAndFalse(t *testing.T) {
	title := "  renamed  "
	done := false
	repo := &fakeRepository{
		getFn: func(context.Context, int64) (Task, error) {
			return Task{OwnerID: 1, ID: 1, Title: "before", Done: true}, nil
		},
		countActiveFn: func(context.Context, int64) (int, error) { return 0, nil },
		updateFn: func(_ context.Context, id int64, input UpdateInput) (Task, error) {
			if id != 1 || input.Title == nil || *input.Title != "renamed" || input.Done == nil || *input.Done {
				t.Fatalf("input=%+v", input)
			}
			return Task{OwnerID: 1, ID: id, Title: *input.Title, Done: *input.Done}, nil
		},
	}
	result, err := NewUseCase(repo).Update(context.Background(), 1, 1, UpdateInput{Title: &title, Done: &done})
	if err != nil || result.Done || result.Title != "renamed" {
		t.Fatalf("result=%+v, %v", result, err)
	}
}

func TestUseCaseUpdateMissingTask(t *testing.T) {
	done := true
	repo := &fakeRepository{getFn: func(context.Context, int64) (Task, error) { return Task{}, ErrNotFound }}
	_, err := NewUseCase(repo).Update(context.Background(), 1, 999, UpdateInput{Done: &done})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error=%v", err)
	}
}

func TestUseCaseReopenRejectsActiveLimit(t *testing.T) {
	done := false
	repo := &fakeRepository{
		getFn:         func(context.Context, int64) (Task, error) { return Task{OwnerID: 1, ID: 1, Done: true}, nil },
		countActiveFn: func(context.Context, int64) (int, error) { return maxActiveTasks, nil },
	}
	_, err := NewUseCase(repo).Update(context.Background(), 1, 1, UpdateInput{Done: &done})
	if !errors.Is(err, ErrActiveLimit) {
		t.Fatalf("error=%v", err)
	}
}

func TestUseCaseWrapsReadAndDeleteErrors(t *testing.T) {
	failure := errors.New("storage unavailable")
	repo := &fakeRepository{
		countActiveFn: func(context.Context, int64) (int, error) { return 0, failure },
		getFn:         func(context.Context, int64) (Task, error) { return Task{}, failure },
		listFn:        func(context.Context, int64) ([]Task, error) { return nil, failure },
		deleteFn:      func(context.Context, int64) error { return failure },
	}
	uc := NewUseCase(repo)
	_, createErr := uc.Create(context.Background(), 1, CreateInput{Title: "task"})
	_, getErr := uc.Get(context.Background(), 1, 1)
	_, listErr := uc.List(context.Background(), 1)
	deleteErr := uc.Delete(context.Background(), 1, 1)
	for _, err := range []error{createErr, getErr, listErr, deleteErr} {
		if !errors.Is(err, failure) {
			t.Fatalf("lost storage cause: %v", err)
		}
	}
}
