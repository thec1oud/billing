package testutil

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/testcontainers/testcontainers-go"
	tcpg "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcrabbit "github.com/testcontainers/testcontainers-go/modules/rabbitmq"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestCluster struct {
	PGContainer     *tcpg.PostgresContainer
	RabbitContainer *tcrabbit.RabbitMQContainer
	DBPool          *pgxpool.Pool
	RabbitConn      *amqp.Connection
}

func SetupTestCluster(ctx context.Context) (*TestCluster, func(), error) {
	// 1. Spin up Postgres 16 container
	pgContainer, err := tcpg.Run(ctx,
		"postgres:16-alpine",
		tcpg.WithDatabase("billing_test"),
		tcpg.WithUsername("postgres"),
		tcpg.WithPassword("postgres"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").WithOccurrence(2).WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	pgConnStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to get pg connection string: %w", err)
	}

	// 2. Run Database Migrations on test postgres container
	_, b, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(b), "../../..")
	migrationsPath := filepath.Join(projectRoot, "internal/database/migrations")

	m, err := migrate.New("file://"+migrationsPath, pgConnStr)
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to init migrations: %w", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		_ = pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to apply migrations: %w", err)
	}

	dbPool, err := pgxpool.New(ctx, pgConnStr)
	if err != nil {
		_ = pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to create pgxpool: %w", err)
	}

	// 3. Spin up RabbitMQ container
	rabbitContainer, err := tcrabbit.Run(ctx,
		"rabbitmq:3-management-alpine",
		testcontainers.WithWaitStrategy(
			wait.ForLog("Server startup complete").WithStartupTimeout(40*time.Second),
		),
	)
	if err != nil {
		dbPool.Close()
		_ = pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to start rabbitmq container: %w", err)
	}

	rabbitAmqpURL, err := rabbitContainer.AmqpURL(ctx)
	if err != nil {
		dbPool.Close()
		_ = rabbitContainer.Terminate(ctx)
		_ = pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to get rabbitmq amqp url: %w", err)
	}

	rabbitConn, err := amqp.Dial(rabbitAmqpURL)
	if err != nil {
		dbPool.Close()
		_ = rabbitContainer.Terminate(ctx)
		_ = pgContainer.Terminate(ctx)
		return nil, nil, fmt.Errorf("failed to dial rabbitmq: %w", err)
	}

	cluster := &TestCluster{
		PGContainer:     pgContainer,
		RabbitContainer: rabbitContainer,
		DBPool:          dbPool,
		RabbitConn:      rabbitConn,
	}

	cleanup := func() {
		if rabbitConn != nil {
			_ = rabbitConn.Close()
		}
		if dbPool != nil {
			dbPool.Close()
		}
		_ = rabbitContainer.Terminate(context.Background())
		_ = pgContainer.Terminate(context.Background())
	}

	return cluster, cleanup, nil
}
