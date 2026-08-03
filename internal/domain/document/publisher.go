package document

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// DocumentUploadedExchange is the fanout exchange for document pipeline events.
	DocumentUploadedExchange = ""
	// DocumentUploadedQueue is the queue consumed by the extractor service.
	DocumentUploadedQueue = "document.uploaded"
)

// RabbitMQEventPublisher publishes document events to a RabbitMQ queue.
type RabbitMQEventPublisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

// NewRabbitMQEventPublisher dials url, declares the queue, and returns a publisher.
func NewRabbitMQEventPublisher(url string) (*RabbitMQEventPublisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("document publisher: dial %q: %w", url, err)
	}

	ch, err := conn.Channel()
	if err != nil {
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("document publisher: close connection after channel failure: %v", closeErr)
		}
		return nil, fmt.Errorf("document publisher: open channel: %w", err)
	}

	// Declare the queue so it exists before the first publish. The
	// x-dead-letter-exchange argument matches what agenthub-extractor
	// (Python pika consumer) declares; without it RabbitMQ rejects with
	// PRECONDITION_FAILED when the consumer connects after the publisher.
	_, err = ch.QueueDeclare(
		DocumentUploadedQueue,
		true,  // durable
		false, // autoDelete
		false, // exclusive
		false, // noWait
		amqp.Table{
			"x-dead-letter-exchange": "agenthub.documents.dlx",
		},
	)
	if err != nil {
		if closeErr := ch.Close(); closeErr != nil {
			log.Printf("document publisher: close channel after queue declaration failure: %v", closeErr)
		}
		if closeErr := conn.Close(); closeErr != nil {
			log.Printf("document publisher: close connection after queue declaration failure: %v", closeErr)
		}
		return nil, fmt.Errorf("document publisher: declare queue: %w", err)
	}

	return &RabbitMQEventPublisher{conn: conn, ch: ch}, nil
}

// PublishUploaded encodes the event as JSON and publishes it to DocumentUploadedQueue.
func (p *RabbitMQEventPublisher) PublishUploaded(ctx context.Context, event DocumentUploadedEvent) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("document publisher: marshal event: %w", err)
	}

	return p.ch.PublishWithContext(ctx,
		DocumentUploadedExchange, // exchange — empty = default (direct to queue)
		DocumentUploadedQueue,    // routing key = queue name
		false,                    // mandatory
		false,                    // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
}

// Close releases the AMQP channel and connection.
func (p *RabbitMQEventPublisher) Close() {
	if p.ch != nil {
		if err := p.ch.Close(); err != nil {
			log.Printf("document publisher: close channel: %v", err)
		}
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil {
			log.Printf("document publisher: close connection: %v", err)
		}
	}
}
