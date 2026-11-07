package kafka

import (
	"errors"
	"github.com/rinatkh/classroom18/internal/task"
	"reflect"
	"testing"
	"time"
)

func TestJSONEventBoundary(t *testing.T) {
	event := task.Event{ID: "event", Type: task.TaskCreated, Version: 1, OccurredAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), TaskID: 42, UserID: 7, Title: "Learn"}
	data, err := Encode(event)
	if err != nil {
		t.Fatal(err)
	}
	restored, err := Decode(data)
	if err != nil || !reflect.DeepEqual(restored, event) {
		t.Fatalf("%+v %v", restored, err)
	}
	for _, data := range []string{`{`, `{}`, `{"event_type":"other","event_version":1}`} {
		if _, err := Decode([]byte(data)); !errors.Is(err, task.ErrInvalidEvent) {
			t.Fatalf("%s: %v", data, err)
		}
	}
}
