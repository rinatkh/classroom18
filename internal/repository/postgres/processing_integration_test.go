//go:build integration

package postgres

import (
	"github.com/rinatkh/classroom18/internal/task"
	"testing"
)

func TestIntegrationProcessingIsIdempotent(t *testing.T) {
	pool, ctx := integrationPool(t)
	var owner int64
	if err := pool.QueryRow(ctx, `INSERT INTO users(email,password_hash) VALUES($1,$2) RETURNING id`, "processing@example.test", "hash").Scan(&owner); err != nil {
		t.Fatal(err)
	}
	repo := NewTaskRepository(pool)
	value, err := repo.Create(ctx, owner, task.CreateInput{Title: "process me"})
	if err != nil {
		t.Fatal(err)
	}
	if value.ProcessingStatus != task.Pending {
		t.Fatal(value)
	}
	if err := repo.MarkProcessed(ctx, value.ID); err != nil {
		t.Fatal(err)
	}
	first, err := repo.Get(ctx, value.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.MarkProcessed(ctx, value.ID); err != nil {
		t.Fatal(err)
	}
	second, err := repo.Get(ctx, value.ID)
	if err != nil || second.Done || second.ProcessingStatus != task.Processed || !first.UpdatedAt.Equal(second.UpdatedAt) {
		t.Fatalf("%+v %v", second, err)
	}
}
