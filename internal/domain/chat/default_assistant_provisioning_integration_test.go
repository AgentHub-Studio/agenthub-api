//go:build integration

package chat_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for the out-of-the-box default-assistant provisioning
// (migrations 000063 + 000068) and the agentless-session routing flow, run
// against a real Postgres (testcontainers pgvector:pg16).
//
//	go test -tags=integration ./internal/domain/chat/ -run TestIntegration_

const provisioningTestTenant = "oobtest"

// schemasMigrationsDir resolves migrations/schemas relative to this test file
// (internal/domain/chat/).
func schemasMigrationsDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../../migrations/schemas")
	require.NoError(t, err)
	require.DirExists(t, abs, "migrations/schemas dir must exist")
	return abs
}

// schemaMigrationFiles returns every *.up.sql under migrations/schemas, sorted
// by name (which is also numeric order).
func schemaMigrationFiles(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(schemasMigrationsDir(t))
	require.NoError(t, err)
	var ups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)
	require.NotEmpty(t, ups)
	return ups
}

// applyOneSchemaMigration executes a single migration file against the test
// tenant's schema (search_path is set by AcquireWithTenant).
func applyOneSchemaMigration(t *testing.T, pool *pgxpool.Pool, ctx context.Context, name string) {
	t.Helper()
	sqlBytes, err := os.ReadFile(filepath.Join(schemasMigrationsDir(t), name))
	require.NoError(t, err, "read migration %s", name)
	conn, release, err := database.AcquireWithTenant(ctx, pool, provisioningTestTenant)
	require.NoError(t, err)
	defer release()
	_, err = conn.Exec(ctx, string(sqlBytes))
	require.NoError(t, err, "apply migration %s", name)
}

// setupTenantSchema spins a Postgres container, creates ah_{tenant}, and applies
// every migrations/schemas/*.up.sql in order — mirroring what MigrateAllTenants
// does when a real tenant is provisioned. Returns the pool and a tenant-scoped
// context.
func setupTenantSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()

	schema := "ah_" + provisioningTestTenant
	testutil.MustExec(t, pool, "CREATE SCHEMA IF NOT EXISTS "+schema)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)
	for _, name := range schemaMigrationFiles(t) {
		sqlBytes, err := os.ReadFile(filepath.Join(schemasMigrationsDir(t), name))
		require.NoError(t, err, "read migration %s", name)
		_, err = conn.Exec(ctx, string(sqlBytes))
		require.NoError(t, err, "apply migration %s", name)
	}
	conn.Release()

	return pool, tenant.NewContext(ctx, provisioningTestTenant)
}

// TestIntegration_FreshTenant_HasDefaultAssistant verifies a freshly migrated
// tenant ships with exactly one agent — the built-in agenthub-assistant — so
// the platform is usable out of the box. The agent is seeded by migration
// 000014; 000063's meu-assistente INSERT is a guarded no-op because the agent
// table is never empty by then.
func TestIntegration_FreshTenant_HasDefaultAssistant(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	conn, release, err := database.AcquireWithTenant(ctx, pool, provisioningTestTenant)
	require.NoError(t, err)
	defer release()

	var count int
	require.NoError(t, conn.QueryRow(ctx, "SELECT COUNT(*) FROM agent").Scan(&count))
	assert.Equal(t, 1, count, "a fresh tenant must have exactly the built-in default agent")

	var name, slug, status string
	require.NoError(t, conn.QueryRow(ctx,
		"SELECT name, slug, status FROM agent",
	).Scan(&name, &slug, &status))
	assert.Equal(t, "agenthub-assistant", slug,
		"the well-known slug the chat routing queries prefer must exist")
	assert.Equal(t, "PUBLISHED", status, "the default agent must be immediately usable")
	assert.NotEmpty(t, name)
}

func TestIntegration_Migration000068_SeedsProviderSettings(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	conn, release, err := database.AcquireWithTenant(ctx, pool, provisioningTestTenant)
	require.NoError(t, err)
	defer release()

	got := map[string]string{}
	rows, err := conn.Query(ctx,
		`SELECT key, value::text FROM settings
		 WHERE key IN ('llm.defaultProvider','openrouter.model','onboarding.completed')`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var k, v string
		require.NoError(t, rows.Scan(&k, &v))
		got[k] = v
	}
	require.NoError(t, rows.Err())

	assert.Equal(t, `"openrouter"`, got["llm.defaultProvider"])
	assert.Equal(t, `"mistralai/mistral-nemo"`, got["openrouter.model"])
	assert.Equal(t, "false", got["onboarding.completed"])
}

