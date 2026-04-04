package testutil

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// NewPostgresContainer starts a PostgreSQL 16 + pgvector container and returns
// a pgxpool.Pool connected to it. The container is stopped when t finishes.
func NewPostgresContainer(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	const (
		dbName = "testdb"
		dbUser = "testuser"
		dbPass = "testpass"
		dbPort = "5432/tcp"
	)

	req := testcontainers.ContainerRequest{
		Image: "pgvector/pgvector:pg16",
		Env: map[string]string{
			"POSTGRES_DB":       dbName,
			"POSTGRES_USER":     dbUser,
			"POSTGRES_PASSWORD": dbPass,
		},
		ExposedPorts: []string{dbPort},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("testutil: start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := ctr.Terminate(ctx); err != nil {
			t.Logf("testutil: terminate postgres container: %v", err)
		}
	})

	host, err := ctr.Host(ctx)
	if err != nil {
		t.Fatalf("testutil: get container host: %v", err)
	}
	port, err := ctr.MappedPort(ctx, dbPort)
	if err != nil {
		t.Fatalf("testutil: get mapped port: %v", err)
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPass, host, port.Port(), dbName)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("testutil: connect to test postgres: %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// MustExec executes a SQL statement against pool, failing the test on error.
func MustExec(t *testing.T, pool *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, sql, args...); err != nil {
		t.Fatalf("testutil: exec %q: %v", sql, err)
	}
}

// CreateTenantSchema creates the ah_{tenantID} schema for testing.
func CreateTenantSchema(t *testing.T, pool *pgxpool.Pool, tenantID string) {
	t.Helper()
	schema := fmt.Sprintf("ah_%s", tenantID)
	MustExec(t, pool, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", schema))
}
