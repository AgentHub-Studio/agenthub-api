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
	PostgresDSN       string
	RabbitMQURL       string
	MinIOEndpoint     string
	ClickHouseAddr    string
	KeycloakBaseURL   string
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

// StartMinIO starts a MinIO container and returns the S3-compatible endpoint.
func StartMinIO(ctx context.Context, t *testing.T) string {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "minio/minio:RELEASE.2024-01-16T16-07-38Z",
		ExposedPorts: []string{"9000/tcp"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     "minioadmin",
			"MINIO_ROOT_PASSWORD": "minioadmin",
		},
		Cmd:        []string{"server", "/data"},
		WaitingFor: wait.ForHTTP("/minio/health/ready").WithPort("9000/tcp").WithStartupTimeout(60 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start minio container: %v", err)
	}
	t.Cleanup(func() { c.Terminate(ctx) })

	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "9000")
	return fmt.Sprintf("%s:%s", host, port.Port())
}

// StartClickHouse starts a ClickHouse container and returns the native TCP address (host:port).
func StartClickHouse(ctx context.Context, t *testing.T) string {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "clickhouse/clickhouse-server:24.1-alpine",
		ExposedPorts: []string{"9000/tcp", "8123/tcp"},
		WaitingFor: wait.ForHTTP("/ping").WithPort("8123/tcp").WithStartupTimeout(60 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start clickhouse container: %v", err)
	}
	t.Cleanup(func() { c.Terminate(ctx) })

	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "9000")
	return fmt.Sprintf("%s:%s", host, port.Port())
}

// StartKeycloak starts a Keycloak container and returns the base URL.
// A realm named realmName is created via the admin API after startup.
func StartKeycloak(ctx context.Context, t *testing.T) string {
	t.Helper()
	req := testcontainers.ContainerRequest{
		Image:        "quay.io/keycloak/keycloak:24.0",
		ExposedPorts: []string{"8080/tcp"},
		Env: map[string]string{
			"KEYCLOAK_ADMIN":          "admin",
			"KEYCLOAK_ADMIN_PASSWORD": "admin",
			"KC_HTTP_ENABLED":         "true",
			"KC_HOSTNAME_STRICT":      "false",
		},
		Cmd: []string{"start-dev"},
		WaitingFor: wait.ForHTTP("/health/ready").
			WithPort("8080/tcp").
			WithStartupTimeout(120 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start keycloak container: %v", err)
	}
	t.Cleanup(func() { c.Terminate(ctx) })

	host, _ := c.Host(ctx)
	port, _ := c.MappedPort(ctx, "8080")
	return fmt.Sprintf("http://%s:%s", host, port.Port())
}