func TestIntegration_Migration000063And68_Idempotent(t *testing.T) {
	pool, ctx := setupTenantSchema(t)

	// Re-apply both migrations: the WHERE NOT EXISTS guard and ON CONFLICT DO
	// NOTHING clauses must make a second application a complete no-op. This is
	// the same guard that protects tenants which already had agents.
	applyOneSchemaMigration(t, pool, ctx, "000063_default_assistant_provisioning.up.sql")
	applyOneSchemaMigration(t, pool, ctx, "000068_oob_usability_fixes.up.sql")

	conn, release, err := database.AcquireWithTenant(ctx, pool, provisioningTestTenant)
	require.NoError(t, err)
	defer release()

	var agentCount int
	require.NoError(t, conn.QueryRow(ctx, "SELECT COUNT(*) FROM agent").Scan(&agentCount))
	assert.Equal(t, 1, agentCount, "re-applying the seed migrations must not duplicate the default agent")

	var slug string
	require.NoError(t, conn.QueryRow(ctx, "SELECT slug FROM agent").Scan(&slug))
	assert.Equal(t, "agenthub-assistant", slug)
}

func TestIntegration_FindAgentsForRouting_ReturnsSeededAgent(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	repo := chat.NewRepository(pool)

	agents, err := repo.FindAgentsForRouting(ctx)
	require.NoError(t, err)
	require.Len(t, agents, 1, "the built-in default agent must be routable")
	assert.Equal(t, "agenthub-assistant", agents[0].Slug)
	assert.NotEmpty(t, agents[0].Name)
}

func TestIntegration_UpdateSessionSnapshots_Persists(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	repo := chat.NewRepository(pool)

	created, err := repo.CreateSession(ctx, chat.ChatSession{Title: "snapshot test", Status: chat.StatusActive})
	require.NoError(t, err)

	prompt := "You are the routed agent."
	agentSnapshot := json.RawMessage(`{"systemPrompt":"You are the routed agent.","modelConfig":{"provider":"openrouter","model":"mistralai/mistral-nemo"},"skillIds":[]}`)
	agentSnapshotHash := sha256.Sum256(agentSnapshot)
	agentSnapshotHashText := hex.EncodeToString(agentSnapshotHash[:])
	err = repo.UpdateSessionSnapshots(ctx, created.ID, &prompt,
		json.RawMessage(`{"provider":"openrouter","model":"mistralai/mistral-nemo"}`),
		json.RawMessage(`{"skillIds":[]}`),
		agentSnapshot,
		&agentSnapshotHashText)
	require.NoError(t, err)

	got, err := repo.GetSessionByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got.SystemPromptSnapshot)
	assert.Equal(t, prompt, *got.SystemPromptSnapshot)
	assert.JSONEq(t, `{"provider":"openrouter","model":"mistralai/mistral-nemo"}`, string(got.ModelConfigSnapshot))
	assert.JSONEq(t, string(agentSnapshot), string(got.AgentSnapshot))
	require.NotNil(t, got.AgentSnapshotHash)
	assert.Equal(t, agentSnapshotHashText, *got.AgentSnapshotHash)
}

func TestIntegration_UpdateSessionConfigHash_PersistsAndLoads(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	repo := chat.NewRepository(pool)

	created, err := repo.CreateSession(ctx, chat.ChatSession{Title: "config hash test", Status: chat.StatusActive})
	require.NoError(t, err)

	hash := strings.Repeat("a", 64)
	require.NoError(t, repo.UpdateSessionConfigHash(ctx, created.ID, hash))

	got, err := repo.GetSessionByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, got.ConfigHash)
	assert.Equal(t, hash, *got.ConfigHash)
}

// integrationRecordingRunner is a chat.SessionRunner that records the RunInput
// and returns an empty, closed event channel.
type integrationRecordingRunner struct {
	lastInput chat.RunInput
}

func (r *integrationRecordingRunner) RunSession(_ context.Context, in chat.RunInput) (<-chan chat.RunEvent, error) {
	r.lastInput = in
	ch := make(chan chat.RunEvent)
	close(ch)
	return ch, nil
}

func TestIntegration_AgentlessSession_RouteBindFlow(t *testing.T) {
	pool, ctx := setupTenantSchema(t)
	repo := chat.NewRepository(pool)

	agents, err := repo.FindAgentsForRouting(ctx)
	require.NoError(t, err)
	require.Len(t, agents, 1)
	seededID := agents[0].ID

	runner := &integrationRecordingRunner{}
	svc := chat.NewService(repo, runner)

	created, err := svc.CreateSession(ctx, chat.CreateSessionRequest{Title: "agentless flow"})
	require.NoError(t, err)
	require.Nil(t, created.AgentID, "session created without agentId")

	_, err = svc.RunSession(ctx, created.ID, "olá, tudo bem?", provisioningTestTenant)
	require.NoError(t, err)

	// The router bound the seeded default agent to the previously agentless session.
	bound, err := repo.GetSessionByID(ctx, created.ID)
	require.NoError(t, err)
	require.NotNil(t, bound.AgentID)
	assert.Equal(t, seededID, *bound.AgentID)
	assert.Equal(t, seededID, runner.lastInput.AgentID)

	// The user message was persisted before the run started.
	conn, release, err := database.AcquireWithTenant(ctx, pool, provisioningTestTenant)
	require.NoError(t, err)
	defer release()
	var userMsgCount int
	require.NoError(t, conn.QueryRow(ctx,
		"SELECT COUNT(*) FROM chat_message WHERE session_id = $1 AND role = 'user'", created.ID,
	).Scan(&userMsgCount))
	assert.Equal(t, 1, userMsgCount)
}
