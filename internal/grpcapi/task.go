// Package grpcapi — второй transport adapter над тем же TaskUseCase.
package grpcapi

import (
	"context"
	"errors"
	"github.com/rinatkh/classroom18/internal/task"
	taskv1 "github.com/rinatkh/classroom18/pkg/proto/task/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TaskUseCase interface {
	Create(context.Context, int64, task.CreateInput) (task.Task, error)
	Get(context.Context, int64, int64) (task.Task, error)
}
type TaskServer struct {
	taskv1.UnimplementedTaskServiceServer
	useCase TaskUseCase
}

func NewTaskServer(useCase TaskUseCase) *TaskServer { return &TaskServer{useCase: useCase} }

func (s *TaskServer) CreateTask(ctx context.Context, r *taskv1.CreateTaskRequest) (*taskv1.TaskResponse, error) {
	if r == nil || r.UserId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "positive user_id required")
	}
	value, err := s.useCase.Create(ctx, r.UserId, task.CreateInput{Title: r.Title})
	if err != nil {
		return nil, mapError(err)
	}
	return response(value), nil
}
func (s *TaskServer) GetTask(ctx context.Context, r *taskv1.GetTaskRequest) (*taskv1.TaskResponse, error) {
	if r == nil || r.UserId <= 0 || r.TaskId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "positive user_id and task_id required")
	}
	value, err := s.useCase.Get(ctx, r.UserId, r.TaskId)
	if err != nil {
		return nil, mapError(err)
	}
	return response(value), nil
}
func response(v task.Task) *taskv1.TaskResponse {
	return &taskv1.TaskResponse{Id: v.ID, UserId: v.OwnerID, Title: v.Title, Done: v.Done, ProcessingStatus: v.ProcessingStatus}
}
func mapError(err error) error {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, task.ErrEmptyTitle), errors.Is(err, task.ErrNoChanges):
		return status.Error(codes.InvalidArgument, "invalid task input")
	case errors.Is(err, task.ErrNotFound):
		return status.Error(codes.NotFound, "task not found")
	case errors.Is(err, task.ErrForbidden):
		return status.Error(codes.PermissionDenied, "task belongs to another user")
	case errors.Is(err, task.ErrActiveLimit), errors.Is(err, task.ErrAlreadyComplete):
		return status.Error(codes.FailedPrecondition, "task rule rejected")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}
