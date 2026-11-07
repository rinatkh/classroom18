// Package postgres реализует прежние user.Repository и task.Repository через SQL.
// HTTP и business rules остаются в тех же слоях, что в занятии 13.
package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DBTX реализуют *pgxpool.Pool и pgx.Tx. Это инфраструктурный контракт,
// он не выходит в task/user и не меняет их Repository interfaces.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
