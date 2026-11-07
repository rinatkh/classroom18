// Команда migrate запускает goose отдельно от HTTP-сервера.
// Запускаем из корня classroom18: go run ./cmd/migrate up|down|status.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/rinatkh/classroom18/internal/logging"
)

func main() {
	if err := logging.Configure("task-migrations"); err != nil {
		logging.Fatal("invalid logging configuration", "error", err)
	}
	if err := run(); err != nil {
		logging.Fatal("migration command failed", "error", err)
	}
}

func run() error {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "down" && os.Args[1] != "status") {
		return errors.New("используйте go run ./cmd/migrate up|down|status из корня classroom18")
	}
	command := os.Args[1]
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("задайте DATABASE_URL; см. docs/setup.md")
	}
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return errors.New("не удалось прочитать DATABASE_URL")
	}
	db := stdlib.OpenDB(*cfg)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return errors.New("не удалось загрузить goose migrations; проверьте каталог migrations")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	slog.InfoContext(ctx, "migration command started", "command", command)
	switch command {
	case "up":
		if _, err := provider.Up(ctx); err != nil {
			logging.Error(ctx, "goose up failed", err)
			return errors.New("goose up не выполнен; проверьте подключение, SQL и версию схемы, см. docs/setup.md")
		}
	case "down":
		if _, err := provider.Down(ctx); err != nil {
			logging.Error(ctx, "goose down failed", err)
			return errors.New("goose down не выполнен; проверьте подключение и текущую версию схемы")
		}
	case "status":
		statuses, err := provider.Status(ctx)
		if err != nil {
			return errors.New("не удалось получить статус goose; проверьте подключение")
		}
		for _, status := range statuses {
			slog.InfoContext(ctx, "migration status", "version", status.Source.Version, "state", status.State)
		}
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return errors.New("не удалось прочитать текущую версию goose")
	}
	slog.InfoContext(ctx, "migration command completed", "command", command, "version", version)
	return nil
}
