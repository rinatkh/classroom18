package postgres

import (
	"context"
	"github.com/rinatkh/classroom18/internal/task"
)

var _ task.ProcessingRepository = (*TaskRepository)(nil)

func (r *TaskRepository) MarkProcessed(ctx context.Context, id int64) error {
	// Повторный UPDATE сохраняет updated_at. Идемпотентность выдерживает конкуренцию.
	tag, err := r.db.Exec(ctx, `UPDATE tasks SET processing_status='processed',
  updated_at=CASE WHEN processing_status='processed' THEN updated_at ELSE now() END
  WHERE id=$1`, id)
	if err != nil {
		return taskError("mark processed", err)
	}
	if tag.RowsAffected() == 0 {
		return task.ErrNotFound
	}
	return nil
}
