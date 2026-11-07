// Package mapper преобразует внутренние модели в HTTP DTO.
package mapper

import (
	"github.com/rinatkh/classroom18/internal/httpapi/dto"
	"github.com/rinatkh/classroom18/internal/task"
)

// TaskToResponse не принимает бизнес-решений. Функция только переводит
// внутреннюю форму данных в публичный HTTP-контракт.
func TaskToResponse(value task.Task) dto.TaskResponse {
	return dto.TaskResponse{
		ID:    value.ID,
		Title: value.Title,
		Done:  value.Done,
	}
}

func TasksToResponse(values []task.Task) []dto.TaskResponse {
	result := make([]dto.TaskResponse, 0, len(values))
	for _, value := range values {
		result = append(result, TaskToResponse(value))
	}
	return result
}
