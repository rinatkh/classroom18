package automation

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/rinatkh/classroom18/internal/logging"
	taskv1 "github.com/rinatkh/classroom18/pkg/proto/task/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type requestIDKey struct{}

var requestSequence atomic.Uint64

// Локальный request_id связывает входящий HTTP и исходящий RPC в одном процессе.
// Не принимаем его из заголовка клиента и не записываем URL, query или тело.
func logRequests(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, route := mux.Handler(r)
		switch route {
		case "GET /health", "POST /automation/tasks", "GET /automation/tasks/{id}":
		default:
			route = "unmatched"
		}
		id := requestSequence.Add(1)
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		attrs := []any{"request_id", id, "http_method", r.Method, "route", route}
		slog.DebugContext(ctx, "automation HTTP request started", attrs...)
		start := time.Now()
		response := &loggedResponse{ResponseWriter: w}
		// Запрос всё ещё проходит через ServeMux: он устанавливает PathValue.
		mux.ServeHTTP(response, r.WithContext(ctx))
		code := response.code
		if code == 0 {
			code = http.StatusOK
		}
		attrs = append(attrs, "http_status", code, "duration_ms", time.Since(start).Milliseconds())
		level := slog.LevelInfo
		switch {
		case response.writeErr != nil:
			level = slog.LevelError
			attrs = append(attrs, "error_type", logging.ErrorType(response.writeErr))
		case code >= 500:
			level = slog.LevelError
		case code >= 400:
			level = slog.LevelWarn
		case route == "GET /health":
			// Readiness/liveness probes не засоряют обычные INFO логи.
			level = slog.LevelDebug
		}
		slog.Log(ctx, level, "automation HTTP request completed", attrs...)
	})
}

type loggedResponse struct {
	http.ResponseWriter
	code     int
	writeErr error
}

func (w *loggedResponse) WriteHeader(code int) {
	if w.code == 0 && code >= 200 {
		w.code = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *loggedResponse) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	if err != nil {
		w.writeErr = err
	}
	return n, err
}

func (w *loggedResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// LogRPC наблюдает исходящий unary call, не меняя context, результат или retry policy.
// Raw status message, request/reply payload и metadata могут содержать секреты.
func LogRPC(ctx context.Context, method string, req, reply any, connection *grpc.ClientConn, invoke grpc.UnaryInvoker, options ...grpc.CallOption) error {
	attrs := []any{"grpc_method", method}
	if id, ok := ctx.Value(requestIDKey{}).(uint64); ok {
		attrs = append(attrs, "request_id", id)
	}
	slog.DebugContext(ctx, "automation gRPC request started", attrs...)
	start := time.Now()
	err := invoke(ctx, method, req, reply, connection, options...)
	code := status.Code(err)
	attrs = append(attrs, "grpc_code", code.String(), "duration_ms", time.Since(start).Milliseconds())
	level := slog.LevelInfo
	if err != nil {
		level = slog.LevelError
		switch code {
		case codes.InvalidArgument, codes.NotFound, codes.PermissionDenied, codes.Unauthenticated, codes.FailedPrecondition, codes.Canceled:
			level = slog.LevelWarn
		}
		attrs = append(attrs, "error_type", logging.ErrorType(err))
	} else if value, ok := reply.(*taskv1.TaskResponse); ok {
		attrs = append(attrs, "task_id", value.GetId())
	}
	slog.Log(ctx, level, "automation gRPC request completed", attrs...)
	return err
}
