// Package middleware содержит HTTP boundary для authentication.
package middleware

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/rinatkh/classroom18/internal/httpapi/authctx"
	"github.com/rinatkh/classroom18/internal/logging"
)

type TokenVerifier interface {
	Verify(token string) (int64, error)
}

type Auth struct {
	verifier TokenVerifier
}

func NewAuth(verifier TokenVerifier) *Auth {
	return &Auth{verifier: verifier}
}

// RequireUser аутентифицирует запрос один раз и передаёт identity дальше через
// request context. Ownership конкретной Task остаётся ответственностью UseCase.
func (m *Auth) RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := bearerToken(r.Header.Get("Authorization"))
		if !ok {
			slog.DebugContext(r.Context(), "authentication rejected", "reason", "missing or malformed bearer")
			unauthorized(w)
			return
		}

		userID, err := m.verifier.Verify(raw)
		if err != nil || userID <= 0 {
			slog.DebugContext(r.Context(), "authentication rejected", "reason", "invalid access token")
			unauthorized(w)
			return
		}

		slog.DebugContext(r.Context(), "request authenticated", "user_id", userID)
		ctx := authctx.WithUserID(r.Context(), userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	if err := json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"}); err != nil {
		logging.Error(context.Background(), "HTTP auth response write failed", err)
	}
}
