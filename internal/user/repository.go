package user

import "context"

// Repository описан рядом с UseCase, который потребляет это поведение.
// Memory implementation находится во внешнем пакете repository/memory.
type Repository interface {
	Create(ctx context.Context, value User) (User, error)
	GetByEmail(ctx context.Context, email string) (User, error)
}
