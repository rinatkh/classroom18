package grpcapi

import (
	"context"
	"github.com/rinatkh/classroom18/internal/repository/memory"
	"github.com/rinatkh/classroom18/internal/task"
	taskv1 "github.com/rinatkh/classroom18/pkg/proto/task/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"net"
	"testing"
	"time"
)

func TestGRPCUsesExistingTaskUseCase(t *testing.T) {
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	taskv1.RegisterTaskServiceServer(server, NewTaskServer(task.NewUseCase(memory.NewTaskRepository())))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	client := taskv1.NewTaskServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	value, err := client.CreateTask(ctx, &taskv1.CreateTaskRequest{UserId: 7, Title: "  via gRPC "})
	if err != nil || value.GetTitle() != "via gRPC" || value.GetProcessingStatus() != task.Pending {
		t.Fatalf("create: %v %v", value, err)
	}
	own, err := client.GetTask(ctx, &taskv1.GetTaskRequest{UserId: 7, TaskId: value.Id})
	if err != nil || own.Id != value.Id {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		req  *taskv1.GetTaskRequest
		code codes.Code
	}{
		{"foreign", &taskv1.GetTaskRequest{UserId: 8, TaskId: value.Id}, codes.PermissionDenied},
		{"missing", &taskv1.GetTaskRequest{UserId: 7, TaskId: 999}, codes.NotFound},
		{"invalid", &taskv1.GetTaskRequest{UserId: 7}, codes.InvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.GetTask(ctx, tc.req)
			if status.Code(err) != tc.code {
				t.Fatalf("%v", err)
			}
		})
	}
	if _, err := client.CreateTask(ctx, &taskv1.CreateTaskRequest{UserId: 7, Title: " "}); status.Code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
	canceled, stop := context.WithCancel(ctx)
	stop()
	if _, err := client.GetTask(canceled, &taskv1.GetTaskRequest{UserId: 7, TaskId: 1}); status.Code(err) != codes.Canceled {
		t.Fatal(err)
	}
}
func TestStatusMapping(t *testing.T) {
	for _, tc := range []struct {
		err  error
		code codes.Code
	}{
		{context.DeadlineExceeded, codes.DeadlineExceeded}, {context.Canceled, codes.Canceled},
		{task.ErrActiveLimit, codes.FailedPrecondition}, {task.ErrForbidden, codes.PermissionDenied},
		{task.ErrEventPublish, codes.Internal},
	} {
		if status.Code(mapError(tc.err)) != tc.code {
			t.Fatal(tc)
		}
	}
}
