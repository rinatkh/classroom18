//go:build integration

package kafka

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/rinatkh/classroom18/internal/task"
	"github.com/rinatkh/classroom18/internal/worker"
	kafka "github.com/segmentio/kafka-go"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

type observeHandler struct {
	next     EventHandler
	received chan task.Event
}

func (h observeHandler) Handle(ctx context.Context, e task.Event) error {
	if h.next != nil {
		if err := h.next.Handle(ctx, e); err != nil {
			return err
		}
	}
	select {
	case h.received <- e:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
func TestIntegrationKafkaWorkerRoundTrip(t *testing.T) {
	brokers := strings.Split(os.Getenv("TEST_KAFKA_BROKERS"), ",")
	if brokers[0] == "" {
		t.Fatal("TEST_KAFKA_BROKERS required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	var suffix [8]byte
	_, _ = rand.Read(suffix[:])
	topic := "task.test." + hex.EncodeToString(suffix[:])
	conn, err := kafka.DialContext(ctx, "tcp", brokers[0])
	if err != nil {
		t.Fatal(err)
	}
	controller, err := conn.Controller()
	_ = conn.Close()
	if err != nil {
		t.Fatal(err)
	}
	admin, err := kafka.DialContext(ctx, "tcp", controller.Host+":"+itoa(controller.Port))
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 3, ReplicationFactor: 1}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = admin.SetDeadline(time.Now().Add(5 * time.Second))
		if err := admin.DeleteTopics(topic); err != nil {
			t.Errorf("cleanup topic: %v", err)
		}
		_ = admin.Close()
	})
	publisher := NewPublisher(brokers, topic)
	defer publisher.Close()
	created := make(chan task.Event, 2)
	processed := make(chan task.Event, 2)
	logger := log.New(io.Discard, "", 0)
	workerConsumer := NewConsumer(brokers, topic, topic+"-workers", task.TaskCreated, observeHandler{worker.NewProcessor(publisher), created}, logger)
	resultConsumer := NewConsumer(brokers, topic, topic+"-api", task.TaskProcessed, observeHandler{nil, processed}, logger)
	done := make(chan error, 2)
	go func() { done <- workerConsumer.Run(ctx) }()
	go func() { done <- resultConsumer.Run(ctx) }()
	defer func() { cancel(); _ = workerConsumer.Close(); _ = resultConsumer.Close(); <-done; <-done }()
	event, err := task.NewCreatedEvent(task.Task{ID: 42, OwnerID: 7, Title: "Go Kafka lesson"})
	if err != nil {
		t.Fatal(err)
	}
	if err := publisher.Publish(ctx, event); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-created:
		if e.ID != event.ID {
			t.Fatal(e)
		}
	case <-ctx.Done():
		t.Fatal("created not consumed")
	}
	select {
	case e := <-processed:
		if e.TaskID != 42 || e.WordCount != 3 || e.Result != "success" {
			t.Fatal(e)
		}
	case <-ctx.Done():
		t.Fatal("processed not consumed")
	}
}
func itoa(n int) string { return strconv.Itoa(n) }
