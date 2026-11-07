//go:build integration

package postgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

// Каждый тест получает свою схему: не TRUNCATE и не DROP чужих users/tasks.
// Отсутствующий DSN — ошибка explicit integration run, а не скрытый Skip.
func integrationPool(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("задайте TEST_DATABASE_URL для -tags=integration; см. docs/setup.md")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	schema := "classroom18_test_" + hex.EncodeToString(suffix[:])
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanupCtx, "DROP SCHEMA "+quoted+" CASCADE"); err != nil {
			t.Errorf("cleanup schema: %v", err)
		}
	})
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	applyMigration(t, ctx, pool, "up")
	return pool, ctx
}

func applyMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, direction string) {
	t.Helper()
	// Используем тот же goose, что и cmd/migrate, без ручного разбора Up/Down.
	// Отдельный Provider и search_path из pool изолируют историю версий каждого теста.
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("../../../migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	switch direction {
	case "up":
		_, err = provider.Up(ctx)
	case "down":
		_, err = provider.Down(ctx)
	default:
		t.Fatalf("unknown migration direction %q", direction)
	}
	if err != nil {
		t.Fatalf("migration %s: %v", direction, err)
	}
}
