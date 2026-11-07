// Package dto описывает внешний JSON-контракт HTTP API.
package dto

type CreateTaskRequest struct {
	Title string `json:"title"`
}

type UpdateTaskRequest struct {
	Title *string `json:"title"`
	Done  *bool   `json:"done"`
}

// TaskResponse сохраняет три знакомых поля API занятия 10.
// Внутренние CreatedAt и UpdatedAt не входят во внешний контракт.
// Внешний API может меняться независимо от способа хранения данных.
type TaskResponse struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
