package handler

import (
	"context"
	"errors"
	"github.com/rinatkh/classroom18/internal/user"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type sessionFakeUseCase struct {
	err        error
	gotContext context.Context
	result     user.LoginResult
}

func (f *sessionFakeUseCase) Register(ctx context.Context, input user.RegisterInput) (user.User, error) {
	return user.User{}, f.err
}
func (f *sessionFakeUseCase) Login(ctx context.Context, input user.LoginInput) (user.LoginResult, error) {
	f.gotContext = ctx
	return f.result, f.err
}
func (f *sessionFakeUseCase) Refresh(ctx context.Context, raw string) (user.LoginResult, error) {
	f.gotContext = ctx
	return f.result, f.err
}
func (f *sessionFakeUseCase) Logout(ctx context.Context, raw string) error {
	f.gotContext = ctx
	return f.err
}

func TestSessionHandlerErrorsAndContext(t *testing.T) {
	for _, cause := range []error{user.ErrInvalidRefreshToken, user.ErrRefreshExpired, user.ErrRefreshRevoked, errors.New("database password_hash detail")} {
		f := &sessionFakeUseCase{err: cause}
		h := NewAuthHandler(f, CookieConfig{Name: "refresh_token"})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		r := httptest.NewRequest("POST", "/auth/refresh", nil).WithContext(ctx)
		r.AddCookie(&http.Cookie{Name: "refresh_token", Value: "raw-secret"})
		w := httptest.NewRecorder()
		h.Refresh(w, r)
		want := 401
		if strings.Contains(cause.Error(), "database") {
			want = 500
		}
		if w.Code != want || f.gotContext != ctx || strings.Contains(w.Body.String(), "detail") || strings.Contains(w.Body.String(), "raw-secret") || w.Header().Get("Cache-Control") != "no-store" || len(w.Result().Cookies()) != 0 {
			t.Fatal("refresh error/context mapping")
		}
		if want == 401 && strings.TrimSpace(w.Body.String()) != `{"error":"invalid refresh token"}` {
			t.Fatal("internal refresh reason leaked")
		}
	}
	f := &sessionFakeUseCase{err: errors.New("database error")}
	h := NewAuthHandler(f, CookieConfig{Name: "refresh_token"})
	r := httptest.NewRequest("POST", "/auth/logout", nil)
	r.AddCookie(&http.Cookie{Name: "refresh_token", Value: "raw-secret"})
	w := httptest.NewRecorder()
	h.Logout(w, r)
	if w.Code != 500 || len(w.Result().Cookies()) != 0 {
		t.Fatal("failed revoke must not pretend successful logout")
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest("POST", "/login", strings.NewReader(`{"email":"a@example.test","password":"secret123"}`))
	h.Login(w, r)
	if w.Code != 500 || len(w.Result().Cookies()) != 0 || strings.Contains(w.Body.String(), "database") {
		t.Fatal("failed login must not set cookie or leak error")
	}
}
func TestCookieConfigAndClear(t *testing.T) {
	for _, c := range []CookieConfig{{Name: ""}, {Name: "bad name"}, {Name: "__Host-refresh", Secure: true}, {Name: "__Secure-refresh", Secure: false}} {
		if c.Validate() == nil {
			t.Fatal("invalid cookie config accepted")
		}
	}
	c := CookieConfig{Name: "custom_refresh", Secure: true}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(nil, c)
	w := httptest.NewRecorder()
	h.setRefreshCookie(w, "raw", time.Now().Add(time.Hour))
	got := w.Result().Cookies()[0]
	if got.Name != c.Name || !got.Secure || !got.HttpOnly || got.Path != "/auth" {
		t.Fatal("custom cookie config")
	}
}
