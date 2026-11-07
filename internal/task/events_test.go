package task_test

import (
	"context"
	"errors"
	"github.com/rinatkh/classroom18/internal/repository/memory"
	"github.com/rinatkh/classroom18/internal/task"
	"testing"
	"time"
)

type publisher struct {
	events []task.Event
	err    error
	ctx    context.Context
}

func (p *publisher) Publish(ctx context.Context, e task.Event) error {
	p.ctx = ctx
	p.events = append(p.events, e)
	return p.err
}
func TestCreatePublishesSavedTask(t *testing.T) {
	repo := memory.NewTaskRepository()
	out := &publisher{}
	ctx := context.WithValue(context.Background(), struct{}{}, "request")
	value, err := task.NewUseCaseWithEvents(repo, out).Create(ctx, 7, task.CreateInput{Title: "  Learn Kafka "})
	if err != nil {
		t.Fatal(err)
	}
	if value.ProcessingStatus != task.Pending || len(out.events) != 1 {
		t.Fatalf("task/event: %+v %+v", value, out.events)
	}
	e := out.events[0]
	if e.TaskID != value.ID || e.UserID != 7 || e.Title != "Learn Kafka" || e.Validate() != nil || out.ctx != ctx {
		t.Fatalf("wrong event: %+v", e)
	}
}
func TestPublishFailureDoesNotPretendSQLRollback(t *testing.T) {
	repo := memory.NewTaskRepository()
	out := &publisher{err: errors.New("broker down")}
	_, err := task.NewUseCaseWithEvents(repo, out).Create(context.Background(), 7, task.CreateInput{Title: "saved"})
	if !errors.Is(err, task.ErrEventPublish) || !errors.Is(err, out.err) {
		t.Fatal(err)
	}
	rows, _ := repo.List(context.Background(), 7)
	if len(rows) != 1 || rows[0].ProcessingStatus != task.Pending {
		t.Fatal("dual write behavior hidden")
	}
}
func TestRejectedCreateDoesNotPublish(t *testing.T) {
	repo := memory.NewTaskRepository()
	out := &publisher{}
	uc := task.NewUseCaseWithEvents(repo, out)
	if _, err := uc.Create(context.Background(), 7, task.CreateInput{Title: " "}); !errors.Is(err, task.ErrEmptyTitle) {
		t.Fatal(err)
	}
	if len(out.events) != 0 {
		t.Fatal("event before validation")
	}
}
func TestProcessedIsIdempotentAndKeepsDone(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewTaskRepository()
	v, _ := repo.Create(ctx, 7, task.CreateInput{Title: "two words"})
	done := true
	_, _ = repo.Update(ctx, v.ID, task.UpdateInput{Done: &done})
	e := task.Event{ID: "result", Type: task.TaskProcessed, Version: 1, OccurredAt: time.Now(), TaskID: v.ID, UserID: 7, Result: "success", WordCount: 2}
	h := task.NewProcessedHandler(repo)
	if err := h.Handle(ctx, e); err != nil {
		t.Fatal(err)
	}
	first, _ := repo.Get(ctx, v.ID)
	if err := h.Handle(ctx, e); err != nil {
		t.Fatal(err)
	}
	second, _ := repo.Get(ctx, v.ID)
	if !second.Done || second.ProcessingStatus != task.Processed || !first.UpdatedAt.Equal(second.UpdatedAt) {
		t.Fatalf("duplicate changed state: %+v", second)
	}
	e.UserID = 8
	if err := h.Handle(ctx, e); !errors.Is(err, task.ErrInvalidEvent) {
		t.Fatalf("owner bypass: %v", err)
	}
	_ = repo.Delete(ctx, v.ID)
	e.UserID = 7
	if err := h.Handle(ctx, e); err != nil {
		t.Fatalf("deleted task should be ignored: %v", err)
	}
}
