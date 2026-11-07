// Package task содержит внутреннюю модель и бизнес-сценарии Task API.
package task

import "time"

// Task — внутренняя модель задачи.
//
// Здесь нет JSON-тегов, HTTP status или типов инфраструктуры. Внешний HTTP-контракт
// описан отдельными DTO, а способ хранения скрыт за Repository.
type Task struct {
	ID        int64
	OwnerID   int64
	Title     string
	Done      bool
	CreatedAt time.Time
	UpdatedAt time.Time
	// ProcessingStatus описывает работу worker, независимо от пользовательского Done.
	ProcessingStatus string
}

// CreateInput описывает данные, которые нужны бизнес-сценарию создания.
type CreateInput struct {
	Title string
}

// UpdateInput описывает частичное изменение Task.
// nil означает, что поле не прислали и его не нужно менять.
type UpdateInput struct {
	Title *string
	Done  *bool
}
