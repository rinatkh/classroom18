package kafka

import (
	"context"
	"errors"
	"github.com/rinatkh/classroom18/internal/task"
	kafka "github.com/segmentio/kafka-go"
	"log"
	"time"
)

type EventHandler interface {
	Handle(context.Context, task.Event) error
}
type messageReader interface {
	FetchMessage(context.Context) (kafka.Message, error)
	CommitMessages(context.Context, ...kafka.Message) error
	Close() error
}

type Consumer struct {
	reader    messageReader
	handler   EventHandler
	eventType string
	logger    *log.Logger
}

func NewConsumer(brokers []string, topic, group, eventType string, handler EventHandler, logger *log.Logger) *Consumer {
	return &Consumer{reader: kafka.NewReader(kafka.ReaderConfig{Brokers: brokers, Topic: topic,
		GroupID: group, StartOffset: kafka.FirstOffset, CommitInterval: 0,
		MaxBytes: 1 << 20, MaxWait: 500 * time.Millisecond}), handler: handler, eventType: eventType, logger: logger}
}

// Fetch + Handle + Commit: offset двигается только после успешного результата.
// Один поток намеренно упрощает первый урок. Ошибка одной partition задерживает reader.
func (c *Consumer) Run(ctx context.Context) error {
	for ctx.Err() == nil {
		message, err := c.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Printf("Kafka fetch: %v", err)
			if err := pause(ctx); err != nil {
				return err
			}
			continue
		}
		event, decodeErr := Decode(message.Value)
		if decodeErr != nil {
			c.logger.Printf("skip invalid event partition=%d offset=%d", message.Partition, message.Offset)
		}
		for decodeErr == nil && event.Type == c.eventType {
			err = c.handler.Handle(ctx, event)
			if err == nil {
				c.logger.Printf("handled %s task_id=%d event_id=%s", event.Type, event.TaskID, event.ID)
				break
			}
			if errors.Is(err, task.ErrInvalidEvent) {
				c.logger.Printf("skip invalid application event task_id=%d", event.TaskID)
				break
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Printf("retry %s task_id=%d: %v", event.Type, event.TaskID, err)
			if err := pause(ctx); err != nil {
				return err
			}
		}
		// Не читаем следующий record до commit: иначе можно пропустить незафиксированный offset.
		for {
			err = c.reader.CommitMessages(ctx, message)
			if err == nil {
				break
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			c.logger.Printf("Kafka commit retry: %v", err)
			if err := pause(ctx); err != nil {
				return err
			}
		}
	}
	return ctx.Err()
}
func (c *Consumer) Close() error { return c.reader.Close() }
func pause(ctx context.Context) error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
