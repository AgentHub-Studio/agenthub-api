//go:build integration

package agent_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
)

func TestIntegration_ExporterRedactsToolConfigSecrets(t *testing.T) {
	pool, ctx := setupVersionHandlerTenantSchema(t)
	agentRepo := agent.NewRepository(pool)
	bindingRepo := agent.NewBindingRepository(pool)
	skillRepo := skill.NewRepository(pool)
	toolRepo := tool.NewRepository(pool)
	agentSvc := agent.NewService(agentRepo, bindingRepo, skillRepo)
	exporter := agent.NewExporter(agentSvc, skillRepo, bindingRepo).WithToolRepo(toolRepo)

	createdAgent, err := agentRepo.Create(ctx, agent.Agent{
		ID:          uuid.New(),
		Name:        "bundle-redaction",
		Slug:        "bundle-redaction",
		Description: "integration agent",
		Status:      agent.StatusPublished,
		Config:      json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	createdSkill, err := skillRepo.Create(ctx, skill.Skill{
		Name:          "Webhook",
		Slug:          "webhook",
		Description:   "Calls webhook",
		Category:      "integration",
		Instructions:  "Call the webhook.",
		AllowedTools:  []string{},
		RequiredRoles: []string{},
	})
	require.NoError(t, err)
	require.NoError(t, bindingRepo.SyncSkills(ctx, createdAgent.ID, []uuid.UUID{createdSkill.ID}))

	createdTool, err := toolRepo.Create(ctx, tool.Tool{
		Name: "Webhook HTTP",
		Slug: "webhook-http",
		Type: tool.ToolTypeHTTP,
		Config: []byte(`{
			"url":"https://api.example.com/hooks",
			"auth_token":"Bearer bundle-secret",
			"authToken":"bundle-secret-2",
			"headers":{
				"Authorization":"Bearer header-secret",
				"X-API-Key":"header-key",
				"X-Safe":"ok"
			}
		}`),
	})
	require.NoError(t, err)
	_, err = toolRepo.BindToSkill(ctx, createdSkill.ID, tool.BindRequest{ToolID: createdTool.ID})
	require.NoError(t, err)

	persisted, err := toolRepo.GetByID(ctx, createdTool.ID)
	require.NoError(t, err)
	assert.Contains(t, string(persisted.Config), "bundle-secret")
	assert.Contains(t, string(persisted.Config), "header-secret")

	bundle, err := exporter.Export(ctx, createdAgent.ID)
	require.NoError(t, err)
	require.Len(t, bundle.Tools, 1)

	data, err := json.Marshal(bundle)
	require.NoError(t, err)
	body := string(data)
	assert.NotContains(t, body, "auth_token")
	assert.NotContains(t, body, "authToken")
	assert.NotContains(t, body, "bundle-secret")
	assert.NotContains(t, body, "bundle-secret-2")
	assert.NotContains(t, body, "Bearer header-secret")
	assert.NotContains(t, body, "header-key")
	assert.Contains(t, body, `"X-Safe":"ok"`)
}

func TestIntegration_ExporterRedactsNestedToolConfigSecrets(t *testing.T) {
	pool, ctx := setupVersionHandlerTenantSchema(t)
	agentRepo := agent.NewRepository(pool)
	bindingRepo := agent.NewBindingRepository(pool)
	skillRepo := skill.NewRepository(pool)
	toolRepo := tool.NewRepository(pool)
	agentSvc := agent.NewService(agentRepo, bindingRepo, skillRepo)
	exporter := agent.NewExporter(agentSvc, skillRepo, bindingRepo).WithToolRepo(toolRepo)

	createdAgent, err := agentRepo.Create(ctx, agent.Agent{
		ID:          uuid.New(),
		Name:        "bundle-nested-redaction",
		Slug:        "bundle-nested-redaction",
		Description: "integration agent",
		Status:      agent.StatusPublished,
		Config:      json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	createdSkill, err := skillRepo.Create(ctx, skill.Skill{
		Name:          "Nested Webhook",
		Slug:          "nested-webhook",
		Description:   "Calls nested webhook",
		Category:      "integration",
		Instructions:  "Call the nested webhook.",
		AllowedTools:  []string{},
		RequiredRoles: []string{},
	})
	require.NoError(t, err)
	require.NoError(t, bindingRepo.SyncSkills(ctx, createdAgent.ID, []uuid.UUID{createdSkill.ID}))

	createdTool, err := toolRepo.Create(ctx, tool.Tool{
		Name: "Nested Webhook HTTP",
		Slug: "nested-webhook-http",
		Type: tool.ToolTypeHTTP,
		Config: []byte(`{
			"url":"https://api.example.com/hooks",
			"auth":{
				"apiKey":"nested-bundle-key",
				"password":"nested-password",
				"safe":"kept"
			},
			"steps":[
				{"name":"safe-step","secret":"nested-secret"}
			],
			"metadata":{
				"headers":{
					"Authorization":"Bearer nested-header",
					"X-Safe":"ok"
				}
			}
		}`),
	})
	require.NoError(t, err)
	_, err = toolRepo.BindToSkill(ctx, createdSkill.ID, tool.BindRequest{ToolID: createdTool.ID})
	require.NoError(t, err)

	persisted, err := toolRepo.GetByID(ctx, createdTool.ID)
	require.NoError(t, err)
	assert.Contains(t, string(persisted.Config), "nested-bundle-key")
	assert.Contains(t, string(persisted.Config), "nested-secret")
	assert.Contains(t, string(persisted.Config), "Bearer nested-header")

	bundle, err := exporter.Export(ctx, createdAgent.ID)
	require.NoError(t, err)
	require.Len(t, bundle.Tools, 1)

	data, err := json.Marshal(bundle)
	require.NoError(t, err)
	body := string(data)
	assert.NotContains(t, body, `"apiKey"`)
	assert.NotContains(t, body, `"password"`)
	assert.NotContains(t, body, `"secret"`)
	assert.NotContains(t, body, "nested-bundle-key")
	assert.NotContains(t, body, "nested-password")
	assert.NotContains(t, body, "nested-secret")
	assert.NotContains(t, body, "Bearer nested-header")
	assert.Contains(t, body, `"safe":"kept"`)
	assert.Contains(t, body, `"name":"safe-step"`)
	assert.Contains(t, body, `"Authorization":"***"`)
	assert.Contains(t, body, `"X-Safe":"ok"`)
}
