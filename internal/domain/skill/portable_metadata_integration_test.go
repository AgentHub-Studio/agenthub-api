//go:build integration

package skill_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const portableMetadataTenant = "skillportablemetadata"

func TestIntegration_SkillMDPortableMetadataPersistsAcrossHTTPImportUpdateAndExport(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := setupPortableMetadataSchema(t, pool)
	adminCtx := middleware.ContextWithRoles(ctx, "admin")

	repo := skill.NewRepository(pool)
	handler := skill.NewHandler(skill.NewService(repo)).WithRepository(repo)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	importBody := `---
version: "1"
name: Incident Research
slug: incident-research
description: Investigates production incidents.
category: OPERATIONS
context_mode: fork
model_overrides:
  default: claude-sonnet-4-6
effort_level: high
associated_agents:
  - incident-commander
dynamic_hooks:
  - pre_tool_use
  - post_tool_failure
---
Investigate the incident, preserve evidence, and report the next safe action.
`

	importRequest := httptest.NewRequest(http.MethodPost, "/api/skills/import", bytes.NewBufferString(importBody)).WithContext(adminCtx)
	importRequest.Header.Set("Content-Type", "text/markdown")
	importResponse := httptest.NewRecorder()
	router.ServeHTTP(importResponse, importRequest)
	require.Equal(t, http.StatusCreated, importResponse.Code, importResponse.Body.String())

	var created skill.Response
	require.NoError(t, json.Unmarshal(importResponse.Body.Bytes(), &created))
	assert.Equal(t, map[string]string{"default": "claude-sonnet-4-6"}, created.ModelOverrides)
	assert.Equal(t, "high", created.EffortLevel)
	assert.Equal(t, []string{"incident-commander"}, created.AssociatedAgents)
	assert.Equal(t, []string{"pre_tool_use", "post_tool_failure"}, created.DynamicHooks)

	assertPortableExport(t, router, adminCtx, created.ID.String(), "claude-sonnet-4-6", "high", []string{"incident-commander"}, []string{"pre_tool_use", "post_tool_failure"})

	updateBody, err := json.Marshal(skill.UpdateRequest{
		ModelOverrides:   map[string]string{"default": "claude-opus-4-6"},
		EffortLevel:      "medium",
		AssociatedAgents: []string{"incident-commander", "sre-reviewer"},
		DynamicHooks:     []string{"post_tool_use", "run_end"},
	})
	require.NoError(t, err)
	updateRequest := httptest.NewRequest(http.MethodPatch, "/api/skills/"+created.ID.String(), bytes.NewReader(updateBody)).WithContext(adminCtx)
	updateRequest.Header.Set("Content-Type", "application/json")
	updateResponse := httptest.NewRecorder()
	router.ServeHTTP(updateResponse, updateRequest)
	require.Equal(t, http.StatusOK, updateResponse.Code, updateResponse.Body.String())

	assertPortableExport(t, router, adminCtx, created.ID.String(), "claude-opus-4-6", "medium", []string{"incident-commander", "sre-reviewer"}, []string{"post_tool_use", "run_end"})
}

func TestIntegration_SkillRepositoryDefaultsOmittedPortableMetadataCollections(t *testing.T) {
	pool := testutil.NewPostgresContainer(t)
	ctx := setupPortableMetadataSchema(t, pool)
	repo := skill.NewRepository(pool)

	created, err := repo.Create(ctx, skill.Skill{
		Name:        "No portable metadata",
		Slug:        "no-portable-metadata",
		Category:    "OPERATIONS",
		ContextMode: "inline",
	})
	require.NoError(t, err)
	assert.Empty(t, created.AllowedTools)
	assert.Empty(t, created.RequiredRoles)
	assert.Empty(t, created.AssociatedAgents)
	assert.Empty(t, created.DynamicHooks)

	updated, err := repo.Update(ctx, created.ID, skill.UpdateRequest{
		Name:         created.Name,
		Description:  created.Description,
		Instructions: created.Instructions,
		Category:     created.Category,
		ContextMode:  created.ContextMode,
	})
	require.NoError(t, err)
	assert.Empty(t, updated.AllowedTools)
	assert.Empty(t, updated.RequiredRoles)
	assert.Empty(t, updated.AssociatedAgents)
	assert.Empty(t, updated.DynamicHooks)
}

func assertPortableExport(t *testing.T, router http.Handler, ctx context.Context, id, model, effort string, agents, hooks []string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "/api/skills/"+id+"/export", nil).WithContext(ctx)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	reparsed, err := skill.ParseSkillMD(response.Body.String())
	require.NoError(t, err)
	assert.Equal(t, model, reparsed.ModelOverrides["default"])
	assert.Equal(t, effort, reparsed.EffortLevel)
	assert.Equal(t, agents, reparsed.AssociatedAgents)
	assert.Equal(t, hooks, reparsed.DynamicHooks)
}

func setupPortableMetadataSchema(t *testing.T, pool *pgxpool.Pool) context.Context {
	t.Helper()
	// This deliberately starts with the pre-000081 skill table shape, then
	// executes the production migration before exercising the HTTP contract.
	// The test proves the new columns are both migratable and persisted.
	schema := "ah_" + portableMetadataTenant
	conn, err := pool.Acquire(context.Background())
	require.NoError(t, err)
	defer conn.Release()

	_, err = conn.Exec(context.Background(), "CREATE EXTENSION IF NOT EXISTS pgcrypto")
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), `
		CREATE TABLE `+schema+`.skill (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			name VARCHAR(255) NOT NULL,
			slug VARCHAR(255) NOT NULL UNIQUE,
			description TEXT,
			category VARCHAR(100),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			instructions TEXT,
			allowed_tools TEXT[] NOT NULL DEFAULT '{}',
			required_roles TEXT[] NOT NULL DEFAULT '{}',
			disable_model_invocation BOOLEAN NOT NULL DEFAULT FALSE,
			context_mode VARCHAR(10) NOT NULL DEFAULT 'inline',
			when_to_use TEXT,
			argument_hint TEXT,
			should_defer BOOLEAN NOT NULL DEFAULT FALSE
		)`)
	require.NoError(t, err)

	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	migration, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "migrations", "schemas", "000081_skill_portable_metadata.up.sql"))
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), "SET search_path TO "+schema)
	require.NoError(t, err)
	_, err = conn.Exec(context.Background(), string(migration))
	require.NoError(t, err)

	return tenant.NewContext(context.Background(), portableMetadataTenant)
}
