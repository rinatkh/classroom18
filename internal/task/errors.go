package task

import "errors"

var (
	ErrForbidden       = errors.New("нет доступа к задаче")
	ErrNotFound        = errors.New("задача не найдена")
	ErrEmptyTitle      = errors.New("заголовок задачи не должен быть пустым")
	ErrNoChanges       = errors.New("не указано ни одного поля для изменения")
	ErrAlreadyComplete = errors.New("задача уже завершена")
	ErrActiveLimit     = errors.New("достигнут лимит активных задач")
)
