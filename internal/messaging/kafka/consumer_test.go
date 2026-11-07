package kafka

import (
	"context"
	"errors"
	"github.com/rinatkh/classroom18/internal/task"
	kafkago "github.com/segmentio/kafka-go"
	"io"
	"log"
	"testing"
	"time"
)

type readerStub struct {
	message          kafkago.Message
	fetched, commits int
	commitFailures   int
	cancel           context.CancelFunc
}

func (r *readerStub) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	r.fetched++
	if r.fetched == 1 {
		return r.message, nil
	}
	<-ctx.Done()
	return kafkago.Message{}, ctx.Err()
}
func (r *readerStub) CommitMessages(ctx context.Context, m ...kafkago.Message) error {
	r.commits++
	if r.commits <= r.commitFailures {
		return errors.New("temporary commit failure")
	}
	r.cancel()
	return nil
}
func (r *readerStub) Close() error { return nil }

type handlerFunc func(context.Context, task.Event) error

func (f handlerFunc) Handle(ctx context.Context, e task.Event) error { return f(ctx, e) }

func TestConsumerHandlesBeforeCommitAndRetriesSameRecord(t *testing.T) {
	for _, tc := range []struct {
		name                            string
		handlerFailures, commitFailures int
	}{
		{"успех", 0, 0}, {"повтор обработки", 1, 0}, {"повтор commit", 0, 1}, {"обе временные ошибки", 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			e, _ := task.NewCreatedEvent(task.Task{ID: 3, OwnerID: 7, Title: "title"})
			data, _ := Encode(e)
			reader := &readerStub{message: kafkago.Message{Value: data, Offset: 4, Partition: 1}, commitFailures: tc.commitFailures, cancel: cancel}
			calls := 0
			handler := handlerFunc(func(ctx context.Context, event task.Event) error {
				calls++
				if reader.commits != 0 {
					t.Fatal("offset committed before handler")
				}
				if calls <= tc.handlerFailures {
					return errors.New("retry")
				}
				if event.ID != e.ID {
					t.Fatal("record changed")
				}
				return nil
			})
			c := &Consumer{reader: reader, handler: handler, eventType: task.TaskCreated, logger: log.New(io.Discard, "", 0)}
			if err := c.Run(ctx); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if calls != tc.handlerFailures+1 || reader.commits != tc.commitFailures+1 || reader.fetched != 1 {
				t.Fatalf("calls=%d commits=%d fetched=%d", calls, reader.commits, reader.fetched)
			}
		})
	}
}
func TestConsumerPoisonAndTypeFiltering(t *testing.T) {
	e, _ := task.NewCreatedEvent(task.Task{ID: 1, OwnerID: 2, Title: "x"})
	data, _ := Encode(e)
	for _, tc := range []struct {
		name               string
		data               []byte
		wantType           string
		invalidApplication bool
	}{
		{"невалидный JSON", []byte("{"), task.TaskCreated, false},
		{"другой event type", data, task.TaskProcessed, false},
		{"невосстановимое application событие", data, task.TaskCreated, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			r := &readerStub{message: kafkago.Message{Value: tc.data}, cancel: cancel}
			calls := 0
			c := &Consumer{reader: r, eventType: tc.wantType, logger: log.New(io.Discard, "", 0), handler: handlerFunc(func(context.Context, task.Event) error { calls++; return task.ErrInvalidEvent })}
			_ = c.Run(ctx)
			if r.commits != 1 {
				t.Fatal("poison/filter offset must follow declared skip policy")
			}
			if tc.invalidApplication && calls != 1 || !tc.invalidApplication && calls != 0 {
				t.Fatal(calls)
			}
		})
	}
}
func TestConsumerCancellationDoesNotCommitFailedHandler(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	e, _ := task.NewCreatedEvent(task.Task{ID: 1, OwnerID: 2, Title: "x"})
	data, _ := Encode(e)
	r := &readerStub{message: kafkago.Message{Value: data}, cancel: cancel}
	c := &Consumer{reader: r, eventType: task.TaskCreated, logger: log.New(io.Discard, "", 0), handler: handlerFunc(func(context.Context, task.Event) error { cancel(); return context.Canceled })}
	if err := c.Run(ctx); !errors.Is(err, context.Canceled) || r.commits != 0 {
		t.Fatal(err, r.commits)
	}
}
