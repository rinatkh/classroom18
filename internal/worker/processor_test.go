package worker

import (
	"context"
	"errors"
	"github.com/rinatkh/classroom18/internal/task"
	"testing"
	"time"
)

type publisher struct {
	event task.Event
	calls int
	err   error
}

func (p *publisher) Publish(_ context.Context, e task.Event) error {
	p.event = e
	p.calls++
	return p.err
}
func TestProcessorAnalyzesTitleAndProducesResult(t *testing.T) {
	out := &publisher{}
	p := NewProcessor(out)
	e := task.Event{ID: "input", Type: task.TaskCreated, Version: 1, OccurredAt: time.Now(), TaskID: 42, UserID: 7, Title: " Go\tи\nKafka "}
	if err := p.Handle(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	if out.event.WordCount != 3 || out.event.Type != task.TaskProcessed || out.event.TaskID != 42 || out.event.UserID != 7 || out.event.ID != "input:processed" {
		t.Fatalf("processing missing: %+v", out.event)
	}
	first := out.event
	if err := p.Handle(context.Background(), e); err != nil || out.event.ID != first.ID || out.event.TaskID != first.TaskID || out.event.WordCount != first.WordCount {
		t.Fatal("deterministic result required")
	}
	out.err = errors.New("publish failed")
	if err := p.Handle(context.Background(), e); !errors.Is(err, out.err) {
		t.Fatal("must retry when output not published")
	}
}
func TestProcessorSkipsOtherTypeAndRejectsBadInput(t *testing.T) {
	out := &publisher{}
	p := NewProcessor(out)
	if err := p.Handle(context.Background(), task.Event{}); !errors.Is(err, task.ErrInvalidEvent) {
		t.Fatal(err)
	}
	e := task.Event{ID: "id", Type: task.TaskProcessed, Version: 1, OccurredAt: time.Now(), TaskID: 1, UserID: 1, Result: "success", WordCount: 1}
	if err := p.Handle(context.Background(), e); err != nil || out.calls != 0 {
		t.Fatal("event loop")
	}
}
