package postgres

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rinatkh/classroom18/internal/user"
)

// SessionDB добавляет BeginTx только для инфраструктуры refresh sessions.
// User/Task repositories и их application contracts остались прежними.
type SessionDB interface {
	DBTX
	BeginTx(ctx context.Context, options pgx.TxOptions) (pgx.Tx, error)
}

type RefreshSessionRepository struct{ db SessionDB }

var _ user.RefreshSessionRepository = (*RefreshSessionRepository)(nil)

func NewRefreshSessionRepository(db SessionDB) *RefreshSessionRepository {
	return &RefreshSessionRepository{db: db}
}

const sessionColumns = `id, user_id, token_hash, expires_at, created_at, revoked_at, replaced_by_session_id`

func scanSession(row pgx.Row) (user.RefreshSession, error) {
	var s user.RefreshSession
	err := row.Scan(&s.ID, &s.UserID, &s.TokenHash, &s.ExpiresAt, &s.CreatedAt, &s.RevokedAt, &s.ReplacedBySessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return user.RefreshSession{}, user.ErrInvalidRefreshToken
	}
	return s, err
}

func insertSession(ctx context.Context, db DBTX, s user.RefreshSession) (user.RefreshSession, error) {
	result, err := scanSession(db.QueryRow(ctx, `INSERT INTO refresh_sessions (user_id, token_hash, expires_at)
        VALUES ($1, $2, $3) RETURNING `+sessionColumns, s.UserID, s.TokenHash, s.ExpiresAt))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "refresh_sessions_token_hash_key" {
		return user.RefreshSession{}, user.ErrSessionAlreadyExists
	}
	return result, err
}

func (r *RefreshSessionRepository) Create(ctx context.Context, s user.RefreshSession) (user.RefreshSession, error) {
	slog.DebugContext(ctx, "repository operation", "adapter", "postgres", "repository", "RefreshSessionRepository", "operation", "Create")
	return insertSession(ctx, r.db, s)
}

func (r *RefreshSessionRepository) GetByTokenHash(ctx context.Context, hash string) (user.RefreshSession, error) {
	slog.DebugContext(ctx, "repository operation", "adapter", "postgres", "repository", "RefreshSessionRepository", "operation", "GetByTokenHash")
	return scanSession(r.db.QueryRow(ctx, `SELECT `+sessionColumns+` FROM refresh_sessions WHERE token_hash=$1`, hash))
}

func (r *RefreshSessionRepository) Rotate(ctx context.Context, hash string, next user.RefreshSession) error {
	slog.DebugContext(ctx, "repository operation", "adapter", "postgres", "repository", "RefreshSessionRepository", "operation", "Rotate")
	// Read committed: ожидающий запрос увидит актуальную версию строки после commit.
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) // после успешного Commit это безопасный no-op

	old, err := scanSession(tx.QueryRow(ctx, `SELECT `+sessionColumns+`
        FROM refresh_sessions WHERE token_hash=$1 FOR UPDATE`, hash))
	if err != nil {
		return err
	}
	if old.RevokedAt != nil {
		return user.ErrRefreshRevoked
	}
	// Проверяем wall clock ПОСЛЕ ожидания row lock. now() в PostgreSQL фиксирован
	// в начале transaction и мог устареть, пока запрос ждал блокировку.
	var now time.Time
	if err := tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return err
	}
	if !old.ExpiresAt.After(now) {
		return user.ErrRefreshExpired
	}
	if old.UserID != next.UserID || !next.ExpiresAt.After(now) {
		return user.ErrInvalidRefreshToken
	}

	if _, err := tx.Exec(ctx, `UPDATE refresh_sessions SET revoked_at=$1 WHERE id=$2`, now, old.ID); err != nil {
		return err
	}
	created, err := insertSession(ctx, tx, next)
	if err != nil {
		return err
	} // rollback вернёт старую session в active
	if _, err := tx.Exec(ctx, `UPDATE refresh_sessions SET replaced_by_session_id=$1 WHERE id=$2`, created.ID, old.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *RefreshSessionRepository) Revoke(ctx context.Context, hash string) error {
	slog.DebugContext(ctx, "repository operation", "adapter", "postgres", "repository", "RefreshSessionRepository", "operation", "Revoke")
	_, err := r.db.Exec(ctx, `UPDATE refresh_sessions SET revoked_at=clock_timestamp()
        WHERE token_hash=$1 AND revoked_at IS NULL`, hash)
	return err
}
