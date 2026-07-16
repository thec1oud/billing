package infra

import (
	"context"
	"log/slog"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"

	"github.com/thec1oud/billing/internal/config"
	"github.com/thec1oud/billing/internal/infra/storage"
)

type Dependencies struct {
	DB     *pgx.Conn
	Redis  *redis.Client
	Rabbit *amqp091.Connection
}

func InitDependencies(ctx context.Context, cfg *config.Config) *Dependencies {
	// PostgreSQL
	slog.Info("Attempting to connect to PostgreSQL...", "host", cfg.DBHost, "port", cfg.DBPort)
	dbConn, err := storage.NewPostgresConnection(ctx, cfg)
	if err != nil {
		slog.Error("Fatal database connection error", "error", err)
		os.Exit(1)
	}
	slog.Info("Successfully connected to PostgreSQL!")

	// Redis
	slog.Info("Attempting to connect to Redis...", "host", cfg.RedisHost, "port", cfg.RedisPort)
	redisClient, err := storage.NewRedisClient(ctx, cfg)
	if err != nil {
		slog.Error("Fatal Redis connection error", "error", err)
		os.Exit(1)
	}
	slog.Info("Successfully connected to Redis!")

	// RabbitMQ
	slog.Info("Attempting to connect to RabbitMQ...", "host", cfg.RabbitMQHost, "port", cfg.RabbitMQPort)
	rabbitConn, err := storage.NewRabbitMQConnection(cfg)
	if err != nil {
		slog.Error("Fatal RabbitMQ connection error", "error", err)
		os.Exit(1)
	}
	slog.Info("Successfully connected to RabbitMQ!")

	return &Dependencies{
		DB:     dbConn,
		Redis:  redisClient,
		Rabbit: rabbitConn,
	}
}
