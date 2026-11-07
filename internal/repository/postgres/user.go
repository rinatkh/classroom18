package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/rinatkh/classroom18/internal/user"
)

type UserRepository struct {
	db DBTX
}

var _ user.Repository = (*UserRepository)(nil)

func NewUserRepository(db DBTX) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, value user.User) (user.User, error) {
	slog.DebugContext(ctx, "repository operation", "adapter", "postgres", "repository", "UserRepository", "operation", "Create")
	const query = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, email, password_hash`
	var result user.User
	err := r.db.QueryRow(ctx, query, strings.ToLower(value.Email), value.PasswordHash).
		Scan(&result.ID, &result.Email, &result.PasswordHash)
	var pgError *pgconn.PgError
	if errors.As(err, &pgError) && pgError.Code == "23505" && pgError.ConstraintName == "users_email_key" {
		// Возвращаем существующую ошибку, без SQL/constraint details в её тексте.
		// Constraint — последняя защита от двух одновременных Register.
		return user.User{}, user.ErrAlreadyExists
	}
	if err != nil {
		return user.User{}, fmt.Errorf("insert user: %w", err)
	}
	return result, nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (user.User, error) {
	slog.DebugContext(ctx, "repository operation", "adapter", "postgres", "repository", "UserRepository", "operation", "GetByEmail")
	const query = `SELECT id, email, password_hash FROM users WHERE email = $1`
	var result user.User
	// Как и memory key, поиск использует lower-case email; UseCase уже выполняет
	// trim/normalization. Extension для email в первом PostgreSQL-уроке не нужен.
	err := r.db.QueryRow(ctx, query, strings.ToLower(email)).
		Scan(&result.ID, &result.Email, &result.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return user.User{}, user.ErrNotFound
	}
	if err != nil {
		return user.User{}, fmt.Errorf("select user by email: %w", err)
	}
	return result, nil
}
