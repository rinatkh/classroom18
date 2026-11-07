// Package handler содержит HTTP-адаптеры Task API.
package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/rinatkh/classroom18/internal/httpapi/authctx"
	"github.com/rinatkh/classroom18/internal/httpapi/dto"
	"github.com/rinatkh/classroom18/internal/httpapi/mapper"
	"github.com/rinatkh/classroom18/internal/task"
)

// TaskUseCase — поведение, которое HTTP-слой ожидает от приложения.
// Интерфейс объявлен рядом с потребителем и удобен для handler tests.
type TaskUseCase interface {
	Create(ctx context.Context, userID int64, input task.CreateInput) (task.Task, error)
	Get(ctx context.Context, userID, id int64) (task.Task, error)
	List(ctx context.Context, userID int64) ([]task.Task, error)
	Update(ctx context.Context, userID, id int64, input task.UpdateInput) (task.Task, error)
	Delete(ctx context.Context, userID, id int64) error
}

type TaskHandler struct {
	useCase TaskUseCase
}

func NewTaskHandler(useCase TaskUseCase) *TaskHandler {
	return &TaskHandler{useCase: useCase}
}

func (h *TaskHandler) Health(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "HTTP request", "operation", "task.health")
	writeJSON(w, http.StatusOK, dto.HealthResponse{Status: "ok"})
}

// Create отвечает только за HTTP-часть создания задачи:
// читает DTO, вызывает UseCase и переводит результат в HTTP response.
func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "HTTP request", "operation", "task.create")
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	var request dto.CreateTaskRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	created, err := h.useCase.Create(r.Context(), userID, task.CreateInput{
		Title: request.Title,
	})
	if err != nil {
		writeUseCaseError(w, r, err)
		return
	}

	w.Header().Set("Location", fmt.Sprintf("/tasks/%d", created.ID))
	slog.InfoContext(r.Context(), "task created", "task_id", created.ID)
	writeJSON(w, http.StatusCreated, mapper.TaskToResponse(created))
}

func (h *TaskHandler) Get(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "HTTP request", "operation", "task.get")
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	result, err := h.useCase.Get(r.Context(), userID, id)
	if err != nil {
		writeUseCaseError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapper.TaskToResponse(result))
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "HTTP request", "operation", "task.list")
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	result, err := h.useCase.List(r.Context(), userID)
	if err != nil {
		writeUseCaseError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, mapper.TasksToResponse(result))
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "HTTP request", "operation", "task.update")
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	var request dto.UpdateTaskRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "некорректный JSON")
		return
	}

	updated, err := h.useCase.Update(r.Context(), userID, id, task.UpdateInput{
		Title: request.Title,
		Done:  request.Done,
	})
	if err != nil {
		writeUseCaseError(w, r, err)
		return
	}
	slog.InfoContext(r.Context(), "task updated", "task_id", updated.ID)
	writeJSON(w, http.StatusOK, mapper.TaskToResponse(updated))
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "HTTP request", "operation", "task.delete")
	userID, ok := requireUserID(w, r)
	if !ok {
		return
	}
	id, ok := parseID(w, r)
	if !ok {
		return
	}

	if err := h.useCase.Delete(r.Context(), userID, id); err != nil {
		writeUseCaseError(w, r, err)
		return
	}
	slog.InfoContext(r.Context(), "task deleted", "task_id", id)
	w.WriteHeader(http.StatusNoContent)
}

func parseID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writeError(w, http.StatusBadRequest, "id должен быть положительным целым числом")
		return 0, false
	}
	return id, true
}

func requireUserID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	userID, ok := authctx.UserID(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return 0, false
	}
	return userID, true
}
