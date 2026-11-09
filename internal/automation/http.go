// Package automation показывает gRPC client в отдельном процессе без database.
package automation

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	taskv1 "github.com/rinatkh/classroom18/pkg/proto/task/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

type Handler struct{ client taskv1.TaskServiceClient }

func NewRouter(client taskv1.TaskServiceClient) http.Handler {
	h := &Handler{client: client}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("POST /automation/tasks", h.create)
	mux.HandleFunc("GET /automation/tasks/{id}", h.get)
	return logRequests(mux)
}
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		UserID int64  `json:"user_id"`
		Title  string `json:"title"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "one JSON object required", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	result, err := h.client.CreateTask(ctx, &taskv1.CreateTaskRequest{UserId: input.UserID, Title: input.Title})
	h.respond(w, result, err, 201)
}
func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		http.Error(w, "positive id required", 400)
		return
	}
	userID, err := strconv.ParseInt(r.URL.Query().Get("user_id"), 10, 64)
	if err != nil || userID <= 0 {
		http.Error(w, "positive user_id required", 400)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	result, err := h.client.GetTask(ctx, &taskv1.GetTaskRequest{UserId: userID, TaskId: id})
	h.respond(w, result, err, 200)
}
func (h *Handler) respond(w http.ResponseWriter, result *taskv1.TaskResponse, err error, success int) {
	if err != nil {
		code := 502
		switch status.Code(err) {
		case codes.InvalidArgument:
			code = 400
		case codes.NotFound:
			code = 404
		case codes.PermissionDenied:
			code = 403
		case codes.Unauthenticated:
			code = 401
		case codes.FailedPrecondition:
			code = 409
		case codes.DeadlineExceeded:
			code = 504
		}
		http.Error(w, status.Code(err).String(), code)
		return
	}
	data, err := (protojson.MarshalOptions{UseProtoNames: true, EmitUnpopulated: true}).Marshal(result)
	if err != nil {
		http.Error(w, "internal error", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(success)
	_, _ = w.Write(append(data, '\n'))
}
