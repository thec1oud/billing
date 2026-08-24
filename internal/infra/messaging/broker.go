package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/rabbitmq/amqp091-go"
)

const (
	DefaultExchangeName = "billing.events"
	DefaultDLXName      = "billing.dlx"
	DefaultDLQName      = "billing.dlq"
)

type Option func(*RabbitBroker)

func WithExchange(exchangeName, dlxName, dlqName string) Option {
	return func(b *RabbitBroker) {
		if exchangeName != "" {
			b.exchangeName = exchangeName
		}
		if dlxName != "" {
			b.dlxName = dlxName
		}
		if dlqName != "" {
			b.dlqName = dlqName
		}
	}
}

type Broker interface {
	InitTopology(ctx context.Context) error
	PublishEvent(ctx context.Context, routingKey string, body []byte) error
	RegisterConsumerGroup(ctx context.Context, queueName string, routingKeys []string, handler func(ctx context.Context, msg []byte) error) error
	Close() error
}

type RabbitBroker struct {
	conn         *amqp091.Connection
	exchangeName string
	dlxName      string
	dlqName      string

	pubMu sync.Mutex // Mutex ensuring thread-safe concurrent publishing calls

	wg     sync.WaitGroup     // WaitGroup tracking all consumer background goroutines
	ctx    context.Context    // Parent lifecycle context
	cancel context.CancelFunc // Cancellation trigger for graceful shutdown
}

func NewRabbitBroker(conn *amqp091.Connection, opts ...Option) (*RabbitBroker, error) {
	if conn == nil {
		return nil, fmt.Errorf("rabbitmq connection cannot be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())

	b := &RabbitBroker{
		conn:         conn,
		exchangeName: DefaultExchangeName,
		dlxName:      DefaultDLXName,
		dlqName:      DefaultDLQName,
		ctx:          ctx,
		cancel:       cancel,
	}

	for _, opt := range opts {
		opt(b)
	}

	return b, nil
}

func (b *RabbitBroker) InitTopology(ctx context.Context) error {
	ch, err := b.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open channel for topology init: %w", err)
	}
	defer ch.Close()

	// 1. Declare Topic Exchange for billing domain events
	err = ch.ExchangeDeclare(
		b.exchangeName, // name e.g. "billing.events"
		"topic",        // type
		true,           // durable
		false,          // auto-deleted
		false,          // internal
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare exchange %s: %w", b.exchangeName, err)
	}

	// 2. Declare Dead Letter Exchange (DLX)
	err = ch.ExchangeDeclare(
		b.dlxName,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare dlx %s: %w", b.dlxName, err)
	}

	// 3. Declare Dead Letter Queue (DLQ)
	_, err = ch.QueueDeclare(
		b.dlqName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to declare dlq %s: %w", b.dlqName, err)
	}

	// 4. Bind DLQ to DLX
	err = ch.QueueBind(
		b.dlqName,
		b.dlqName, // routing key matches queue name
		b.dlxName,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind dlq %s to dlx %s: %w", b.dlqName, b.dlxName, err)
	}

	slog.Info("RabbitMQ messaging topology initialized successfully",
		"exchange", b.exchangeName,
		"dlx", b.dlxName,
		"dlq", b.dlqName,
	)
	return nil
}

func (b *RabbitBroker) PublishEvent(ctx context.Context, routingKey string, body []byte) error {
	b.pubMu.Lock()
	defer b.pubMu.Unlock()

	ch, err := b.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open publish channel: %w", err)
	}
	defer ch.Close()

	// Enable Publisher Confirms to guarantee message durability before acknowledging success.
	// This puts the channel into confirm mode, meaning RabbitMQ will send an Ack or Nack
	// once it processes the published message.
	if err := ch.Confirm(false); err != nil {
		return fmt.Errorf("failed to put channel in confirm mode: %w", err)
	}

	// Create a channel to receive the confirmation from RabbitMQ.
	confirms := ch.NotifyPublish(make(chan amqp091.Confirmation, 1))

	err = ch.PublishWithContext(
		ctx,
		b.exchangeName,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp091.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp091.Persistent, // Request the message to be saved to disk
			Body:         body,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish event with routingKey %s: %w", routingKey, err)
	}

	// Block until we receive an Ack from the broker confirming it has received
	// and persisted the message, or the context times out.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case confirm, ok := <-confirms:
		if !ok {
			return fmt.Errorf("publisher confirm channel closed prematurely")
		}
		if !confirm.Ack {
			return fmt.Errorf("message nacked by rabbitmq broker (delivery tag: %d)", confirm.DeliveryTag)
		}
	}

	return nil
}

func (b *RabbitBroker) RegisterConsumerGroup(
	parentCtx context.Context,
	queueName string,
	routingKeys []string,
	handler func(ctx context.Context, msg []byte) error,
) error {
	ch, err := b.conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open consumer channel: %w", err)
	}

	// Queue configured with Dead-Letter Exchange (DLX) & Dead-Letter Routing Key (DLQ)
	args := amqp091.Table{
		"x-dead-letter-exchange":    b.dlxName,
		"x-dead-letter-routing-key": b.dlqName,
	}

	_, err = ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		args,
	)
	if err != nil {
		ch.Close()
		return fmt.Errorf("failed to declare consumer queue %s: %w", queueName, err)
	}

	for _, rKey := range routingKeys {
		err = ch.QueueBind(
			queueName,
			rKey,
			b.exchangeName,
			false,
			nil,
		)
		if err != nil {
			ch.Close()
			return fmt.Errorf("failed to bind queue %s with key %s: %w", queueName, rKey, err)
		}
	}

	deliveries, err := ch.Consume(
		queueName,
		"",    // consumer tag
		false, // auto-ack (false = manual ack)
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		ch.Close()
		return fmt.Errorf("failed to start consuming from %s: %w", queueName, err)
	}

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer ch.Close()

		for {
			select {
			case <-parentCtx.Done():
				slog.Info("Stopping consumer group (parent context done)", "queue", queueName)
				return
			case <-b.ctx.Done():
				slog.Info("Stopping consumer group (broker closed)", "queue", queueName)
				return
			case d, ok := <-deliveries:
				if !ok {
					slog.Warn("Consumer delivery channel closed", "queue", queueName)
					return
				}

				// Execute module handler in isolated context
				if err := handler(parentCtx, d.Body); err != nil {
					slog.Error("Consumer handler failed; sending message to DLQ", "queue", queueName, "err", err)
					// Reject message without requeue -> routes to x-dead-letter-exchange (billing.dlq)
					_ = d.Nack(false, false)
				} else {
					_ = d.Ack(false)
				}
			}
		}
	}()

	slog.Info("Registered consumer group successfully", "queue", queueName, "routingKeys", routingKeys)
	return nil
}

func (b *RabbitBroker) Close() error {
	b.cancel()  // Signal all consumer loops to terminate cleanly
	b.wg.Wait() // Wait for all background goroutines to finish processing
	return nil
}

var _ Broker = (*RabbitBroker)(nil)
