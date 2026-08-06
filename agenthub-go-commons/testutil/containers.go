package testutil

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	postgresDatabase = "testdb"
	postgresUser     = "testuser"
	postgresPassword = "testpass"
	postgresPort     = "5432/tcp"
	postgresDSNEnv   = "AGENTHUB_TEST_POSTGRES_DSN"
)

type sharedPostgresContainer struct {
	dsn string
}

var (
	sharedPostgresMu  sync.Mutex
	sharedPostgres    *sharedPostgresContainer
	sharedPostgresErr error
	testDatabaseSeq   atomic.Uint64
)

// NewPostgresContainer returns a pool connected to a database exclusive to t.
// A build-scoped fixture is used when the test runner provides postgresDSNEnv;
// otherwise, one container is shared by the current Go test process.
func NewPostgresContainer(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	shared, err := postgresContainer(ctx)
	if err != nil {
		t.Fatalf("testutil: start shared postgres container: %v", err)
	}

	databaseName := fmt.Sprintf("testdb_%d_%d", os.Getpid(), testDatabaseSeq.Add(1))
	admin, err := pgx.Connect(ctx, shared.dsn)
	if err != nil {
		t.Fatalf("testutil: connect to shared postgres: %v", err)
	}
	if _, err := admin.Exec(ctx, fmt.Sprintf("CREATE DATABASE %s", databaseName)); err != nil {
		admin.Close(ctx)
		t.Fatalf("testutil: create isolated database: %v", err)
	}
	if err := admin.Close(ctx); err != nil {
		t.Fatalf("testutil: close shared postgres admin connection: %v", err)
	}

	pool, err := pgxpool.New(ctx, databaseDSN(shared.dsn, databaseName))
	if err != nil {
		t.Fatalf("testutil: connect to isolated test postgres: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		admin, err := pgx.Connect(ctx, shared.dsn)
		if err != nil {
			t.Logf("testutil: reconnect to shared postgres for cleanup: %v", err)
			return
		}
		defer admin.Close(ctx)
		if _, err := admin.Exec(ctx, fmt.Sprintf("DROP DATABASE IF EXISTS %s WITH (FORCE)", databaseName)); err != nil {
			t.Logf("testutil: drop isolated database %q: %v", databaseName, err)
		}
	})

	return pool
}

func postgresContainer(ctx context.Context) (*sharedPostgresContainer, error) {
	sharedPostgresMu.Lock()
	defer sharedPostgresMu.Unlock()

	if sharedPostgres != nil || sharedPostgresErr != nil {
		return sharedPostgres, sharedPostgresErr
	}
	if dsn := strings.TrimSpace(os.Getenv(postgresDSNEnv)); dsn != "" {
		sharedPostgres = &sharedPostgresContainer{dsn: dsn}
		return sharedPostgres, nil
	}

	req := testcontainers.ContainerRequest{
		Image: "pgvector/pgvector:pg16",
		Cmd: []string{
			"postgres",
			"-c", "fsync=off",
			"-c", "synchronous_commit=off",
			"-c", "full_page_writes=off",
		},
		Env: map[string]string{
			"POSTGRES_DB":       postgresDatabase,
			"POSTGRES_USER":     postgresUser,
			"POSTGRES_PASSWORD": postgresPassword,
		},
		ExposedPorts: []string{postgresPort},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(60 * time.Second),
	}

	ctr, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		sharedPostgresErr = err
		return nil, err
	}

	host, err := ctr.Host(ctx)
	if err != nil {
		sharedPostgresErr = err
		return nil, err
	}
	port, err := ctr.MappedPort(ctx, postgresPort)
	if err != nil {
		sharedPostgresErr = err
		return nil, err
	}

	sharedPostgres = &sharedPostgresContainer{
		dsn: fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", postgresUser, postgresPassword, host, port.Port(), postgresDatabase),
	}
	return sharedPostgres, nil
}

func databaseDSN(baseDSN, databaseName string) string {
	return strings.Replace(baseDSN, "/"+postgresDatabase+"?", "/"+databaseName+"?", 1)
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
