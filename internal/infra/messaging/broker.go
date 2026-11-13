package messaging

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/rabbitmq/amqp091-go"

	"github.com/thec1oud/billing/internal/infra/logger"
)

const (
	DefaultExchangeName = "billing.events"
	DefaultDLXName      = "billing.dlx"
	DefaultDLQName      = "billing.dlq"
)

type Option func(*RabbitBroker)

const (
	DefaultPrefetch       = 10
	DefaultRetries        = 3
	DefaultHandlerTimeout = 30 * time.Second
)

func WithDialer(dial func() (*amqp091.Connection, error)) Option {
	return func(b *RabbitBroker) { b.dial = dial }
}

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
	RegisterConsumerGroup(
		ctx context.Context,
		queueName string,
		routingKeys []string,
		handler func(ctx context.Context, msg []byte) error,
	) error
	Close() error
}

type RabbitBroker struct {
	conn         *amqp091.Connection
	exchangeName string
	dlxName      string
	dlqName      string

	pubMu          sync.Mutex
	pubCh          *amqp091.Channel
	pubConfirms    <-chan amqp091.Confirmation
	dial           func() (*amqp091.Connection, error)
	prefetch       int
	retries        int
	handlerTimeout time.Duration

	wg     sync.WaitGroup     // WaitGroup tracking all consumer background goroutines
	ctx    context.Context    // Parent lifecycle context
	cancel context.CancelFunc // Cancellation trigger for graceful shutdown
	log    *slog.Logger
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
		log:          logger.ForComponent("infra_messaging"),
		prefetch:     DefaultPrefetch, retries: DefaultRetries, handlerTimeout: DefaultHandlerTimeout,
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
	defer func() { _ = ch.Close() }()

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

	b.log.Info("RabbitMQ messaging topology initialized successfully",
		slog.String("exchange", b.exchangeName),
		slog.String("dlx", b.dlxName),
		slog.String("dlq", b.dlqName),
	)
	return nil
}

func (b *RabbitBroker) PublishEvent(ctx context.Context, routingKey string, body []byte) error {
	b.pubMu.Lock()
	defer b.pubMu.Unlock()
	ch, err := b.publisherChannel()
	if err != nil {
		return err
	}
	confirms := b.pubConfirms

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
		b.resetPublisher()
		return fmt.Errorf("failed to publish event with routingKey %s: %w", routingKey, err)
	}

	// Block until we receive an Ack from the broker confirming it has received
	// and persisted the message, or the context times out.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case confirm, ok := <-confirms:
		if !ok {
			b.resetPublisher()
			return fmt.Errorf("publisher confirm channel closed prematurely")
		}
		if !confirm.Ack {
			b.resetPublisher()
			return fmt.Errorf("message nacked by rabbitmq broker (delivery tag: %d)", confirm.DeliveryTag)
		}
	}

	return nil
}

func (b *RabbitBroker) publisherChannel() (*amqp091.Channel, error) {
	if b.pubCh != nil && !b.pubCh.IsClosed() {
		return b.pubCh, nil
	}
	if err := b.ensureConnection(); err != nil {
		return nil, err
	}
	ch, err := b.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open publish channel: %w", err)
	}
	if err = ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, err
	}
	b.pubCh = ch
	b.pubConfirms = ch.NotifyPublish(make(chan amqp091.Confirmation, 1))
	return ch, nil
}
func (b *RabbitBroker) resetPublisher() {
	if b.pubCh != nil {
		_ = b.pubCh.Close()
		b.pubCh = nil
		b.pubConfirms = nil
	}
}
func (b *RabbitBroker) ensureConnection() error {
	if b.conn != nil && !b.conn.IsClosed() {
		return nil
	}
	if b.dial == nil {
		return fmt.Errorf("rabbitmq connection is closed and no reconnect dialer is configured")
	}
	for b.ctx.Err() == nil {
		conn, err := b.dial()
		if err == nil {
			b.conn = conn
			return nil
		}
		b.log.Warn("RabbitMQ reconnect failed", logger.Err(err))
		select {
		case <-b.ctx.Done():
			return b.ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return b.ctx.Err()
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
	if err := ch.Qos(b.prefetch, 0, false); err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to set consumer prefetch: %w", err)
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
		_ = ch.Close()
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
			_ = ch.Close()
			return fmt.Errorf("failed to bind queue %s with key %s: %w", queueName, rKey, err)
		}
	}
	// Expired retry messages return to this queue through RabbitMQ's default exchange.
	retryQueue := queueName + ".retry"
	_, err = ch.QueueDeclare(retryQueue, true, false, false, false, amqp091.Table{
		"x-message-ttl": int32(5000), "x-dead-letter-exchange": "", "x-dead-letter-routing-key": queueName,
	})
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to declare retry queue %s: %w", retryQueue, err)
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
		_ = ch.Close()
		return fmt.Errorf("failed to start consuming from %s: %w", queueName, err)
	}

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		defer func() { _ = ch.Close() }()

		for {
			select {
			case <-parentCtx.Done():
				b.log.Info("Stopping consumer group (parent context done)", slog.String("queue", queueName))
				return
			case <-b.ctx.Done():
				b.log.Info("Stopping consumer group (broker closed)", slog.String("queue", queueName))
				return
			case d, ok := <-deliveries:
				if !ok {
					b.log.Warn("Consumer delivery channel closed", slog.String("queue", queueName))
					return
				}

				// Bound each handler invocation so a stuck downstream call cannot hold a delivery forever.
				msgCtx, cancel := context.WithTimeout(parentCtx, b.handlerTimeout)
				handlerErr := handler(msgCtx, d.Body)
				cancel()
				if handlerErr != nil {
					count := retryCount(d.Headers)
					if count < b.retries {
						err := ch.PublishWithContext(parentCtx, "", retryQueue, false, false, amqp091.Publishing{
							ContentType: d.ContentType, DeliveryMode: amqp091.Persistent, Body: d.Body,
							Headers: amqp091.Table{"x-retry-count": int32(count + 1)},
						})
						if err == nil {
							err = d.Ack(false)
						}
						if err != nil {
							b.log.Error("Failed to schedule message retry", slog.String("queue", queueName), logger.Err(err))
							_ = d.Nack(false, true)
						}
					} else {
						b.log.Error("Consumer retries exhausted; sending message to DLQ", slog.String("queue", queueName), logger.Err(handlerErr))
						_ = d.Nack(false, false)
					}
				} else {
					_ = d.Ack(false)
				}
			}
		}
	}()

	b.log.Info("Registered consumer group successfully", slog.String("queue", queueName), slog.Any("routingKeys", routingKeys))
	return nil
}

func retryCount(headers amqp091.Table) int {
	if headers == nil {
		return 0
	}
	switch n := headers["x-retry-count"].(type) {
	case int32:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

func (b *RabbitBroker) Close() error {
	b.cancel()  // Signal all consumer loops to terminate cleanly
	b.wg.Wait() // Wait for all background goroutines to finish processing
	b.pubMu.Lock()
	b.resetPublisher()
	b.pubMu.Unlock()
	return nil
}

var _ Broker = (*RabbitBroker)(nil)
