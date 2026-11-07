package kafka

import (
	"context"
	"github.com/rinatkh/classroom18/internal/task"
	kafka "github.com/segmentio/kafka-go"
	"log"
	"strconv"
	"time"
)

type Publisher struct{ writer *kafka.Writer }

var _ task.EventPublisher = (*Publisher)(nil)

func NewPublisher(brokers []string, topic string) *Publisher {
	return &Publisher{writer: &kafka.Writer{Addr: kafka.TCP(brokers...), Topic: topic,
		Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll, Async: false,
		BatchTimeout: 10 * time.Millisecond, ReadTimeout: 3 * time.Second, WriteTimeout: 3 * time.Second,
		MaxAttempts: 3}}
}
func (p *Publisher) Publish(ctx context.Context, event task.Event) error {
	data, err := Encode(event)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err = p.writer.WriteMessages(ctx, kafka.Message{Key: []byte(strconv.FormatInt(event.TaskID, 10)), Value: data})
	if err != nil {
		log.Printf("Kafka publish failed type=%s task_id=%d: %v", event.Type, event.TaskID, err)
	}
	return err
}
func (p *Publisher) Close() error { return p.writer.Close() }
