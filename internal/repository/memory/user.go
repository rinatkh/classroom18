// Package memory содержит потокобезопасные in-memory реализации repository.
// Это infrastructure detail: usecase видит только интерфейсы user.Repository
// и task.Repository.
package memory

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/rinatkh/classroom18/internal/user"
)

type UserRepository struct {
	mu      sync.RWMutex
	nextID  int64
	byEmail map[string]user.User
}

func NewUserRepository() *UserRepository {
	return &UserRepository{nextID: 1, byEmail: make(map[string]user.User)}
}

func (r *UserRepository) Create(ctx context.Context, value user.User) (user.User, error) {
	slog.DebugContext(ctx, "repository operation", "adapter", "memory", "repository", "UserRepository", "operation", "Create")
	r.mu.Lock()
	defer r.mu.Unlock()

	key := strings.ToLower(value.Email)
	if _, exists := r.byEmail[key]; exists {
		return user.User{}, user.ErrAlreadyExists
	}
	value.ID = r.nextID
	r.nextID++
	r.byEmail[key] = value
	return value, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (user.User, error) {
	slog.DebugContext(ctx, "repository operation", "adapter", "memory", "repository", "UserRepository", "operation", "GetByEmail")
	r.mu.RLock()
	defer r.mu.RUnlock()

	value, ok := r.byEmail[strings.ToLower(email)]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return value, nil
}

var _ user.Repository = (*UserRepository)(nil)
