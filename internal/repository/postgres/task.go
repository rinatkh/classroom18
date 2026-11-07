package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"

	"github.com/rinatkh/classroom18/internal/task"
)

type TaskRepository struct {
	db DBTX
}

var _ task.Repository = (*TaskRepository)(nil)

func NewTaskRepository(db DBTX) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) Create(ctx context.Context, ownerID int64, input task.CreateInput) (task.Task, error) {
	slog.DebugContext(ctx, "task repository operation", "adapter", "postgres", "operation", "create")
	const query = `
		INSERT INTO tasks (owner_id, title)
		VALUES ($1, $2)
		RETURNING id, owner_id, title, done, created_at, updated_at, processing_status`
	var result task.Task
	if err := scanTask(r.db.QueryRow(ctx, query, ownerID, input.Title), &result); err != nil {
		return task.Task{}, fmt.Errorf("insert task: %w", err)
	}
	return result, nil
}

func (r *TaskRepository) Get(ctx context.Context, id int64) (task.Task, error) {
	slog.DebugContext(ctx, "task repository operation", "adapter", "postgres", "operation", "get")
	const query = `
		SELECT id, owner_id, title, done, created_at, updated_at, processing_status
		FROM tasks WHERE id = $1`
	var result task.Task
	if err := scanTask(r.db.QueryRow(ctx, query, id), &result); err != nil {
		return task.Task{}, taskError("select task", err)
	}
	// Загружаем данные. Сравнение OwnerID с текущим userID остаётся в UseCase.
	return result, nil
}

func (r *TaskRepository) List(ctx context.Context, ownerID int64) ([]task.Task, error) {
	slog.DebugContext(ctx, "task repository operation", "adapter", "postgres", "operation", "list")
	const query = `
		SELECT id, owner_id, title, done, created_at, updated_at, processing_status
		FROM tasks WHERE owner_id = $1 ORDER BY id ASC`
	rows, err := r.db.Query(ctx, query, ownerID)
	if err != nil {
		return nil, fmt.Errorf("select tasks by owner: %w", err)
	}
	defer rows.Close()
	result := make([]task.Task, 0)
	for rows.Next() {
		var current task.Task
		if err := rows.Scan(&current.ID, &current.OwnerID, &current.Title, &current.Done, &current.CreatedAt, &current.UpdatedAt, &current.ProcessingStatus); err != nil {
			return nil, fmt.Errorf("scan task: %w", err)
		}
		result = append(result, current)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate tasks: %w", err)
	}
	return result, nil
}

func (r *TaskRepository) Update(ctx context.Context, id int64, input task.UpdateInput) (task.Task, error) {
	slog.DebugContext(ctx, "task repository operation", "adapter", "postgres", "operation", "update")
	const query = `
		UPDATE tasks
		SET title = COALESCE($2, title), done = COALESCE($3, done), updated_at = now()
		WHERE id = $1
		RETURNING id, owner_id, title, done, created_at, updated_at, processing_status`
	var result task.Task
	if err := scanTask(r.db.QueryRow(ctx, query, id, input.Title, input.Done), &result); err != nil {
		return task.Task{}, taskError("update task", err)
	}
	return result, nil
}

func (r *TaskRepository) Delete(ctx context.Context, id int64) error {
	slog.DebugContext(ctx, "task repository operation", "adapter", "postgres", "operation", "delete")
	// Exec нужен, когда возвращаемые строки не нужны. RowsAffected отличает
	// удалённую запись от отсутствующей; обе SQL-команды сами по себе успешны.
	tag, err := r.db.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return task.ErrNotFound
	}
	return nil
}

func (r *TaskRepository) CountActive(ctx context.Context, ownerID int64) (int, error) {
	slog.DebugContext(ctx, "task repository operation", "adapter", "postgres", "operation", "countactive")
	const query = `SELECT count(*) FROM tasks WHERE owner_id = $1 AND done = false`
	var count int
	if err := r.db.QueryRow(ctx, query, ownerID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active tasks: %w", err)
	}
	return count, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTask(row rowScanner, value *task.Task) error {
	return row.Scan(&value.ID, &value.OwnerID, &value.Title, &value.Done, &value.CreatedAt, &value.UpdatedAt, &value.ProcessingStatus)
}

func taskError(operation string, err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return task.ErrNotFound
	}
	return fmt.Errorf("%s: %w", operation, err)
}
