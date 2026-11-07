// Package memory содержит in-memory реализации Repository.
package memory

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/rinatkh/classroom18/internal/task"
)

// TaskRepository — развитие Store занятия 10: та же map под RWMutex.
// Проверки title, изменений и лимитов переехали в UseCase.
type TaskRepository struct {
	mu     sync.RWMutex
	tasks  map[int64]task.Task
	nextID int64
}

var _ task.Repository = (*TaskRepository)(nil)

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{tasks: make(map[int64]task.Task), nextID: 1}
}

func (r *TaskRepository) Create(ctx context.Context, ownerID int64, input task.CreateInput) (task.Task, error) {
	slog.DebugContext(ctx, "task repository operation", "adapter", "memory", "operation", "create")
	if err := ctx.Err(); err != nil {
		return task.Task{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	value := task.Task{ID: r.nextID, OwnerID: ownerID, Title: input.Title, CreatedAt: now, UpdatedAt: now, ProcessingStatus: task.Pending}
	r.tasks[value.ID] = value
	r.nextID++
	return value, nil
}

func (r *TaskRepository) Get(ctx context.Context, id int64) (task.Task, error) {
	slog.DebugContext(ctx, "task repository operation", "adapter", "memory", "operation", "get")
	if err := ctx.Err(); err != nil {
		return task.Task{}, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	value, ok := r.tasks[id]
	if !ok {
		return task.Task{}, task.ErrNotFound
	}
	return value, nil
}

func (r *TaskRepository) List(ctx context.Context, ownerID int64) ([]task.Task, error) {
	slog.DebugContext(ctx, "task repository operation", "adapter", "memory", "operation", "list")
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]task.Task, 0, len(r.tasks))
	for _, value := range r.tasks {
		if value.OwnerID == ownerID {
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (r *TaskRepository) Update(ctx context.Context, id int64, input task.UpdateInput) (task.Task, error) {
	slog.DebugContext(ctx, "task repository operation", "adapter", "memory", "operation", "update")
	if err := ctx.Err(); err != nil {
		return task.Task{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.tasks[id]
	if !ok {
		return task.Task{}, task.ErrNotFound
	}
	if input.Title != nil {
		value.Title = *input.Title
	}
	if input.Done != nil {
		value.Done = *input.Done
	}
	value.UpdatedAt = time.Now().UTC()
	r.tasks[id] = value
	return value, nil
}

func (r *TaskRepository) Delete(ctx context.Context, id int64) error {
	slog.DebugContext(ctx, "task repository operation", "adapter", "memory", "operation", "delete")
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.tasks[id]; !ok {
		return task.ErrNotFound
	}
	delete(r.tasks, id)
	return nil
}

func (r *TaskRepository) CountActive(ctx context.Context, ownerID int64) (int, error) {
	slog.DebugContext(ctx, "task repository operation", "adapter", "memory", "operation", "countactive")
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, value := range r.tasks {
		if value.OwnerID == ownerID && !value.Done {
			count++
		}
	}
	return count, nil
}
