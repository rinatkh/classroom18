//go:build integration

package postgres

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rinatkh/classroom18/internal/task"
	"github.com/rinatkh/classroom18/internal/user"
)

func TestIntegrationUserRepository(t *testing.T) {
	pool, ctx := integrationPool(t)
	repo := NewUserRepository(pool)
	alice, err := repo.Create(ctx, user.User{Email: "alice@example.test", PasswordHash: "opaque-hash"})
	if err != nil || alice.ID <= 0 || alice.PasswordHash != "opaque-hash" {
		t.Fatalf("Create=%+v, %v", alice, err)
	}
	found, err := repo.GetByEmail(ctx, "ALICE@example.test")
	if err != nil || found != alice {
		t.Fatalf("GetByEmail=%+v, %v", found, err)
	}
	_, err = repo.Create(ctx, user.User{Email: "alice@example.test", PasswordHash: "other"})
	if !errors.Is(err, user.ErrAlreadyExists) || err.Error() != user.ErrAlreadyExists.Error() {
		t.Fatalf("duplicate exposed database error: %v", err)
	}
	if _, err := repo.GetByEmail(ctx, "missing@example.test"); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("missing=%v", err)
	}
}

func TestIntegrationConcurrentDuplicateEmail(t *testing.T) {
	pool, ctx := integrationPool(t)
	repo := NewUserRepository(pool)
	var created atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := repo.Create(ctx, user.User{Email: "same@example.test", PasswordHash: "hash"})
			if err == nil {
				created.Add(1)
			} else if !errors.Is(err, user.ErrAlreadyExists) {
				t.Errorf("Create=%v", err)
			}
		}()
	}
	wg.Wait()
	if created.Load() != 1 {
		t.Fatalf("created=%d", created.Load())
	}
}

func TestIntegrationTaskRepository(t *testing.T) {
	pool, ctx := integrationPool(t)
	users := NewUserRepository(pool)
	repo := NewTaskRepository(pool)
	alice, err := users.Create(ctx, user.User{Email: "alice@example.test", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	bob, err := users.Create(ctx, user.User{Email: "bob@example.test", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	empty, err := repo.List(ctx, alice.ID)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty=%#v, %v", empty, err)
	}
	// Title остаётся данными, даже если похож на SQL; параметр не становится командой.
	title := "read'; DROP TABLE users; --"
	first, err := repo.Create(ctx, alice.ID, task.CreateInput{Title: title})
	if err != nil || first.OwnerID != alice.ID || first.Title != title || first.Done || first.CreatedAt.IsZero() {
		t.Fatalf("Create=%+v, %v", first, err)
	}
	second, err := repo.Create(ctx, alice.ID, task.CreateInput{Title: "second"})
	if err != nil {
		t.Fatal(err)
	}
	third, err := repo.Create(ctx, bob.ID, task.CreateInput{Title: "Bob"})
	if err != nil {
		t.Fatal(err)
	}
	list, err := repo.List(ctx, alice.ID)
	if err != nil || len(list) != 2 || list[0].ID != first.ID || list[1].ID != second.ID {
		t.Fatalf("List=%+v, %v", list, err)
	}
	own, err := repo.Get(ctx, first.ID)
	if err != nil || own != first {
		t.Fatalf("Get=%+v, %v", own, err)
	}
	done := true
	updated, err := repo.Update(ctx, first.ID, task.UpdateInput{Done: &done})
	if err != nil || !updated.Done || updated.Title != title || updated.OwnerID != alice.ID || !updated.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("Update=%+v, %v", updated, err)
	}
	active, err := repo.CountActive(ctx, alice.ID)
	if err != nil || active != 1 {
		t.Fatalf("CountActive Alice=%d, %v", active, err)
	}
	active, err = repo.CountActive(ctx, bob.ID)
	if err != nil || active != 1 {
		t.Fatalf("CountActive Bob=%d, %v", active, err)
	}
	done = false
	updated, err = repo.Update(ctx, first.ID, task.UpdateInput{Done: &done})
	if err != nil || updated.Done || updated.Title != title {
		t.Fatalf("explicit false=%+v, %v", updated, err)
	}
	if err := repo.Delete(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Get(ctx, first.ID); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("Get missing=%v", err)
	}
	if _, err := repo.Update(ctx, first.ID, task.UpdateInput{Done: &done}); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("Update missing=%v", err)
	}
	if err := repo.Delete(ctx, first.ID); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("Delete missing=%v", err)
	}
	if _, err := repo.Get(ctx, third.ID); err != nil {
		t.Fatalf("Bob row changed: %v", err)
	}
}

