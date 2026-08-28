package testutil

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/thec1oud/billing/internal/database"
)

// defaultPostgresImage matches docker-compose.yaml's app-db image. Override via
// the TEST_POSTGRES_IMAGE env var in environments where that tag isn't already
// pulled and the registry isn't reachable (e.g. an offline/sandboxed CI runner).
const defaultPostgresImage = "postgres:16-alpine"

func postgresImage() string {
	if img := os.Getenv("TEST_POSTGRES_IMAGE"); img != "" {
		return img
	}
	return defaultPostgresImage
}

// NewPostgresContainer starts an ephemeral postgres container (see postgresImage),
// applies the embedded schema migrations to it via database.RunMigrations, and
// returns a connected pgxpool.Pool. The container and pool are terminated/closed
// automatically via t.Cleanup. Unlike GetTestPool, this requires no pre-running dev
// database and no .env — only a reachable Docker daemon.
func NewPostgresContainer(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	const (
		dbName = "billing_test"
		dbUser = "test"
		dbPass = "test"
	)

	pgContainer, err := postgres.Run(ctx,
		postgresImage(),
		postgres.WithDatabase(dbName),
		postgres.WithUsername(dbUser),
		postgres.WithPassword(dbPass),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := pgContainer.Terminate(context.Background()); err != nil {
			t.Logf("terminate postgres container: %v", err)
		}
	})

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("get postgres container connection string: %v", err)
	}

	migrateConn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test container for migrations: %v", err)
	}
	defer migrateConn.Close(ctx)

	if err := database.RunMigrations(migrateConn); err != nil {
		t.Fatalf("run migrations against test container: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("open test pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test pool: %v", err)
	}

	return pool
}
