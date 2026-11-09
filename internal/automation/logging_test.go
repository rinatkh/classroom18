package automation

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/rinatkh/classroom18/internal/logging"
	taskv1 "github.com/rinatkh/classroom18/pkg/proto/task/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

func captureAutomationLogs(t *testing.T, level string) *bytes.Buffer {
	t.Helper()
	var output bytes.Buffer
	logger, err := logging.New(&output, "automation-service", level, "json")
	if err != nil {
		t.Fatal(err)
	}
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })
	return &output
}

func logRecords(t *testing.T, output *bytes.Buffer) []map[string]any {
	t.Helper()
	var result []map[string]any
	decoder := json.NewDecoder(bytes.NewReader(output.Bytes()))
	for {
		var record map[string]any
		if err := decoder.Decode(&record); err == io.EOF {
			return result
		} else if err != nil {
			t.Fatal(err)
		}
		result = append(result, record)
	}
}

func completedRecord(t *testing.T, records []map[string]any, message string) map[string]any {
	t.Helper()
	for _, record := range records {
		if record["msg"] == message {
			return record
		}
	}
	t.Fatalf("missing record %q", message)
	return nil
}

type loggingRPCServer struct {
	taskv1.UnimplementedTaskServiceServer
}

func (loggingRPCServer) CreateTask(_ context.Context, input *taskv1.CreateTaskRequest) (*taskv1.TaskResponse, error) {
	return &taskv1.TaskResponse{Id: 42, UserId: input.UserId, Title: input.Title}, nil
}

func TestHTTPAndRealRPCShareRequestIDWithoutPayload(t *testing.T) {
	output := captureAutomationLogs(t, "DEBUG")
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	taskv1.RegisterTaskServiceServer(server, loggingRPCServer{})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	connection, err := grpc.NewClient("passthrough:///logs-test",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(LogRPC))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	r := httptest.NewRequest("POST", "/automation/tasks?token=private-query",
		strings.NewReader(`{"user_id":7,"title":"private-title"}`))
	r.Header.Set("Authorization", "Bearer private-authorization")
	r.Header.Set("Cookie", "refresh_token=private-cookie")
	r.Header.Set("X-Request-ID", "private-header")
	w := httptest.NewRecorder()
	NewRouter(taskv1.NewTaskServiceClient(connection)).ServeHTTP(w, r)
	if w.Code != 201 || !strings.Contains(w.Body.String(), "private-title") {
		t.Fatalf("HTTP contract changed: %d", w.Code)
	}
	records := logRecords(t, output)
	httpLog := completedRecord(t, records, "automation HTTP request completed")
	rpcLog := completedRecord(t, records, "automation gRPC request completed")
	if httpLog["request_id"] == nil || httpLog["request_id"] != rpcLog["request_id"] {
		t.Fatal("HTTP/RPC correlation lost")
	}
	if httpLog["http_status"] != float64(201) || httpLog["route"] != "POST /automation/tasks" || rpcLog["grpc_code"] != "OK" || rpcLog["task_id"] != float64(42) {
		t.Fatal("outcome fields lost")
	}
	if httpLog["service"] != "automation-service" || rpcLog["duration_ms"] == nil || httpLog["duration_ms"] == nil {
		t.Fatal("service/duration fields lost")
	}
	for _, secret := range []string{"private-title", "private-query", "private-authorization", "private-cookie", "private-header"} {
		if strings.Contains(output.String(), secret) {
			t.Fatalf("request data leaked: %s", secret)
		}
	}
}

func TestRPCLogsSeverityAndPreservesErrorWithoutMessage(t *testing.T) {
	for _, tc := range []struct {
		code  codes.Code
		level string
	}{
		{codes.OK, "INFO"}, {codes.InvalidArgument, "WARN"}, {codes.NotFound, "WARN"},
		{codes.PermissionDenied, "WARN"}, {codes.Unauthenticated, "WARN"},
		{codes.FailedPrecondition, "WARN"}, {codes.Canceled, "WARN"},
		{codes.DeadlineExceeded, "ERROR"}, {codes.Unavailable, "ERROR"},
		{codes.Internal, "ERROR"}, {codes.Unknown, "ERROR"},
	} {
		t.Run(tc.code.String(), func(t *testing.T) {
			output := captureAutomationLogs(t, "DEBUG")
			ctx := context.WithValue(context.Background(), requestIDKey{}, uint64(99))
			expected := status.Error(tc.code, "private-upstream-message")
			called := false
			err := LogRPC(ctx, "/task.v1.TaskService/GetTask", "private-request", "private-reply", nil,
				func(actual context.Context, method string, req, reply any, connection *grpc.ClientConn, options ...grpc.CallOption) error {
					called = true
					if actual != ctx || method != "/task.v1.TaskService/GetTask" || req != "private-request" || reply != "private-reply" {
						t.Fatal("interceptor changed invocation")
					}
					return expected
				})
			if !called || err != expected {
				t.Fatal("interceptor changed returned error")
			}
			record := completedRecord(t, logRecords(t, output), "automation gRPC request completed")
			if record["grpc_code"] != tc.code.String() || record["level"] != tc.level || record["request_id"] != float64(99) {
				t.Fatalf("wrong outcome: %v", record)
			}
			for _, secret := range []string{"private-upstream-message", "private-request", "private-reply"} {
				if strings.Contains(output.String(), secret) {
					t.Fatal("RPC data leaked")
				}
			}
		})
	}
}

