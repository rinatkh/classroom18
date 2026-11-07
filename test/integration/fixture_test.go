package integration

import (
	"encoding/json"
	"net/http"
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

// Прежние Task API tests выполняются для аутентифицированного владельца.
func newAuthenticatedRouter(t *testing.T) http.Handler {
	t.Helper()
	tokens := auth.NewJWTManager("0123456789abcdef0123456789abcdef", time.Hour)
	app := httpapi.NewRouter(handler.NewAuthHandler(user.NewUseCase(memory.NewUserRepository(), auth.NewBcryptHasher(bcrypt.MinCost), tokens, memory.NewRefreshSessionRepository(), auth.NewCryptoRefreshTokenGenerator(), user.SessionConfig{AccessTTL: time.Hour, RefreshTTL: 7 * 24 * time.Hour}), handler.CookieConfig{Name: "refresh_token"}), handler.NewTaskHandler(task.NewUseCase(memory.NewTaskRepository())), middleware.NewAuth(tokens))
	body := `{"email":"alice@example.com","password":"secret123"}`
	created := request(t, app, http.MethodPost, "/register", body)
	assertCode(t, created, 201)
	login := request(t, app, http.MethodPost, "/login", body)
	assertCode(t, login, 200)
	var result struct {
		Token string `json:"access_token"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &result); err != nil || result.Token == "" {
		t.Fatalf("login=%s, %v", login.Body.String(), err)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", "Bearer "+result.Token)
		app.ServeHTTP(w, r)
	})
}
