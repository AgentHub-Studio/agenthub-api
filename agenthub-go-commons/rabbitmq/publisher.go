package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publishes messages to RabbitMQ with auto-reconnection.
type Publisher struct {
	url        string
	mu         sync.Mutex
	conn       *amqp.Connection
	ch         *amqp.Channel
	maxRetries int
}

// NewPublisher creates a Publisher connected to url.
func NewPublisher(url string) (*Publisher, error) {
	p := &Publisher{url: url, maxRetries: 3}
	if err := p.connect(); err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Publisher) connect() error {
	conn, err := amqp.Dial(p.url)
	if err != nil {
		return fmt.Errorf("rabbitmq: dial %q: %w", p.url, err)
	}
	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("rabbitmq: open channel: %w", err)
	}
	p.conn = conn
	p.ch = ch
	return nil
}

// Publish encodes payload as JSON and publishes to exchange with routingKey.
func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("rabbitmq: marshal payload: %w", err)
	}

	msg := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		err = p.ch.PublishWithContext(ctx, exchange, routingKey, false, false, msg)
		if err == nil {
			return nil
		}
		slog.Warn("rabbitmq: publish failed, reconnecting", "attempt", attempt+1, "err", err)
		if reconnErr := p.connect(); reconnErr != nil {
			continue
		}
	}
	return fmt.Errorf("rabbitmq: publish to %s/%s after %d retries: %w", exchange, routingKey, p.maxRetries, err)
}

// Close closes the channel and connection.
func (p *Publisher) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch != nil {
		p.ch.Close()
	}
	if p.conn != nil {
		p.conn.Close()
	}
}