func TestIntegrationConstraintsAndCascade(t *testing.T) {
	pool, ctx := integrationPool(t)
	users := NewUserRepository(pool)
	tasks := NewTaskRepository(pool)
	alice, err := users.Create(ctx, user.User{Email: "alice@example.test", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	value, err := tasks.Create(ctx, alice.ID, task.CreateInput{Title: "owned"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tasks.Create(ctx, 999999, task.CreateInput{Title: "orphan"})
	assertSQLState(t, err, "23503")
	_, err = tasks.Create(ctx, alice.ID, task.CreateInput{Title: "   "})
	assertSQLState(t, err, "23514")
	_, err = pool.Exec(ctx, `INSERT INTO users(email,password_hash) VALUES(NULL,'hash')`)
	assertSQLState(t, err, "23502")
	_, err = pool.Exec(ctx, `INSERT INTO tasks(owner_id,title,done) VALUES($1,'null done',NULL)`, alice.ID)
	assertSQLState(t, err, "23502")
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, alice.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := tasks.Get(ctx, value.ID); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("CASCADE left Task: %v", err)
	}
}

func TestIntegrationRollback(t *testing.T) {
	pool, ctx := integrationPool(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	u, err := NewUserRepository(tx).Create(ctx, user.User{Email: "rollback@example.test", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	taskValue, err := NewTaskRepository(tx).Create(ctx, u.ID, task.CreateInput{Title: "rollback"})
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := NewUserRepository(pool).GetByEmail(ctx, u.Email); !errors.Is(err, user.ErrNotFound) {
		t.Fatalf("user survived rollback: %v", err)
	}
	if _, err := NewTaskRepository(pool).Get(ctx, taskValue.ID); !errors.Is(err, task.ErrNotFound) {
		t.Fatalf("task survived rollback: %v", err)
	}
}

func TestIntegrationContextCancellation(t *testing.T) {
	pool, parent := integrationPool(t)
	ctx, cancel := context.WithCancel(parent)
	cancel()
	users := NewUserRepository(pool)
	tasks := NewTaskRepository(pool)
	_, uCreate := users.Create(ctx, user.User{Email: "cancel@example.test", PasswordHash: "hash"})
	_, uGet := users.GetByEmail(ctx, "cancel@example.test")
	_, create := tasks.Create(ctx, 1, task.CreateInput{Title: "cancel"})
	_, get := tasks.Get(ctx, 1)
	_, list := tasks.List(ctx, 1)
	_, update := tasks.Update(ctx, 1, task.UpdateInput{})
	del := tasks.Delete(ctx, 1)
	_, count := tasks.CountActive(ctx, 1)
	for _, err := range []error{uCreate, uGet, create, get, list, update, del, count} {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("context replaced or lost: %v", err)
		}
	}
}

func TestIntegrationMigrationUpDownUp(t *testing.T) {
	pool, ctx := integrationPool(t)
	applyMigration(t, ctx, pool, "down")
	// Первый Down удаляет только новый processing_status, сохраняя SQL sessions.
	var sessions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name='refresh_sessions'`).Scan(&sessions); err != nil || sessions != 1 {
		t.Fatal("003 down removed sessions", err)
	}
	applyMigration(t, ctx, pool, "down")

	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name IN ('users','tasks')`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("003/002 down must preserve users/tasks: got %d tables", count)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name='refresh_sessions'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("down must remove only refresh_sessions: count=%d err=%v", count, err)
	}
	// Следующий отдельный Down откатывает базовую 001, как в CLI.
	applyMigration(t, ctx, pool, "down")
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema=current_schema() AND table_name IN ('users','tasks','refresh_sessions')`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("third down must remove base tables: count=%d err=%v", count, err)
	}
	applyMigration(t, ctx, pool, "up")
	if _, err := NewUserRepository(pool).Create(ctx, user.User{Email: "again@example.test", PasswordHash: "hash"}); err != nil {
		t.Fatalf("up after down=%v", err)
	}
}

func assertSQLState(t *testing.T, err error, want string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != want {
		t.Fatalf("SQLSTATE=%v, want %s", err, want)
	}
}

func TestIntegrationMigrationUpDoesNotRepeatAppliedDDL(t *testing.T) {
	pool, ctx := integrationPool(t)
	repo := NewUserRepository(pool)
	alice, err := repo.Create(ctx, user.User{Email: "kept@example.test", PasswordHash: "hash"})
	if err != nil {
		t.Fatal(err)
	}
	// Прямое повторное CREATE TABLE завершилось бы ошибкой. Goose читает version
	// и пропускает уже применённый Up, сохраняя существующие данные.
	applyMigration(t, ctx, pool, "up")
	found, err := repo.GetByEmail(ctx, alice.Email)
	if err != nil || found != alice {
		t.Fatalf("repeated up changed data: %+v, %v", found, err)
	}
}
