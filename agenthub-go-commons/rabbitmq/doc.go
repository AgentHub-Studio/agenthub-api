// Package rabbitmq provides Publisher and Consumer helpers for AgentHub Go services,
// wrapping rabbitmq/amqp091-go with retry, reconnection, and structured logging.
//
// Usage:
//
//	pub, err := rabbitmq.NewPublisher(cfg.RabbitMQURL)
//	err = pub.Publish(ctx, "exchange", "routing.key", payload)
package rabbitmq
