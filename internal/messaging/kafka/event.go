// Package kafka переводит application events в JSON records и обратно.
package kafka

import (
	"encoding/json"
	"fmt"
	"github.com/rinatkh/classroom18/internal/task"
	"time"
)

type envelope struct {
	ID         string    `json:"event_id"`
	Type       string    `json:"event_type"`
	Version    int       `json:"event_version"`
	OccurredAt time.Time `json:"occurred_at"`
	TaskID     int64     `json:"task_id"`
	UserID     int64     `json:"user_id"`
	Title      string    `json:"title,omitempty"`
	Result     string    `json:"result,omitempty"`
	WordCount  int       `json:"word_count,omitempty"`
}

func Encode(e task.Event) ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(envelope{e.ID, e.Type, e.Version, e.OccurredAt, e.TaskID, e.UserID, e.Title, e.Result, e.WordCount})
}
func Decode(data []byte) (task.Event, error) {
	var v envelope
	if err := json.Unmarshal(data, &v); err != nil {
		return task.Event{}, fmt.Errorf("%w: JSON", task.ErrInvalidEvent)
	}
	e := task.Event{ID: v.ID, Type: v.Type, Version: v.Version, OccurredAt: v.OccurredAt, TaskID: v.TaskID, UserID: v.UserID, Title: v.Title, Result: v.Result, WordCount: v.WordCount}
	return e, e.Validate()
}
