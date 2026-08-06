//go:build integration

package prompttemplate_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/prompttemplate"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const promptTemplateIsolationTenant = "prompttemplateisolation"

func TestIntegration_PromptTemplateSameSlugIsIsolatedPerAgent(t *testing.T) {
	pool, ctx := setupPromptTemplateIsolationSchema(t)
	repo := prompttemplate.NewRepository(pool)
	service := prompttemplate.NewService(repo)

	agentA := uuid.New()
	agentB := uuid.New()
	seedPromptTemplateAgents(t, pool, agentA, agentB)

	global := createPromptTemplate(t, service, nil, "shared-policy", "global policy")
	templateA := createPromptTemplate(t, service, &agentA, "shared-policy", "agent A policy")
	templateB := createPromptTemplate(t, service, &agentB, "shared-policy", "agent B policy")

	assert.NotEqual(t, templateA.ID, templateB.ID)
	assert.NotEqual(t, global.ID, templateA.ID)

	effectiveA, err := repo.FindEffectiveByAgentAndSlug(ctx, agentA, "shared-policy")
	require.NoError(t, err)
	assert.Equal(t, "agent A policy", effectiveA.Content)
	assert.Equal(t, &agentA, effectiveA.AgentID)

	effectiveB, err := repo.FindEffectiveByAgentAndSlug(ctx, agentB, "shared-policy")
	require.NoError(t, err)
	assert.Equal(t, "agent B policy", effectiveB.Content)
	assert.Equal(t, &agentB, effectiveB.AgentID)

	listedA, total, err := repo.ListByAgent(ctx, agentA, pagination.PageRequest{Page: 0, Size: 10})
	require.NoError(t, err)
	assert.EqualValues(t, 2, total)
	assert.ElementsMatch(t, []string{"global policy", "agent A policy"}, promptTemplateContents(listedA))

	_, err = service.Create(ctx, &agentA, prompttemplate.CreateRequest{
		Name:    "Duplicate agent A policy",
		Slug:    "shared-policy",
		Content: "must be rejected",
	})
	require.ErrorIs(t, err, prompttemplate.ErrDuplicateSlug)
}

func setupPromptTemplateIsolationSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	schema := "ah_" + promptTemplateIsolationTenant

	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS pgcrypto")
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "SET search_path TO "+schema)
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "CREATE TABLE agent (id UUID PRIMARY KEY)")
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "CREATE TABLE agent_memory (agent_id UUID NOT NULL REFERENCES agent (id))")
	require.NoError(t, err)

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migrationPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations", "schemas", "000012_skills_memory_permissions.up.sql")
	migration, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), string(migration))
	require.NoError(t, err)

	return pool, tenant.NewContext(context.Background(), promptTemplateIsolationTenant)
}

func seedPromptTemplateAgents(t *testing.T, pool *pgxpool.Pool, agentIDs ...uuid.UUID) {
	t.Helper()
	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), "SET search_path TO ah_"+promptTemplateIsolationTenant)
	require.NoError(t, err)
	for _, agentID := range agentIDs {
		_, err = conn.Exec(context.Background(), "INSERT INTO agent (id) VALUES ($1)", agentID)
		require.NoError(t, err)
	}
}

func createPromptTemplate(t *testing.T, service *prompttemplate.Service, agentID *uuid.UUID, slug, content string) prompttemplate.Response {
	t.Helper()
	created, err := service.Create(tenant.NewContext(context.Background(), promptTemplateIsolationTenant), agentID, prompttemplate.CreateRequest{
		Name:    slug + " template",
		Slug:    slug,
		Content: content,
	})
	require.NoError(t, err)
	return created
}

func promptTemplateContents(templates []prompttemplate.PromptTemplate) []string {
	contents := make([]string, len(templates))
	for index, template := range templates {
		contents[index] = template.Content
	}
	return contents
}
