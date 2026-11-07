package task

import (
	"context"
	"errors"
	"fmt"
)

// ProcessedHandler принадлежит приложению-владельцу Task, не worker.
type ProcessedHandler struct{ repository ProcessingRepository }

func NewProcessedHandler(repository ProcessingRepository) *ProcessedHandler {
	return &ProcessedHandler{repository: repository}
}
func (h *ProcessedHandler) Handle(ctx context.Context, event Event) error {
	if err := event.Validate(); err != nil {
		return err
	}
	if event.Type != TaskProcessed {
		return nil
	}
	current, err := h.repository.Get(ctx, event.TaskID)
	// Task могли удалить после Create, до async результата. Повторять нечего.
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if current.OwnerID != event.UserID {
		return fmt.Errorf("%w: owner mismatch", ErrInvalidEvent)
	}
	if current.ProcessingStatus == Processed {
		return nil
	}
	if err := h.repository.MarkProcessed(ctx, current.ID); errors.Is(err, ErrNotFound) {
		return nil
	} else {
		return err
	}
}
