package task

import "context"

// Repository описывает только те операции с данными, которые нужны UseCase.
// Интерфейс находится рядом с потребителем. UseCase не знает, выполняются ли
// операции через память или тестовый fake.
type Repository interface {
	Create(ctx context.Context, ownerID int64, input CreateInput) (Task, error)
	Get(ctx context.Context, id int64) (Task, error)
	List(ctx context.Context, ownerID int64) ([]Task, error)
	Update(ctx context.Context, id int64, input UpdateInput) (Task, error)
	Delete(ctx context.Context, id int64) error
	CountActive(ctx context.Context, ownerID int64) (int, error)
}
