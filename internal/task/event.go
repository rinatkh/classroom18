package task

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

const (
	TaskCreated   = "task.created"
	TaskProcessed = "task.processed"
	Pending       = "pending"
	Processed     = "processed"
)

// Event — application contract. Kafka JSON и protobuf остаются в adapters.
type Event struct {
	ID         string
	Type       string
	Version    int
	OccurredAt time.Time
	TaskID     int64
	UserID     int64
	Title      string
	Result     string
	WordCount  int
}

type EventPublisher interface {
	Publish(context.Context, Event) error
}

// Старые CRUD-потребители не обязаны знать об async processing.
type ProcessingRepository interface {
	Repository
	MarkProcessed(context.Context, int64) error
}

var ErrInvalidEvent = errors.New("invalid task event")
var ErrEventPublish = errors.New("task saved but event publication failed")

func NewCreatedEvent(value Task) (Event, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return Event{}, err
	}
	return Event{ID: hex.EncodeToString(id[:]), Type: TaskCreated, Version: 1,
		OccurredAt: time.Now().UTC(), TaskID: value.ID, UserID: value.OwnerID, Title: value.Title}, nil
}

func (e Event) Validate() error {
	if e.ID == "" || e.Version != 1 || e.OccurredAt.IsZero() || e.TaskID <= 0 || e.UserID <= 0 {
		return ErrInvalidEvent
	}
	switch e.Type {
	case TaskCreated:
		if e.Title == "" {
			return ErrInvalidEvent
		}
	case TaskProcessed:
		if e.Result != "success" || e.WordCount <= 0 {
			return ErrInvalidEvent
		}
	default:
		return ErrInvalidEvent
	}
	return nil
}

type noopPublisher struct{}

func (noopPublisher) Publish(context.Context, Event) error { return nil }
