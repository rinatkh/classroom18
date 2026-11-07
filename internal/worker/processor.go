// Package worker содержит обработку TaskCreated без Kafka client и SQL.
package worker

import (
	"context"
	"strings"
	"time"

	"github.com/rinatkh/classroom18/internal/task"
)

type Processor struct{ publisher task.EventPublisher }

func NewProcessor(publisher task.EventPublisher) *Processor { return &Processor{publisher: publisher} }
func (p *Processor) Handle(ctx context.Context, event task.Event) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if event.Type != task.TaskCreated {
		return nil
	}
	// Минимальная обработка: анализ title по Unicode-пробелам, без изменения Done.
	words := len(strings.Fields(event.Title))
	if words == 0 {
		return task.ErrInvalidEvent
	}
	// Повторная доставка входа даёт тот же ID результата. API идемпотентен.
	result := task.Event{ID: event.ID + ":processed", Type: task.TaskProcessed, Version: 1,
		OccurredAt: time.Now().UTC(), TaskID: event.TaskID, UserID: event.UserID,
		Result: "success", WordCount: words}
	return p.publisher.Publish(ctx, result)
}
