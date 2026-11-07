//go:build integration

package postgres

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rinatkh/classroom18/internal/auth"
	"github.com/rinatkh/classroom18/internal/httpapi"
	"github.com/rinatkh/classroom18/internal/httpapi/handler"
	"github.com/rinatkh/classroom18/internal/httpapi/middleware"
	"github.com/rinatkh/classroom18/internal/task"

	"github.com/rinatkh/classroom18/internal/user"
	"golang.org/x/crypto/bcrypt"
)

func TestIntegrationApplicationAuthAndOwnership(t *testing.T) {
	pool, ctx := integrationPool(t)
	tokens := auth.NewJWTManager("0123456789abcdef0123456789abcdef", time.Hour)
	router := httpapi.NewRouter(handler.NewAuthHandler(user.NewUseCase(NewUserRepository(pool), auth.NewBcryptHasher(bcrypt.MinCost), tokens, NewRefreshSessionRepository(pool), auth.NewCryptoRefreshTokenGenerator(), user.SessionConfig{AccessTTL: time.Hour, RefreshTTL: 7 * 24 * time.Hour}), handler.CookieConfig{Name: "refresh_token"}), handler.NewTaskHandler(task.NewUseCase(NewTaskRepository(pool))), middleware.NewAuth(tokens))
	call := func(method, path, token, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body)).WithContext(ctx)
		if body != "" {
			r.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: status=%d body=%s", method, path, w.Code, w.Body.String())
		}
		return w
	}
	registerLogin := func(email string) string {
		t.Helper()
		body := fmt.Sprintf(`{"email":%q,"password":"secret123"}`, email)
		registered := call("POST", "/register", "", body, 201)
		if strings.Contains(strings.ToLower(registered.Body.String()), "hash") {
			t.Fatal("hash leaked")
		}
		logged := call("POST", "/login", "", body, 200)
		var result struct {
			Token string `json:"access_token"`
		}
		if err := json.Unmarshal(logged.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result.Token
	}
	unauth := call("GET", "/tasks", "", "", 401)
	if unauth.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Fatal("missing challenge")
	}
	alice := registerLogin("alice@example.test")
	bob := registerLogin("bob@example.test")
	duplicate := call("POST", "/register", "", `{"email":"ALICE@example.test","password":"secret123"}`, 409)
	if strings.TrimSpace(duplicate.Body.String()) != `{"error":"user already exists"}` {
		t.Fatalf("raw database error: %s", duplicate.Body.String())
	}
	created := call("POST", "/tasks", alice, `{"title":"Alice"}`, 201)
	if created.Header().Get("Location") != "/tasks/1" {
		t.Fatalf("Location=%q", created.Header().Get("Location"))
	}
	call("GET", "/tasks/1", alice, "", 200)
	for _, method := range []string{"GET", "PATCH", "DELETE"} {
		call(method, "/tasks/1", bob, `{"done":true}`, 403)
	}
	list := call("GET", "/tasks", bob, "", 200)
	if strings.TrimSpace(list.Body.String()) != "[]" {
		t.Fatalf("Bob list=%s", list.Body.String())
	}
	patched := call("PATCH", "/tasks/1", alice, `{"done":true}`, 200)
	if strings.TrimSpace(patched.Body.String()) != `{"id":1,"title":"Alice","done":true}` {
		t.Fatalf("PATCH/DTO changed: %s", patched.Body.String())
	}
	call("PATCH", "/tasks/1", alice, `{"done":true}`, 409)
	call("PATCH", "/tasks/1", alice, `{"done":false}`, 200)
	deleted := call("DELETE", "/tasks/1", alice, "", 204)
	if deleted.Body.Len() != 0 {
		t.Fatal("204 body")
	}
	call("GET", "/tasks/1", alice, "", 404)
	// Искусственно исчезнувший owner даёт FK error; существующий Handler скрывает SQL details.
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE email=$1`, "alice@example.test"); err != nil {
		t.Fatal(err)
	}
	failure := call("POST", "/tasks", alice, `{"title":"FK error"}`, 500)
	if strings.TrimSpace(failure.Body.String()) != `{"error":"внутренняя ошибка сервера"}` {
		t.Fatalf("database error leaked: %s", failure.Body.String())
	}
}
