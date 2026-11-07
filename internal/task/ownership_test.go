package task

import (
	"context"
	"errors"
	"testing"
)

func TestCreateAssignsOwnerID(t *testing.T) {
	repo := &fakeRepository{
		countActiveFn: func(_ context.Context, ownerID int64) (int, error) {
			if ownerID != 7 {
				t.Fatalf("count owner=%d", ownerID)
			}
			return 0, nil
		},
		createFn: func(_ context.Context, ownerID int64, input CreateInput) (Task, error) {
			if ownerID != 7 || input.Title != "mine" {
				t.Fatalf("create owner=%d input=%+v", ownerID, input)
			}
			return Task{ID: 1, OwnerID: ownerID, Title: input.Title}, nil
		},
	}
	value, err := NewUseCase(repo).Create(context.Background(), 7, CreateInput{Title: " mine "})
	if err != nil || value.OwnerID != 7 {
		t.Fatalf("create=%+v, %v", value, err)
	}
}

func TestOwnerCanUpdateAndDelete(t *testing.T) {
	value := Task{ID: 1, OwnerID: 7, Title: "before"}
	deleted := false
	repo := &fakeRepository{
		getFn: func(context.Context, int64) (Task, error) { return value, nil },
		updateFn: func(_ context.Context, id int64, input UpdateInput) (Task, error) {
			value.Title = *input.Title
			return value, nil
		},
		deleteFn: func(context.Context, int64) error { deleted = true; return nil },
	}
	uc := NewUseCase(repo)
	own, err := uc.Get(context.Background(), 7, 1)
	if err != nil || own.OwnerID != 7 {
		t.Fatalf("get=%+v, %v", own, err)
	}
	title := "after"
	updated, err := uc.Update(context.Background(), 7, 1, UpdateInput{Title: &title})
	if err != nil || updated.Title != "after" || updated.OwnerID != 7 {
		t.Fatalf("update=%+v, %v", updated, err)
	}
	if err := uc.Delete(context.Background(), 7, 1); err != nil || !deleted {
		t.Fatalf("delete=%v deleted=%v", err, deleted)
	}
}

func TestAnotherUserGetsForbidden(t *testing.T) {
	// Write callbacks намеренно отсутствуют: любая запись в чужую Task сломает тест.
	repo := &fakeRepository{getFn: func(context.Context, int64) (Task, error) { return Task{ID: 1, OwnerID: 7, Title: "private"}, nil }}
	uc := NewUseCase(repo)
	done := true
	_, getErr := uc.Get(context.Background(), 8, 1)
	_, updateErr := uc.Update(context.Background(), 8, 1, UpdateInput{Done: &done})
	deleteErr := uc.Delete(context.Background(), 8, 1)
	for _, err := range []error{getErr, updateErr, deleteErr} {
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	}
}

func TestListContainsOnlyOwnerTasks(t *testing.T) {
	repo := &fakeRepository{listFn: func(_ context.Context, ownerID int64) ([]Task, error) {
		if ownerID != 7 {
			t.Fatalf("list scope=%d", ownerID)
		}
		return []Task{{ID: 1, OwnerID: 7, Title: "mine"}}, nil
	}}
	values, err := NewUseCase(repo).List(context.Background(), 7)
	if err != nil || len(values) != 1 || values[0].OwnerID != 7 {
		t.Fatalf("list=%+v, %v", values, err)
	}
}
