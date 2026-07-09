package storage

import (
	"fmt"

	"github.com/rabbitmq/amqp091-go"
	"github.com/thec1oud/billing/internal/config"
)

func NewRabbitMQConnection(cfg *config.Config) (*amqp091.Connection, error) {
	uri := fmt.Sprintf("amqp://%s:%s@%s:%s/",
		cfg.RabbitMQUser, cfg.RabbitMQPass, cfg.RabbitMQHost, cfg.RabbitMQPort)

	conn, err := amqp091.Dial(uri)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq connection failed: %w", err)
	}

	return conn, nil
}
