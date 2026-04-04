package rabbitmq

import (
	"context"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Handler is a function that processes a single delivery.
// Return nil to ack, non-nil to nack (requeue=false).
type Handler func(ctx context.Context, body []byte) error

// Consumer consumes messages from a queue.
type Consumer struct {
	url  string
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewConsumer creates a Consumer connected to url.
func NewConsumer(url string) (*Consumer, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: consumer dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("rabbitmq: consumer channel: %w", err)
	}
	return &Consumer{url: url, conn: conn, ch: ch}, nil
}

// Consume starts consuming messages from queue, calling handler for each.
// Blocks until ctx is cancelled or the connection drops.
func (c *Consumer) Consume(ctx context.Context, queue string, prefetch int, handler Handler) error {
	if err := c.ch.Qos(prefetch, 0, false); err != nil {
		return fmt.Errorf("rabbitmq: set qos: %w", err)
	}

	deliveries, err := c.ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("rabbitmq: consume queue %q: %w", queue, err)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("rabbitmq: channel closed")
			}
			if err := handler(ctx, d.Body); err != nil {
				slog.Error("rabbitmq: handler error, nacking", "err", err)
				if err := d.Nack(false, false); err != nil {
					slog.Error("rabbitmq: nack failed", "err", err)
				}
			} else {
				if err := d.Ack(false); err != nil {
					slog.Error("rabbitmq: ack failed", "err", err)
				}
			}
		}
	}
}

// Close closes the consumer.
func (c *Consumer) Close() {
	if c.ch != nil {
		c.ch.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}
