package database

import (
	"embed"
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	migratorPgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func RunMigrations(conn *pgx.Conn) error {
	sqlDB := stdlib.OpenDB(*conn.Config())
	defer func() {
		if err := sqlDB.Close(); err != nil {
			slog.Error("failed to close sql.DB", "error", err)
		}
	}()

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source driver: %w", err)
	}

	dbDriver, err := migratorPgx.WithInstance(sqlDB, &migratorPgx.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration database driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "pgx5", dbDriver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrator: %w", err)
	}
	defer func() {
		if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
			slog.Error("failed to close migrator instance", "src_err", srcErr, "db_err", dbErr)
		}
	}()

	slog.Info("Applying database migrations...")
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("Database schema up to date. No changes applied.")
			return nil
		}
		return fmt.Errorf("failed to execute up migrations: %w", err)
	}

	slog.Info("Database migrations applied successfully")
	return nil
}
