package handler_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rinatkh/classroom18/internal/httpapi"
	"github.com/rinatkh/classroom18/internal/httpapi/handler"
	"github.com/rinatkh/classroom18/internal/httpapi/middleware"
	"github.com/rinatkh/classroom18/internal/task"
)

type fakeUseCase struct {
	createFn func(context.Context, int64, task.CreateInput) (task.Task, error)
	getFn    func(context.Context, int64, int64) (task.Task, error)
	listFn   func(context.Context, int64) ([]task.Task, error)
	updateFn func(context.Context, int64, int64, task.UpdateInput) (task.Task, error)
	deleteFn func(context.Context, int64, int64) error
}

func (f *fakeUseCase) Create(ctx context.Context, userID int64, input task.CreateInput) (task.Task, error) {
	return f.createFn(ctx, userID, input)
}
func (f *fakeUseCase) Get(ctx context.Context, userID, id int64) (task.Task, error) {
	return f.getFn(ctx, userID, id)
}
func (f *fakeUseCase) List(ctx context.Context, userID int64) ([]task.Task, error) {
	return f.listFn(ctx, userID)
}
func (f *fakeUseCase) Update(
	ctx context.Context,
	userID, id int64,
	input task.UpdateInput,
) (task.Task, error) {
	return f.updateFn(ctx, userID, id, input)
}
func (f *fakeUseCase) Delete(ctx context.Context, userID, id int64) error {
	return f.deleteFn(ctx, userID, id)
}

type validVerifier struct{}

func (validVerifier) Verify(string) (int64, error) { return 1, nil }

func newRouter(useCase *fakeUseCase) http.Handler {
	router := httpapi.NewRouter(handler.NewAuthHandler(nil, handler.CookieConfig{Name: "refresh_token"}), handler.NewTaskHandler(useCase), middleware.NewAuth(validVerifier{}))
	// Проверяем Task transport с подставленной identity, без зависимости от JWT.
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer unit-test")
		router.ServeHTTP(w, r)
	})
}

func TestCreateInvalidJSONReturns400(t *testing.T) {
	useCase := &fakeUseCase{
		createFn: func(context.Context, int64, task.CreateInput) (task.Task, error) {
			t.Fatal("use case не должен вызываться для сломанного JSON")
			return task.Task{}, nil
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/tasks", strings.NewReader(`{"title":`))
	response := httptest.NewRecorder()

	newRouter(useCase).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", response.Code)
	}
}

func TestCreateBusinessValidationReturns400(t *testing.T) {
	useCase := &fakeUseCase{
		createFn: func(context.Context, int64, task.CreateInput) (task.Task, error) {
			return task.Task{}, task.ErrEmptyTitle
		},
	}
	request := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBufferString(`{"title":" "}`))
	response := httptest.NewRecorder()

	newRouter(useCase).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", response.Code)
	}
}

func TestGetNotFoundReturns404(t *testing.T) {
	useCase := &fakeUseCase{
		getFn: func(context.Context, int64, int64) (task.Task, error) {
			return task.Task{}, task.ErrNotFound
		},
	}
	request := httptest.NewRequest(http.MethodGet, "/tasks/99", nil)
	response := httptest.NewRecorder()

	newRouter(useCase).ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", response.Code)
	}
}

func TestUpdateConflictReturns409(t *testing.T) {
	useCase := &fakeUseCase{
		updateFn: func(context.Context, int64, int64, task.UpdateInput) (task.Task, error) {
			return task.Task{}, task.ErrAlreadyComplete
		},
	}
	request := httptest.NewRequest(
		http.MethodPatch,
		"/tasks/1",
		strings.NewReader(`{"done":true}`),
	)
	response := httptest.NewRecorder()

	newRouter(useCase).ServeHTTP(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d, want 409", response.Code)
	}
}

func TestCreateSuccessReturns201AndPublicDTO(t *testing.T) {
	createdAt := time.Date(2026, time.September, 22, 12, 0, 0, 0, time.UTC)
	useCase := &fakeUseCase{
		createFn: func(_ context.Context, userID int64, input task.CreateInput) (task.Task, error) {
			return task.Task{
				ID:        42,
				Title:     input.Title,
				CreatedAt: createdAt,
				UpdatedAt: createdAt.Add(time.Hour),
			}, nil
		},
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/tasks",
		strings.NewReader(`{"title":"Learn architecture"}`),
	)
	response := httptest.NewRecorder()

	newRouter(useCase).ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d, want 201", response.Code)
	}
	if location := response.Header().Get("Location"); location != "/tasks/42" {
		t.Fatalf("Location=%q", location)
	}
	if strings.Contains(response.Body.String(), "updated_at") {
		t.Fatalf("internal UpdatedAt leaked to DTO: %s", response.Body.String())
	}
}

func TestUnexpectedErrorReturnsNeutral500(t *testing.T) {
	useCase := &fakeUseCase{
		createFn: func(context.Context, int64, task.CreateInput) (task.Task, error) {
			return task.Task{}, errors.New("password=secret schema=private")
		},
	}
	request := httptest.NewRequest(
		http.MethodPost,
		"/tasks",
		strings.NewReader(`{"title":"Task"}`),
	)
	response := httptest.NewRecorder()

	newRouter(useCase).ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want 500", response.Code)
	}
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatalf("internal error leaked: %s", response.Body.String())
	}
}
