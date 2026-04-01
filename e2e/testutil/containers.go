package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Containers holds all running test containers.
type Containers struct {
	PostgresDSN   string
	RabbitMQURL   string
	MinIOEndpoint string
}

// StartPostgres starts a PostgreSQL 16 + pgvector container and returns the DSN.
func StartPostgres(ctx context.Context, t *testing.T) string {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "pgvector/pgvector:pg16",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_DB":       "agenthub_test",
			"POSTGRES_USER":     "agenthub",
			"POSTGRES_PASSWORD": "agenthub",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { c.Terminate(ctx) })

	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "5432")
	return fmt.Sprintf("postgres://agenthub:agenthub@%s:%s/agenthub_test?sslmode=disable", host, port.Port())
}

// StartRabbitMQ starts a RabbitMQ container and returns the AMQP URL.
func StartRabbitMQ(ctx context.Context, t *testing.T) string {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3.13-management-alpine",
		ExposedPorts: []string{"5672/tcp"},
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "agenthub",
			"RABBITMQ_DEFAULT_PASS": "agenthub",
		},
		WaitingFor: wait.ForLog("Server startup complete").WithStartupTimeout(60 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start rabbitmq container: %v", err)
	}
	t.Cleanup(func() { c.Terminate(ctx) })

	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "5672")
	return fmt.Sprintf("amqp://agenthub:agenthub@%s:%s/", host, port.Port())
}
