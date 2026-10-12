package infra

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/infra/logger"
	"github.com/thec1oud/billing/internal/infra/storage"
)


type Dependencies struct {
	DB     *pgx.Conn     // single connection, used for migrations
	Pool   *pgxpool.Pool // pooled connections, used by repositories that compose transactions
	Redis  *redis.Client
	Rabbit *amqp091.Connection
}

func InitDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
	log := logger.ForComponent("infra_container")
	log.Info("Attempting to connect to PostgreSQL...", slog.String("host", cfg.DBHost), slog.String("port", cfg.DBPort))
	dbConn, err := storage.NewPostgresConnection(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres connection failed: %w", err)
	}
	log.Info("Successfully connected to PostgreSQL!")

	dbPool, err := storage.NewPostgresPool(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("postgres pool connection failed: %w", err)
	}

	log.Info("Attempting to connect to Redis...", slog.String("host", cfg.RedisHost), slog.String("port", cfg.RedisPort))
	redisClient, err := storage.NewRedisClient(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}
	log.Info("Successfully connected to Redis!")

	log.Info("Attempting to connect to RabbitMQ...", slog.String("host", cfg.RabbitMQHost), slog.String("port", cfg.RabbitMQPort))
	rabbitConn, err := storage.NewRabbitMQConnection(cfg)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq connection failed: %w", err)
	}
	log.Info("Successfully connected to RabbitMQ!")

	return &Dependencies{
		DB:     dbConn,
		Pool:   dbPool,
		Redis:  redisClient,
		Rabbit: rabbitConn,
	}, nil
}
