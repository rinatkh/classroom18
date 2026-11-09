package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rinatkh/classroom18/internal/automation"
	"github.com/rinatkh/classroom18/internal/logging"
	taskv1 "github.com/rinatkh/classroom18/pkg/proto/task/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
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
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(automation.LogRPC))
	if err != nil {
		return err
	}
	defer conn.Close()
	// NewClient подключается лениво: это configuration, а не успешный handshake.
	slog.Debug("automation gRPC client configured", "grpc", target)
	addr := os.Getenv("AUTOMATION_HTTP_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8090"
	}
	server := &http.Server{Addr: addr, Handler: automation.NewRouter(taskv1.NewTaskServiceClient(conn)), ReadHeaderTimeout: 5 * time.Second}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		logging.Error(ctx, "automation HTTP listener failed", err, "http", addr)
		return err
	}
	defer listener.Close()
	failed := make(chan error, 1)
	go func() { failed <- server.Serve(listener) }()
	slog.Info("automation started", "http", listener.Addr().String(), "grpc", target)
	select {
	case <-ctx.Done():
		slog.Info("automation stopping", "reason", "signal")
	case err = <-failed:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logging.Error(ctx, "automation HTTP server failed", err)
		}
	}
	stop, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	if shutdownErr := server.Shutdown(stop); shutdownErr != nil {
		logging.Error(stop, "automation shutdown failed", shutdownErr)
		_ = server.Close()
		return shutdownErr
	}
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	slog.Info("automation stopped")
	return nil
}
