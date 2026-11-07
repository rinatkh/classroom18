package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rinatkh/classroom18/internal/auth"
	"github.com/rinatkh/classroom18/internal/httpapi"
	"github.com/rinatkh/classroom18/internal/httpapi/handler"
	"github.com/rinatkh/classroom18/internal/httpapi/middleware"
	"github.com/rinatkh/classroom18/internal/repository/memory"
	"github.com/rinatkh/classroom18/internal/task"
	"github.com/rinatkh/classroom18/internal/user"
	"golang.org/x/crypto/bcrypt"
)

func sessionRouter(secure bool) http.Handler {
	tokens := auth.NewJWTManager("0123456789abcdef0123456789abcdef", 15*time.Minute)
	uc := user.NewUseCase(memory.NewUserRepository(), auth.NewBcryptHasher(bcrypt.MinCost), tokens, memory.NewRefreshSessionRepository(), auth.NewCryptoRefreshTokenGenerator(), user.SessionConfig{AccessTTL: 15 * time.Minute, RefreshTTL: 7 * 24 * time.Hour})
	return httpapi.NewRouter(handler.NewAuthHandler(uc, handler.CookieConfig{Name: "refresh_token", Secure: secure}), handler.NewTaskHandler(task.NewUseCase(memory.NewTaskRepository())), middleware.NewAuth(tokens))
}
func sessionCall(router http.Handler, path string, cookie *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	out := httptest.NewRecorder()
	router.ServeHTTP(out, req)
	return out
}
func tokenResponse(t *testing.T, w *httptest.ResponseRecorder, secure bool) (string, *http.Cookie) {
	t.Helper()
	assertStatus(t, w, 200)
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 3 || body["access_token"] == "" || body["token_type"] != "Bearer" || body["expires_in"] != float64(900) {
		t.Fatal("token response contract")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("expected refresh Set-Cookie")
	}
	c := cookies[0]
	if c.Name != "refresh_token" || c.Value == "" || !c.HttpOnly || c.Secure != secure || c.Path != "/auth" || c.Domain != "" || c.SameSite != http.SameSiteLaxMode || c.MaxAge <= 0 || !c.Expires.After(time.Now()) {
		t.Fatal("refresh cookie attributes")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("credentials must not be cached")
	}
	if strings.Contains(w.Body.String(), c.Value) {
		t.Fatal("raw refresh leaked in JSON")
	}
	return body["access_token"].(string), c
}
func TestSessionHTTPFlow(t *testing.T) {
	for _, secure := range []bool{false, true} {
		router := sessionRouter(secure)
		body := `{"email":"alice@example.test","password":"secret123"}`
		assertStatus(t, request(t, router, "POST", "/register", "", body), 201)
		login := request(t, router, "POST", "/login", "", body)
		access, first := tokenResponse(t, login, secure)
		assertStatus(t, request(t, router, "GET", "/tasks", access, ""), 200)
		// Refresh cookie не является credential для обычного protected endpoint.
		req := httptest.NewRequest("GET", "/tasks", nil)
		req.AddCookie(first)
		out := httptest.NewRecorder()
		router.ServeHTTP(out, req)
		assertStatus(t, out, 401)
		for _, c := range []*http.Cookie{nil, {Name: "refresh_token", Value: "unknown"}} {
			assertStatus(t, sessionCall(router, "/auth/refresh", c), 401)
		}
		refreshed := sessionCall(router, "/auth/refresh", first)
		newAccess, next := tokenResponse(t, refreshed, secure)
		if next.Value == first.Value {
			t.Fatal("refresh token did not rotate")
		}
		assertStatus(t, sessionCall(router, "/auth/refresh", first), 401)
		assertStatus(t, request(t, router, "GET", "/tasks", newAccess, ""), 200)
		logout := sessionCall(router, "/auth/logout", next)
		assertStatus(t, logout, 204)
		cleared := logout.Result().Cookies()
		if len(cleared) != 1 || cleared[0].Name != next.Name || cleared[0].Path != next.Path || cleared[0].Value != "" || cleared[0].MaxAge != -1 || !cleared[0].Expires.Before(time.Now()) || cleared[0].Secure != secure || cleared[0].SameSite != next.SameSite || !cleared[0].HttpOnly || logout.Body.Len() != 0 {
			t.Fatal("cookie was not cleared correctly")
		}
		assertStatus(t, sessionCall(router, "/auth/refresh", next), 401)
		assertStatus(t, sessionCall(router, "/auth/logout", nil), 204)
		assertStatus(t, sessionCall(router, "/auth/logout", next), 204)
		// Stateless access JWT остаётся рабочим после logout до своего exp.
		assertStatus(t, request(t, router, "GET", "/tasks", newAccess, ""), 200)
	}
}
