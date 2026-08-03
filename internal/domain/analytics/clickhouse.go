package analytics

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
)

// ClickHouseClient talks to ClickHouse through the HTTP interface.
type ClickHouseClient struct {
	baseURL  string
	database string
	username string
	password string
	http     *http.Client
}

// NewClickHouseClient creates a ClickHouse HTTP client from config.
func NewClickHouseClient(cfg config.ClickHouseConfig) *ClickHouseClient {
	return &ClickHouseClient{
		baseURL:  strings.TrimRight(cfg.URL, "/"),
		database: cfg.Database,
		username: cfg.Username,
		password: cfg.Password,
		http:     &http.Client{Timeout: 10 * time.Second},
	}
}

// EnsureSchema creates the analytics database and tables when ClickHouse is configured.
func (c *ClickHouseClient) EnsureSchema(ctx context.Context) error {
	if c == nil || c.baseURL == "" {
		return nil
	}
	db := c.database
	if db == "" {
		db = "agenthub"
	}
	if err := c.execWithDatabase(ctx, "", "CREATE DATABASE IF NOT EXISTS "+quoteIdent(db)); err != nil {
		return err
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS agent_run_metrics (
			tenant_id String,
			agent_id UUID,
			session_id UUID,
			run_id String,
			started_at DateTime64(3),
			completed_at DateTime64(3),
			duration_ms UInt64,
			total_turns UInt32,
			total_tokens UInt64,
			prompt_tokens UInt64,
			completion_tokens UInt64,
			cost_usd Float64,
			provider String,
			model String,
			finish_reason String,
			error Nullable(String)
		) ENGINE = MergeTree() ORDER BY (tenant_id, started_at)`,
		`CREATE TABLE IF NOT EXISTS tool_execution_metrics (
			tenant_id String,
			agent_id UUID,
			session_id UUID,
			run_id String,
			tool_name String,
			tool_id String,
			started_at DateTime64(3),
			duration_ms UInt64,
			state String,
			error Nullable(String),
			was_cached Bool,
			was_denied Bool
		) ENGINE = MergeTree() ORDER BY (tenant_id, tool_name, started_at)`,
	}
	for _, stmt := range stmts {
		if err := c.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

// Exec executes a statement against the configured database.
func (c *ClickHouseClient) Exec(ctx context.Context, query string) error {
	return c.execWithDatabase(ctx, c.database, query)
}

func (c *ClickHouseClient) execWithDatabase(ctx context.Context, database, query string) error {
	_, err := c.do(ctx, database, []byte(query))
	return err
}

// InsertJSONEachRow inserts JSONEachRow rows into a table.
func (c *ClickHouseClient) InsertJSONEachRow(ctx context.Context, table string, rows []byte) error {
	if len(bytes.TrimSpace(rows)) == 0 {
		return nil
	}
	query := fmt.Sprintf("INSERT INTO %s FORMAT JSONEachRow\n", quoteIdent(table))
	_, err := c.do(ctx, c.database, append([]byte(query), rows...))
	return err
}

func (c *ClickHouseClient) do(ctx context.Context, database string, body []byte) ([]byte, error) {
	if c == nil || c.baseURL == "" {
		return nil, fmt.Errorf("clickhouse: URL is not configured")
	}
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: invalid URL: %w", err)
	}
	q := endpoint.Query()
	if database != "" {
		q.Set("database", database)
	}
	endpoint.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("clickhouse: request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("clickhouse: status %d: %s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

func clickhouseTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05.000")
}

func quoteString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "\\'") + "'"
}

func quoteIdent(s string) string {
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			return r
		default:
			return -1
		}
	}, s)
	if clean == "" {
		clean = "agenthub"
	}
	return "`" + clean + "`"
}

func uuidExpr(id uuid.UUID) string {
	return "toUUID(" + quoteString(id.String()) + ")"
}

func timeExpr(t time.Time) string {
	return "parseDateTime64BestEffort(" + quoteString(t.UTC().Format(time.RFC3339Nano)) + ")"
}