func TestRejectedHTTPLogsNoRawPathOrQuery(t *testing.T) {
	for _, tc := range []struct {
		method, path, body string
		code               int
	}{
		{"POST", "/automation/tasks?secret=private-query", `{"title":"private-body",`, 400},
		{"GET", "/automation/tasks/private-path?user_id=7", "", 400},
		{"GET", "/private-path?token=private-query", "", 404},
		{"PUT", "/automation/tasks?token=private-query", "", 405},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			output := captureAutomationLogs(t, "INFO")
			client := rpcStub{call: func(context.Context, int64, int64, string) (*taskv1.TaskResponse, error) {
				t.Fatal("invalid HTTP request reached RPC")
				return nil, nil
			}}
			w := httptest.NewRecorder()
			NewRouter(client).ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
			record := completedRecord(t, logRecords(t, output), "automation HTTP request completed")
			if w.Code != tc.code || record["http_status"] != float64(tc.code) || record["level"] != "WARN" {
				t.Fatalf("invalid request outcome: %v", record)
			}
			for _, secret := range []string{"private-body", "private-path", "private-query"} {
				if strings.Contains(output.String(), secret) {
					t.Fatal("HTTP data leaked")
				}
			}
		})
	}
}

func TestHealthProbeIsQuietAtInfo(t *testing.T) {
	output := captureAutomationLogs(t, "INFO")
	w := httptest.NewRecorder()
	NewRouter(nil).ServeHTTP(w, httptest.NewRequest("GET", "/health", nil))
	if w.Code != 200 || output.Len() != 0 {
		t.Fatal("successful health probe should only log at DEBUG")
	}
}

type failingHTTPWriter struct{ headers http.Header }

func (w failingHTTPWriter) Header() http.Header { return w.headers }
func (failingHTTPWriter) WriteHeader(int)       {}
func (failingHTTPWriter) Write([]byte) (int, error) {
	return 0, errors.New("private-write-error")
}

func TestResponseWriteFailureIsLoggedWithoutRawError(t *testing.T) {
	output := captureAutomationLogs(t, "INFO")
	client := rpcStub{call: func(context.Context, int64, int64, string) (*taskv1.TaskResponse, error) {
		return &taskv1.TaskResponse{Id: 3}, nil
	}}
	NewRouter(client).ServeHTTP(failingHTTPWriter{make(http.Header)}, httptest.NewRequest("GET", "/automation/tasks/3?user_id=7", nil))
	record := completedRecord(t, logRecords(t, output), "automation HTTP request completed")
	if record["level"] != "ERROR" || record["error_type"] == nil || strings.Contains(output.String(), "private-write-error") {
		t.Fatalf("write failure not safely logged: %v", record)
	}
}

func TestConcurrentRequestsHaveDifferentIDs(t *testing.T) {
	output := captureAutomationLogs(t, "INFO")
	client := rpcStub{call: func(context.Context, int64, int64, string) (*taskv1.TaskResponse, error) {
		return &taskv1.TaskResponse{Id: 3}, nil
	}}
	router := NewRouter(client)
	var finished sync.WaitGroup
	for i := 0; i < 16; i++ {
		finished.Add(1)
		go func() {
			defer finished.Done()
			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/automation/tasks/3?user_id=7", nil))
		}()
	}
	finished.Wait()
	ids := make(map[any]bool)
	for _, record := range logRecords(t, output) {
		id := record["request_id"]
		if id == nil || ids[id] {
			t.Fatal("request IDs are not unique")
		}
		ids[id] = true
	}
	if len(ids) != 16 {
		t.Fatal("missing request records")
	}
}
