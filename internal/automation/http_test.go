package automation

import (
	"context"
	taskv1 "github.com/rinatkh/classroom18/pkg/proto/task/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type rpcStub struct {
	call func(context.Context, int64, int64, string) (*taskv1.TaskResponse, error)
}

func (s rpcStub) CreateTask(ctx context.Context, r *taskv1.CreateTaskRequest, o ...grpc.CallOption) (*taskv1.TaskResponse, error) {
	return s.call(ctx, r.GetUserId(), 0, r.GetTitle())
}
func (s rpcStub) GetTask(ctx context.Context, r *taskv1.GetTaskRequest, o ...grpc.CallOption) (*taskv1.TaskResponse, error) {
	return s.call(ctx, r.GetUserId(), r.GetTaskId(), "")
}
func TestFacadePreservesDeadlineAndCancelsRPCContext(t *testing.T) {
	for _, method := range []string{"create", "get"} {
		for _, parent := range []string{"own-budget", "short-parent", "canceled-parent"} {
			t.Run(method+"/"+parent, func(t *testing.T) {
				ctx := context.Background()
				cleanup := func() {}
				if parent == "short-parent" {
					ctx, cleanup = context.WithTimeout(ctx, time.Second)
				}
				if parent == "canceled-parent" {
					var cancel context.CancelFunc
					ctx, cancel = context.WithCancel(ctx)
					cancel()
				}
				defer cleanup()
				var rpcctx context.Context
				client := rpcStub{call: func(c context.Context, uid, id int64, title string) (*taskv1.TaskResponse, error) {
					rpcctx = c
					deadline, ok := c.Deadline()
					budget := 6 * time.Second
					if method == "get" {
						budget = 3 * time.Second
					}
					if parent == "short-parent" {
						budget = time.Second
					}
					if !ok || time.Until(deadline) > budget {
						t.Fatal("deadline not bounded")
					}
					if uid != 7 || method == "get" && id != 3 || method == "create" && title != "title" {
						t.Fatal("mapping")
					}
					if parent == "canceled-parent" && c.Err() != context.Canceled {
						t.Fatal("cancellation lost")
					}
					return &taskv1.TaskResponse{Id: 3, Title: "title"}, nil
				}}
				path, verb, body := "/automation/tasks", "POST", `{"user_id":7,"title":"title"}`
				if method == "get" {
					path, verb, body = "/automation/tasks/3?user_id=7", "GET", ""
				}
				r := httptest.NewRequest(verb, path, strings.NewReader(body)).WithContext(ctx)
				w := httptest.NewRecorder()
				NewRouter(client).ServeHTTP(w, r)
				if w.Code != 201 && w.Code != 200 {
					t.Fatal(w.Code)
				}
				if rpcctx == nil || rpcctx.Err() != context.Canceled {
					t.Fatal("RPC context not cleaned up")
				}
			})
		}
	}
}
func TestFacadeRejectsMalformedInputBeforeRPC(t *testing.T) {
	for _, tc := range []struct{ name, method, path, body string }{
		{"broken JSON", "POST", "/automation/tasks", "{"}, {"unknown field", "POST", "/automation/tasks", `{"user_id":7,"title":"x","extra":1}`}, {"two values", "POST", "/automation/tasks", `{} {}`}, {"oversized", "POST", "/automation/tasks", `{"title":"` + strings.Repeat("x", 65536) + `"}`}, {"negative task", "GET", "/automation/tasks/-1?user_id=7", ""}, {"bad task", "GET", "/automation/tasks/no?user_id=7", ""}, {"no actor", "GET", "/automation/tasks/1", ""}, {"zero actor", "GET", "/automation/tasks/1?user_id=0", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			c := rpcStub{call: func(context.Context, int64, int64, string) (*taskv1.TaskResponse, error) { calls++; return nil, nil }}
			w := httptest.NewRecorder()
			NewRouter(c).ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			if w.Code != 400 || calls != 0 {
				t.Fatal(w.Code, calls)
			}
		})
	}
}
func TestFacadeMapsRPCStatusWithoutRawError(t *testing.T) {
	for _, tc := range []struct {
		code codes.Code
		http int
	}{{codes.InvalidArgument, 400}, {codes.NotFound, 404}, {codes.PermissionDenied, 403}, {codes.Unauthenticated, 401}, {codes.FailedPrecondition, 409}, {codes.DeadlineExceeded, 504}, {codes.Unavailable, 502}, {codes.Internal, 502}} {
		t.Run(tc.code.String(), func(t *testing.T) {
			c := rpcStub{call: func(context.Context, int64, int64, string) (*taskv1.TaskResponse, error) {
				return nil, status.Error(tc.code, "private details")
			}}
			w := httptest.NewRecorder()
			NewRouter(c).ServeHTTP(w, httptest.NewRequest("GET", "/automation/tasks/1?user_id=7", nil))
			if w.Code != tc.http || strings.Contains(w.Body.String(), "private details") {
				t.Fatal(w.Code, w.Body.String())
			}
		})
	}
}
