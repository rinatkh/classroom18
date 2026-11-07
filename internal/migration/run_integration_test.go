//go:build integration

package migration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"os"
	"testing"
	"time"
)

func fixture(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)
	admin, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	var id [8]byte
	_, _ = rand.Read(id[:])
	schema := "migration_" + hex.EncodeToString(id[:])
	quoted := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+quoted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+quoted+" CASCADE")
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
	return pool, ctx
}
func TestIntegrationMigrationJournalAndRollback(t *testing.T) {
	pool, ctx := fixture(t)
	for i := 0; i < 2; i++ {
		if err := Run(ctx, pool, "up"); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM goose_db_version WHERE is_applied AND version_id>0").Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO users(email,password_hash) VALUES('alice@example.test','hash');INSERT INTO tasks(owner_id,title) VALUES(1,'persist')`); err != nil {
		t.Fatal(err)
	}
	if err := Run(ctx, pool, "down"); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM tasks").Scan(&count); err != nil || count != 1 {
		t.Fatal("business data lost", err)
	}
	if err := Run(ctx, pool, "up"); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := pool.QueryRow(ctx, "SELECT processing_status FROM tasks").Scan(&status); err != nil || status != "pending" {
		t.Fatal(status, err)
	}
}

// Финальное занятие 15 использует тот же goose history. Подключаем только 003.
func TestIntegrationBaselineFinal15(t *testing.T) {
	pool, ctx := fixture(t)
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("../../migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	if _, err = provider.UpTo(ctx, 2); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO users(email,password_hash) VALUES('alice@example.test','hash'); INSERT INTO tasks(owner_id,title) VALUES(1,'persist'); INSERT INTO refresh_sessions(user_id,token_hash,expires_at) VALUES(1,'old-hash',now()+interval '1 day')`); err != nil {
		t.Fatal(err)
	}
	if err = Run(ctx, pool, "up"); err != nil {
		t.Fatal(err)
	}
	var title string
	var sessions int
	if err = pool.QueryRow(ctx, "SELECT title FROM tasks WHERE id=1").Scan(&title); err != nil || title != "persist" {
		t.Fatal(title, err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM refresh_sessions").Scan(&sessions); err != nil || sessions != 1 {
		t.Fatal(sessions, err)
	}
	if err = Run(ctx, pool, "down"); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM refresh_sessions").Scan(&sessions); err != nil || sessions != 1 {
		t.Fatal("003 down lost session", err)
	}
	if err = Run(ctx, pool, "up"); err != nil {
		t.Fatal(err)
	}
}
