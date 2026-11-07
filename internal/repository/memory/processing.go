package memory

import (
	"context"
	"github.com/rinatkh/classroom18/internal/task"
	"time"
)

var _ task.ProcessingRepository = (*TaskRepository)(nil)

func (r *TaskRepository) MarkProcessed(ctx context.Context, id int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.tasks[id]
	if !ok {
		return task.ErrNotFound
	}
	if value.ProcessingStatus == task.Processed {
		return nil
	}
	value.ProcessingStatus = task.Processed
	value.UpdatedAt = time.Now().UTC()
	r.tasks[id] = value
	return nil
}
