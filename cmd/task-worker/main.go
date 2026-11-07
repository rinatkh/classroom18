package main

import (
	"context"
	"errors"
	"github.com/rinatkh/classroom18/internal/logging"
	"github.com/rinatkh/classroom18/internal/messaging/kafka"
	"github.com/rinatkh/classroom18/internal/task"
	"github.com/rinatkh/classroom18/internal/worker"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

func main() {
	if err := logging.Configure("task-worker"); err != nil {
		logging.Fatal("invalid logging configuration", "error_type", logging.ErrorType(err))
	}
	if err := run(); err != nil {
		logging.Fatal("service stopped", "error_type", logging.ErrorType(err))
	}
}
func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	brokers := envOr("KAFKA_BROKERS", "localhost:19092")
	topic := envOr("KAFKA_TOPIC", "task.events")
	publisher := kafka.NewPublisher(strings.Split(brokers, ","), topic)
	defer publisher.Close()
	processor := worker.NewProcessor(publisher)
	consumer := kafka.NewConsumer(strings.Split(brokers, ","), topic, "task-workers", task.TaskCreated, processor, log.Default())
	defer consumer.Close()
	log.Printf("task-worker consumes %s and produces %s, topic=%s", task.TaskCreated, task.TaskProcessed, topic)
	err := consumer.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}
func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
