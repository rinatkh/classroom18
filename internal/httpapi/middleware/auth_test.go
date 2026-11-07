package middleware

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rinatkh/classroom18/internal/httpapi/authctx"
)

type fakeVerifier struct {
	userID int64
	err    error
}

func (v fakeVerifier) Verify(string) (int64, error) { return v.userID, v.err }

func TestAuthMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		verifier   fakeVerifier
		wantStatus int
		wantNext   bool
	}{
		{name: "missing header", wantStatus: http.StatusUnauthorized},
		{name: "empty Bearer", header: "Bearer", wantStatus: http.StatusUnauthorized},
		{name: "extra fields", header: "Bearer one two", wantStatus: http.StatusUnauthorized},
		{name: "zero identity", header: "Bearer token", wantStatus: http.StatusUnauthorized},
		{name: "wrong scheme", header: "Basic abc", wantStatus: http.StatusUnauthorized},
		{name: "invalid token", header: "Bearer bad", verifier: fakeVerifier{err: errors.New("invalid")}, wantStatus: http.StatusUnauthorized},
		{name: "valid token", header: "Bearer good", verifier: fakeVerifier{userID: 42}, wantStatus: http.StatusNoContent, wantNext: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextCalled := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				userID, ok := authctx.UserID(r.Context())
				if !ok || userID != 42 {
					t.Fatalf("context userID = %d, %v", userID, ok)
				}
				w.WriteHeader(http.StatusNoContent)
			})
			request := httptest.NewRequest(http.MethodGet, "/tasks", nil)
			request.Header.Set("Authorization", tt.header)
			response := httptest.NewRecorder()
			NewAuth(tt.verifier).RequireUser(next).ServeHTTP(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if response.Code == http.StatusUnauthorized && response.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Fatal("missing Bearer challenge")
			}
			if nextCalled != tt.wantNext {
				t.Fatalf("nextCalled = %v, want %v", nextCalled, tt.wantNext)
			}
		})
	}
}
