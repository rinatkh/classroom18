package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/rinatkh/classroom18/internal/httpapi/dto"
	"github.com/rinatkh/classroom18/internal/logging"
	"github.com/rinatkh/classroom18/internal/task"
)

func decodeJSON(r *http.Request, destination any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("прочитать JSON: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("body должен содержать один JSON-объект")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		logging.Error(context.Background(), "HTTP response write failed", err, "status", status)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	slog.Debug("HTTP request rejected", "status", status)
	writeJSON(w, status, dto.ErrorResponse{Error: message})
}

// writeUseCaseError переводит ошибки приложения в HTTP contract.
// Детали внутренней ошибки хранилища не уходят клиенту целиком.
func writeUseCaseError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, task.ErrForbidden):
		writeError(w, http.StatusForbidden, task.ErrForbidden.Error())
	case errors.Is(err, task.ErrNotFound):
		writeError(w, http.StatusNotFound, task.ErrNotFound.Error())
	case errors.Is(err, task.ErrEmptyTitle), errors.Is(err, task.ErrNoChanges):
		writeError(w, http.StatusBadRequest, knownMessage(err))
	case errors.Is(err, task.ErrAlreadyComplete), errors.Is(err, task.ErrActiveLimit):
		writeError(w, http.StatusConflict, knownMessage(err))
	default:
		logging.Error(r.Context(), "task operation failed", err)
		writeError(w, http.StatusInternalServerError, "внутренняя ошибка сервера")
	}
}

func knownMessage(err error) string {
	known := []error{
		task.ErrEmptyTitle,
		task.ErrNoChanges,
		task.ErrAlreadyComplete,
		task.ErrActiveLimit,
	}
	for _, candidate := range known {
		if errors.Is(err, candidate) {
			return candidate.Error()
		}
	}
	return err.Error()
}
