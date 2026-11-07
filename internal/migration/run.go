// Package migration использует тот же goose Provider, что и cmd/migrate.
package migration

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"os"
)

// Run нужен инфраструктурным тестам. Путь migrations задаётся от корня проекта.
// SQL и история версий принадлежат goose, собственный журнал здесь не создаётся.
func Run(ctx context.Context, pool *pgxpool.Pool, direction string) error {
	db := stdlib.OpenDB(*pool.Config().ConnConfig)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("../../migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return err
	}
	switch direction {
	case "up":
		_, err = provider.Up(ctx)
	case "down":
		_, err = provider.Down(ctx)
	case "status":
		_, err = provider.Status(ctx)
	default:
		return fmt.Errorf("direction must be up, down or status")
	}
	return err
}
