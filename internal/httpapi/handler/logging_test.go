package handler

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rinatkh/classroom18/internal/logging"
	"github.com/rinatkh/classroom18/internal/user"
)

type failingLogWriter struct {
	header http.Header
	status int
}

func TestAuthLogsDoNotContainCredentials(t *testing.T) {
	var output bytes.Buffer
	logger, err := logging.New(&output, "auth-test", "DEBUG", "json")
	if err != nil {
		t.Fatal(err)
	}
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })
	fake := &sessionFakeUseCase{result: user.LoginResult{
		AccessToken: "private-access-jwt", RefreshToken: "private-refresh-raw",
		RefreshExpiresAt: time.Now().Add(time.Hour), ExpiresIn: 900,
	}}
	h := NewAuthHandler(fake, CookieConfig{Name: "refresh_token"})
	for _, fail := range []bool{false, true} {
		fake.err = nil
		if fail {
			fake.err = errors.New("private-password-hash SQL detail")
		}
		for _, operation := range []struct {
			path    string
			handler http.HandlerFunc
		}{{"/login", h.Login}, {"/auth/refresh", h.Refresh}, {"/auth/logout", h.Logout}} {
			req := httptest.NewRequest("POST", operation.path, strings.NewReader(`{"email":"private-email@example.test","password":"private-password"}`))
			req.Header.Set("Authorization", "Bearer private-header-jwt")
			req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "private-refresh-raw"})
			operation.handler(httptest.NewRecorder(), req)
		}
	}
	got := output.String()
	if !strings.Contains(got, `"level":"DEBUG"`) || !strings.Contains(got, `"level":"INFO"`) || !strings.Contains(got, `"level":"ERROR"`) {
		t.Fatal("missing auth log levels")
	}
	if strings.Contains(got, "private-") {
		t.Fatal("credentials or request payload leaked to auth logs")
	}
}

func (w *failingLogWriter) Header() http.Header    { return w.header }
func (w *failingLogWriter) WriteHeader(status int) { w.status = status }
func (w *failingLogWriter) Write([]byte) (int, error) {
	return 0, errors.New("private-token-in-writer-error")
}

func TestResponseFailureLogsWithoutLeakingData(t *testing.T) {
	var output bytes.Buffer
	logger, err := logging.New(&output, "http-test", "DEBUG", "json")
	if err != nil {
		t.Fatal(err)
	}
	previous := slog.Default()
	slog.SetDefault(logger)
	t.Cleanup(func() { slog.SetDefault(previous) })
	writer := &failingLogWriter{header: make(http.Header)}
	writeJSON(writer, http.StatusOK, map[string]string{"title": "private-body-value"})
	log := output.String()
	if !strings.Contains(log, `"level":"ERROR"`) || strings.Contains(log, "private-") {
		t.Fatalf("unsafe/missing log: %s", log)
	}
	if writer.status != http.StatusOK {
		t.Fatalf("response already started: status=%d", writer.status)
	}
}
