package main

import (
	"context"
	"errors"
	"github.com/rinatkh/classroom18/internal/automation"
	"github.com/rinatkh/classroom18/internal/logging"
	taskv1 "github.com/rinatkh/classroom18/pkg/proto/task/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := logging.Configure("automation-service"); err != nil {
		logging.Fatal("invalid logging configuration", "error_type", logging.ErrorType(err))
	}
	if err := run(); err != nil {
		logging.Fatal("service stopped", "error_type", logging.ErrorType(err))
	}
}
func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	target := os.Getenv("API_GRPC_ADDR")
	if target == "" {
		target = "localhost:50051"
	}
	// Plaintext + user_id — только loopback/Compose demo. Production: identity/mTLS.
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return err
	}
	defer conn.Close()
	addr := os.Getenv("AUTOMATION_HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8090"
	}
	server := &http.Server{Addr: addr, Handler: automation.NewRouter(taskv1.NewTaskServiceClient(conn)), ReadHeaderTimeout: 5 * time.Second}
	failed := make(chan error, 1)
	go func() { failed <- server.ListenAndServe() }()
	slog.Info("automation started", "http", addr, "grpc", target)
	select {
	case <-ctx.Done():
	case err = <-failed:
	}
	stop, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	_ = server.Shutdown(stop)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
