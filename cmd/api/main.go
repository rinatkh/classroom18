package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net"
	"os/signal"
	"strings"
	"syscall"

	goredis "github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/rinatkh/classroom18/internal/auth"
	"github.com/rinatkh/classroom18/internal/grpcapi"
	"github.com/rinatkh/classroom18/internal/httpapi"
	"github.com/rinatkh/classroom18/internal/httpapi/handler"
	"github.com/rinatkh/classroom18/internal/httpapi/middleware"
	"github.com/rinatkh/classroom18/internal/logging"
	"github.com/rinatkh/classroom18/internal/messaging/kafka"
	"github.com/rinatkh/classroom18/internal/repository/postgres"
	redisrepo "github.com/rinatkh/classroom18/internal/repository/redis"
	"github.com/rinatkh/classroom18/internal/task"
	"github.com/rinatkh/classroom18/internal/user"
	taskv1 "github.com/rinatkh/classroom18/pkg/proto/task/v1"
)

func main() {
	if err := logging.Configure("task-api"); err != nil {
		logging.Fatal("invalid logging configuration", "error_type", logging.ErrorType(err))
	}
	if err := run(); err != nil {
		logging.Fatal("API stopped", "error_type", logging.ErrorType(err))
	}
}
func run() error {
	appCtx, stopApp := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopApp()
	secret := os.Getenv("JWT_SECRET")
	if len(secret) < 32 {
		return errors.New("JWT_SECRET должен содержать не менее 32 байт; см. README")
	}
	accessTTL, err := duration("ACCESS_TOKEN_TTL", "15m")
	if err != nil {
		return err
	}
	refreshTTL, err := duration("REFRESH_TOKEN_TTL", "168h")
	if err != nil {
		return err
	}
	if refreshTTL <= accessTTL {
		return errors.New("REFRESH_TOKEN_TTL должен быть больше ACCESS_TOKEN_TTL")
	}
	secure, err := strconv.ParseBool(envOr("COOKIE_SECURE", "true"))
	if err != nil {
		return errors.New("COOKIE_SECURE должен быть true или false")
	}
	cookie := handler.CookieConfig{Name: envOr("REFRESH_COOKIE_NAME", "refresh_token"), Secure: secure}
	if err := cookie.Validate(); err != nil {
		return errors.New("неверная настройка refresh cookie; см. README")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL обязателен; см. docs/setup.md")
	}
	ctx, cancel := context.WithTimeout(appCtx, 5*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return errors.New("не удалось создать pool PostgreSQL; проверьте DATABASE_URL")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return errors.New("PostgreSQL недоступен; проверьте подключение и docs/setup.md")
	}

	// Users и Tasks остаются в PostgreSQL; меняется только session adapter.
	userRepository := postgres.NewUserRepository(pool)
	taskRepository := postgres.NewTaskRepository(pool)
	redisOptions, err := goredis.ParseURL(envOr("REDIS_URL", "redis://localhost:6379/0"))
	if err != nil {
		return errors.New("invalid REDIS_URL")
	}
	redisOptions.ContextTimeoutEnabled = true
	redisOptions.DialTimeout = time.Second
	redisOptions.ReadTimeout = time.Second
	redisOptions.WriteTimeout = time.Second
	redisClient := goredis.NewClient(redisOptions)
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return errors.New("Redis unavailable")
	}
	sessionRepository := redisrepo.NewRefreshSessionRepository(redisClient)
	brokers := strings.Split(envOr("KAFKA_BROKERS", "localhost:19092"), ",")
	publisher := kafka.NewPublisher(brokers, envOr("KAFKA_TOPIC", "task.events"))
	defer publisher.Close()
	passwordHasher := auth.NewBcryptHasher(0)
	tokenManager := auth.NewJWTManager(secret, accessTTL)
	authUseCase := user.NewUseCase(userRepository, passwordHasher, tokenManager,
		sessionRepository, auth.NewCryptoRefreshTokenGenerator(),
		user.SessionConfig{AccessTTL: accessTTL, RefreshTTL: refreshTTL})
	taskUseCase := task.NewUseCaseWithEvents(taskRepository, publisher)
	results := task.NewProcessedHandler(taskRepository)
	consumer := kafka.NewConsumer(brokers, envOr("KAFKA_TOPIC", "task.events"), "api-task-results", task.TaskProcessed, results, log.Default())
	defer consumer.Close()
	authHandler := handler.NewAuthHandler(authUseCase, cookie)
	taskHandler := handler.NewTaskHandler(taskUseCase)
	authMiddleware := middleware.NewAuth(tokenManager)
	router := httpapi.NewRouter(authHandler, taskHandler, authMiddleware)
	server := &http.Server{Addr: envOr("HTTP_ADDR", ":8080"), Handler: router, ReadHeaderTimeout: 5 * time.Second}
	listener, err := net.Listen("tcp", envOr("GRPC_ADDR", "127.0.0.1:50051"))
	if err != nil {
		return err
	}
	defer listener.Close()
	grpcServer := grpc.NewServer(grpc.MaxRecvMsgSize(64 << 10))
	taskv1.RegisterTaskServiceServer(grpcServer, grpcapi.NewTaskServer(taskUseCase))
	// grpcurl в локальном demo. Не публикуем этот trusted gRPC port в интернет.
	reflection.Register(grpcServer)
	failed := make(chan error, 3)
	go func() { failed <- server.ListenAndServe() }()
	go func() { failed <- grpcServer.Serve(listener) }()
	consumerDone := make(chan struct{})
	go func() { defer close(consumerDone); failed <- consumer.Run(appCtx) }()
	slog.Info("Task API started", "http", server.Addr, "grpc", listener.Addr().String())
	select {
	case <-appCtx.Done():
	case err = <-failed:
	}
	stopApp()
	shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
	defer done()
	if stopErr := server.Shutdown(shutdown); stopErr != nil {
		_ = server.Close()
	}
	grpcDone := make(chan struct{})
	go func() { grpcServer.GracefulStop(); close(grpcDone) }()
	select {
	case <-grpcDone:
	case <-shutdown.Done():
		grpcServer.Stop()
	}
	_ = consumer.Close()
	<-consumerDone
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

func envOr(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func duration(name, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(envOr(name, fallback))
	if err != nil || value < time.Second || value%time.Second != 0 {
		return 0, errors.New("invalid token lifetime: " + name)
	}
	return value, nil
}
