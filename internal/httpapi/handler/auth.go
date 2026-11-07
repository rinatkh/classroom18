package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/rinatkh/classroom18/internal/httpapi/dto"
	"github.com/rinatkh/classroom18/internal/httpapi/mapper"
	"github.com/rinatkh/classroom18/internal/logging"
	"github.com/rinatkh/classroom18/internal/user"
)

type AuthUseCase interface {
	Register(ctx context.Context, input user.RegisterInput) (user.User, error)
	Login(ctx context.Context, input user.LoginInput) (user.LoginResult, error)
	Refresh(ctx context.Context, raw string) (user.LoginResult, error)
	Logout(ctx context.Context, raw string) error
}

type AuthHandler struct {
	useCase AuthUseCase
	cookie  CookieConfig
}

func NewAuthHandler(useCase AuthUseCase, cookie CookieConfig) *AuthHandler {
	return &AuthHandler{useCase: useCase, cookie: cookie}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "HTTP request", "operation", "auth.register")
	var request dto.RegisterRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	result, err := h.useCase.Register(r.Context(), user.RegisterInput{
		Email: request.Email, Password: request.Password,
	})
	switch {
	case errors.Is(err, user.ErrInvalidEmail), errors.Is(err, user.ErrWeakPassword):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, user.ErrAlreadyExists):
		writeError(w, http.StatusConflict, err.Error())
		return
	case err != nil:
		logging.Error(r.Context(), "registration failed", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	slog.InfoContext(r.Context(), "user registered", "user_id", result.ID)
	writeJSON(w, http.StatusCreated, mapper.UserToResponse(result))
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "HTTP request", "operation", "auth.login")
	noStore(w)
	var request dto.LoginRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	result, err := h.useCase.Login(r.Context(), user.LoginInput{
		Email: request.Email, Password: request.Password,
	})
	if errors.Is(err, user.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}
	if err != nil {
		logging.Error(r.Context(), "login failed", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	slog.InfoContext(r.Context(), "auth credentials issued", "operation", "login")
	h.writeTokens(w, result)
}

// Refresh публичен относительно access JWT: истёкший access не должен блокировать
// восстановление. Единственный credential здесь — refresh cookie.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "HTTP request", "operation", "auth.refresh")
	noStore(w)
	cookie, err := r.Cookie(h.cookie.Name)
	if err != nil || cookie.Value == "" {
		refreshUnauthorized(w)
		return
	}
	result, err := h.useCase.Refresh(r.Context(), cookie.Value)
	switch {
	case errors.Is(err, user.ErrInvalidRefreshToken), errors.Is(err, user.ErrRefreshExpired), errors.Is(err, user.ErrRefreshRevoked):
		refreshUnauthorized(w)
	case err != nil:
		logging.Error(r.Context(), "refresh failed", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	default:
		slog.InfoContext(r.Context(), "refresh session rotated")
		h.writeTokens(w, result)
	}
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	slog.DebugContext(r.Context(), "HTTP request", "operation", "auth.logout")
	noStore(w)
	raw := ""
	if cookie, err := r.Cookie(h.cookie.Name); err == nil {
		raw = cookie.Value
	}
	if err := h.useCase.Logout(r.Context(), raw); err != nil {
		logging.Error(r.Context(), "logout failed", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	h.clearRefreshCookie(w)
	slog.InfoContext(r.Context(), "refresh session revoked")
	w.WriteHeader(http.StatusNoContent)
}

func (h *AuthHandler) writeTokens(w http.ResponseWriter, result user.LoginResult) {
	h.setRefreshCookie(w, result.RefreshToken, result.RefreshExpiresAt)
	writeJSON(w, http.StatusOK, dto.TokenResponse{AccessToken: result.AccessToken,
		TokenType: "Bearer", ExpiresIn: result.ExpiresIn})
}

func refreshUnauthorized(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "invalid refresh token")
}

func noStore(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
}
