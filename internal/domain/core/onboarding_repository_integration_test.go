//go:build integration

package core_test

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/database"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

// Integration tests for OnboardingStatusRepository.LoadSignals against a real
// Postgres (testcontainers pgvector:pg16). The all-false baseline test alone
// proves the five EXISTS sub-queries parse and run against the real per-tenant
// schema (settings, agent, agent_skill, knowledge_base, chat_run all present).
//
//	go test -tags=integration ./internal/domain/core/ -run TestIntegration_Onboarding

const onboardingTestTenant = "obtest"

// onboardingSchemasDir resolves migrations/schemas relative to this test file.
func onboardingSchemasDir(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../../migrations/schemas")
	require.NoError(t, err)
	require.DirExists(t, abs, "migrations/schemas dir must exist")
	return abs
}

// setupOnboardingSchema spins a Postgres container, creates ah_{tenant}, and
// applies every migrations/schemas/*.up.sql in order — mirroring what
// MigrateAllTenants does when a real tenant is provisioned.
func setupOnboardingSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()

	schema := "ah_" + onboardingTestTenant
	testutil.MustExec(t, pool, "CREATE SCHEMA IF NOT EXISTS "+schema)

	dir := onboardingSchemasDir(t)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var ups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			ups = append(ups, e.Name())
		}
	}
	sort.Strings(ups)
	require.NotEmpty(t, ups)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = conn.Exec(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)
	for _, name := range ups {
		sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err, "read migration %s", name)
		_, err = conn.Exec(ctx, string(sqlBytes))
		require.NoError(t, err, "apply migration %s", name)
	}
	conn.Release()

	return pool, tenant.NewContext(ctx, onboardingTestTenant)
}

func TestIntegration_OnboardingSignals_FreshTenant_AllFalse(t *testing.T) {
	pool, ctx := setupOnboardingSchema(t)
	repo := core.NewOnboardingStatusRepository(pool)

	sig, err := repo.LoadSignals(ctx)
	require.NoError(t, err)
	assert.False(t, sig.LLMConfigured, "fresh tenant has no LLM provider key")
	assert.False(t, sig.HasUserAgent, "only the auto-seeded default agent exists")
	assert.False(t, sig.HasSkillBound)
	assert.False(t, sig.HasKnowledgeBase)
	assert.False(t, sig.HasCompletedRun)
	assert.False(t, sig.Dismissed, "onboarding.completed is seeded false by 000063")
}

func TestIntegration_OnboardingSignals_DetectsLLMKey(t *testing.T) {
	pool, ctx := setupOnboardingSchema(t)
	repo := core.NewOnboardingStatusRepository(pool)

	conn, release, err := database.AcquireWithTenant(ctx, pool, onboardingTestTenant)
	require.NoError(t, err)
	defer release()
	_, err = conn.Exec(ctx,
		`INSERT INTO settings (key, value) VALUES ('openrouter.apiKey', '"sk-test-key"'::jsonb)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`)
	require.NoError(t, err)

	sig, err := repo.LoadSignals(ctx)
	require.NoError(t, err)
	assert.True(t, sig.LLMConfigured, "a non-empty provider key must mark connect-llm done")
}

func TestIntegration_OnboardingSignals_EmptyApiKeyNotConfigured(t *testing.T) {
	pool, ctx := setupOnboardingSchema(t)
	repo := core.NewOnboardingStatusRepository(pool)

	conn, release, err := database.AcquireWithTenant(ctx, pool, onboardingTestTenant)
	require.NoError(t, err)
	defer release()
	// An empty-string key value must NOT count as configured.
	_, err = conn.Exec(ctx,
		`INSERT INTO settings (key, value) VALUES ('openai.apiKey', '""'::jsonb)
		 ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`)
	require.NoError(t, err)

	sig, err := repo.LoadSignals(ctx)
	require.NoError(t, err)
	assert.False(t, sig.LLMConfigured, "an empty-string API key must not count as configured")
}

func TestIntegration_OnboardingSignals_DismissedFlag(t *testing.T) {
	pool, ctx := setupOnboardingSchema(t)
	repo := core.NewOnboardingStatusRepository(pool)

	conn, release, err := database.AcquireWithTenant(ctx, pool, onboardingTestTenant)
	require.NoError(t, err)
	defer release()
	_, err = conn.Exec(ctx,
		`UPDATE settings SET value = 'true'::jsonb WHERE key = 'onboarding.completed'`)
	require.NoError(t, err)

	sig, err := repo.LoadSignals(ctx)
	require.NoError(t, err)
	assert.True(t, sig.Dismissed, "settings.onboarding.completed = true must set Dismissed")
}

func TestIntegration_OnboardingSignals_DetectsUserAgent(t *testing.T) {
	pool, ctx := setupOnboardingSchema(t)
	repo := core.NewOnboardingStatusRepository(pool)

	conn, release, err := database.AcquireWithTenant(ctx, pool, onboardingTestTenant)
	require.NoError(t, err)
	defer release()
	// A user-created agent (slug != agenthub-assistant) marks create-agent done.
	_, err = conn.Exec(ctx,
		`INSERT INTO agent (name, slug, status, config)
		 VALUES ('My Agent', 'my-agent', 'PUBLISHED', '{}'::jsonb)`)
	require.NoError(t, err)

	sig, err := repo.LoadSignals(ctx)
	require.NoError(t, err)
	assert.True(t, sig.HasUserAgent, "an agent other than the seeded default must mark create-agent done")
}
