package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/rinatkh/classroom18/internal/auth"
	"github.com/rinatkh/classroom18/internal/httpapi"
	"github.com/rinatkh/classroom18/internal/httpapi/handler"
	"github.com/rinatkh/classroom18/internal/httpapi/middleware"
	"github.com/rinatkh/classroom18/internal/repository/memory"
	"github.com/rinatkh/classroom18/internal/task"
	"github.com/rinatkh/classroom18/internal/user"
)

func newRouter() http.Handler {
	users := memory.NewUserRepository()
	tasks := memory.NewTaskRepository()
	tokens := auth.NewJWTManager("0123456789abcdef0123456789abcdef", time.Hour)
	authUseCase := user.NewUseCase(users, auth.NewBcryptHasher(bcrypt.MinCost), tokens, memory.NewRefreshSessionRepository(), auth.NewCryptoRefreshTokenGenerator(), user.SessionConfig{AccessTTL: time.Hour, RefreshTTL: 7 * 24 * time.Hour})
	taskUseCase := task.NewUseCase(tasks)
	return httpapi.NewRouter(
		handler.NewAuthHandler(authUseCase, handler.CookieConfig{Name: "refresh_token"}),
		handler.NewTaskHandler(taskUseCase),
		middleware.NewAuth(tokens),
	)
}

func TestProtectedRoutesAndOwnership(t *testing.T) {
	router := newRouter()

	withoutToken := request(t, router, http.MethodGet, "/tasks", "", "")
	assertStatus(t, withoutToken, http.StatusUnauthorized)

	alice := registerAndLogin(t, router, "alice@example.com")
	bob := registerAndLogin(t, router, "bob@example.com")

	created := request(t, router, http.MethodPost, "/tasks", alice, `{"title":"private"}`)
	assertStatus(t, created, http.StatusCreated)
	if got := created.Header().Get("Location"); got != "/tasks/1" {
		t.Fatalf("Location = %q", got)
	}

	foreign := request(t, router, http.MethodGet, "/tasks/1", bob, "")
	assertStatus(t, foreign, http.StatusForbidden)
	for _, method := range []string{http.MethodPatch, http.MethodDelete} {
		denied := request(t, router, method, "/tasks/1", bob, `{"done":true}`)
		assertStatus(t, denied, http.StatusForbidden)
	}
	own := request(t, router, http.MethodGet, "/tasks/1", alice, "")
	assertStatus(t, own, http.StatusOK)
	patched := request(t, router, http.MethodPatch, "/tasks/1", alice, `{"done":true}`)
	assertStatus(t, patched, http.StatusOK)
	var changed struct {
		Title string
		Done  bool
	}
	if err := json.Unmarshal(patched.Body.Bytes(), &changed); err != nil || changed.Title != "private" || !changed.Done {
		t.Fatalf("PATCH=%s, %v", patched.Body.String(), err)
	}

	aliceList := request(t, router, http.MethodGet, "/tasks", alice, "")
	assertStatus(t, aliceList, http.StatusOK)
	var aliceTasks []map[string]any
	if err := json.Unmarshal(aliceList.Body.Bytes(), &aliceTasks); err != nil {
		t.Fatalf("decode Alice list: %v", err)
	}
	if len(aliceTasks) != 1 {
		t.Fatalf("Alice list length = %d", len(aliceTasks))
	}

	bobList := request(t, router, http.MethodGet, "/tasks", bob, "")
	assertStatus(t, bobList, http.StatusOK)
	var bobTasks []map[string]any
	if err := json.Unmarshal(bobList.Body.Bytes(), &bobTasks); err != nil {
		t.Fatalf("decode Bob list: %v", err)
	}
	if len(bobTasks) != 0 {
		t.Fatalf("Bob can see foreign tasks: %s", bobList.Body.String())
	}
}

func registerAndLogin(t *testing.T, router http.Handler, email string) string {
	t.Helper()
	body := fmt.Sprintf(`{"email":%q,"password":"secret123"}`, email)
	created := request(t, router, http.MethodPost, "/register", "", body)
	assertStatus(t, created, http.StatusCreated)
	loggedIn := request(t, router, http.MethodPost, "/login", "", body)
	assertStatus(t, loggedIn, http.StatusOK)
	var response struct {
		Token string `json:"access_token"`
	}
	if err := json.Unmarshal(loggedIn.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if response.Token == "" {
		t.Fatal("empty token")
	}
	return response.Token
}

func request(t *testing.T, router http.Handler, method, target, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, target, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, req)
	return response
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()
	if response.Code != want {
		t.Fatalf("status = %d, want %d, body = %s", response.Code, want, response.Body.String())
	}
}
