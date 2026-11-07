// Package httpapi собирает public и protected HTTP routes.
package httpapi

import (
	"net/http"

	"github.com/rinatkh/classroom18/internal/httpapi/handler"
	"github.com/rinatkh/classroom18/internal/httpapi/middleware"
)

func NewRouter(
	authHandler *handler.AuthHandler,
	taskHandler *handler.TaskHandler,
	authMiddleware *middleware.Auth,
) http.Handler {
	mux := http.NewServeMux()

	// Public endpoints: token ещё нет.
	mux.HandleFunc("GET /health", taskHandler.Health)
	mux.HandleFunc("POST /register", authHandler.Register)
	mux.HandleFunc("POST /login", authHandler.Login)
	mux.HandleFunc("POST /auth/refresh", authHandler.Refresh)
	mux.HandleFunc("POST /auth/logout", authHandler.Logout)

	// Protected endpoints: одна и та же authentication-проверка оборачивает
	// каждый handler, но ownership всё равно проверяет Task UseCase.
	protect := authMiddleware.RequireUser
	mux.Handle("GET /tasks", protect(http.HandlerFunc(taskHandler.List)))
	mux.Handle("GET /tasks/{id}", protect(http.HandlerFunc(taskHandler.Get)))
	mux.Handle("POST /tasks", protect(http.HandlerFunc(taskHandler.Create)))
	mux.Handle("PATCH /tasks/{id}", protect(http.HandlerFunc(taskHandler.Update)))
	mux.Handle("DELETE /tasks/{id}", protect(http.HandlerFunc(taskHandler.Delete)))

	return mux
}
