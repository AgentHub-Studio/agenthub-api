package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/trigger"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/workloadidentity"
)

type stubTriggerRunCompletionRepository struct {
	sessionID uuid.UUID
	status    trigger.RunStatus
	turns     *int
	tokens    *int
	errMsg    *string
	err       error
}

func TestNewMCPClient_WiresCircuitBreaker(t *testing.T) {
	client, err := newMCPClient(&config.Config{
		KeycloakBaseURL:    "https://keycloak.test",
		MCPRuntimeURL:      "http://mcp-runtime.test",
		MCPRuntimeClientID: "agenthub-api",
	}, stubWorkloadCredentialResolver{})

	require.NoError(t, err)
	assert.IsType(t, &agentic.CircuitBreakerMCPClient{}, client)
}

func (s *stubTriggerRunCompletionRepository) CompleteRunBySession(_ context.Context, sessionID uuid.UUID, status trigger.RunStatus, turns, tokens *int, errMsg *string) error {
	s.sessionID = sessionID
	s.status = status
	s.turns = turns
	s.tokens = tokens
	s.errMsg = errMsg
	return s.err
}

func TestCompleteTriggerRunForChatCompletion(t *testing.T) {
	sessionID := uuid.New()
	cases := []struct {
		name             string
		chatStatus       chat.ChatRunStatus
		turns            int
		tokens           int
		errMsg           string
		wantStatus       trigger.RunStatus
		wantTurns        int
		wantTokens       int
		wantUsage        bool
		wantErrorMessage bool
	}{
		{
			name:       "completed keeps persisted usage",
			chatStatus: chat.ChatRunStatusCompleted,
			turns:      3,
			tokens:     18,
			wantStatus: trigger.RunStatusCompleted,
			wantTurns:  3,
			wantTokens: 18,
			wantUsage:  true,
		},
		{
			name:             "cancelled is recorded as failed with reason",
			chatStatus:       chat.ChatRunStatusCancelled,
			errMsg:           "cancelled by user",
			wantStatus:       trigger.RunStatusFailed,
			wantErrorMessage: true,
		},
		{
			name:             "failed keeps error and zero values absent",
			chatStatus:       chat.ChatRunStatusFailed,
			errMsg:           "provider rejected request",
			wantStatus:       trigger.RunStatusFailed,
			wantErrorMessage: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &stubTriggerRunCompletionRepository{}

			completeTriggerRunForChatCompletion(context.Background(), repo, sessionID, tc.chatStatus, tc.turns, tc.tokens, tc.errMsg)

			assert.Equal(t, sessionID, repo.sessionID)
			assert.Equal(t, tc.wantStatus, repo.status)
			if tc.wantUsage {
				require.NotNil(t, repo.turns)
				require.NotNil(t, repo.tokens)
				assert.Equal(t, tc.wantTurns, *repo.turns)
				assert.Equal(t, tc.wantTokens, *repo.tokens)
			} else {
				assert.Nil(t, repo.turns)
				assert.Nil(t, repo.tokens)
			}
			if tc.wantErrorMessage {
				require.NotNil(t, repo.errMsg)
				assert.Equal(t, tc.errMsg, *repo.errMsg)
			} else {
				assert.Nil(t, repo.errMsg)
			}
		})
	}
}

// --- agentConfigAdapter tests ---

type stubAgentRepo struct {
	agents map[uuid.UUID]agent.Agent
	err    error
}

func (s *stubAgentRepo) FindAll(_ context.Context, _ agent.AgentStatus, _ string, _ pagination.PageRequest) ([]agent.Agent, int64, error) {
	return nil, 0, nil
}

func (s *stubAgentRepo) FindByID(_ context.Context, id uuid.UUID) (agent.Agent, error) {
	if s.err != nil {
		return agent.Agent{}, s.err
	}
	a, ok := s.agents[id]
	if !ok {
		return agent.Agent{}, agent.ErrNotFound
	}
	return a, nil
}

func (s *stubAgentRepo) Create(_ context.Context, a agent.Agent) (agent.Agent, error) {
	return a, nil
}

func (s *stubAgentRepo) Update(_ context.Context, a agent.Agent) (agent.Agent, error) {
	return a, nil
}

func (s *stubAgentRepo) Delete(_ context.Context, _ uuid.UUID) error { return nil }

func (s *stubAgentRepo) UpdateStatus(_ context.Context, id uuid.UUID, _ agent.AgentStatus) (agent.Agent, error) {
	return s.agents[id], nil
}

func (s *stubAgentRepo) CountPublishedWithoutProvider(_ context.Context) (int64, error) {
	return 0, nil
}

func TestAgentConfigAdapter_GetAgentForRun_Success(t *testing.T) {
	agentID := uuid.New()
	systemPrompt := "You are a helpful assistant."
	modelCfg := json.RawMessage(`{"provider":"anthropic","model":"claude-sonnet-4-20250514"}`)

	repo := &stubAgentRepo{
		agents: map[uuid.UUID]agent.Agent{
			agentID: {
				ID:               agentID,
				Name:             "Test Agent",
				SystemPrompt:     &systemPrompt,
				ModelConfig:      modelCfg,
				InputProcessors:  []string{"upper_caser", "pii_redactor"},
				OutputProcessors: []string{"pii_redactor"},
			},
		},
	}

	adapter := &agentConfigAdapter{repo: repo}
	cfg, err := adapter.GetAgentForRun(context.Background(), agentID)

	require.NoError(t, err)
	assert.Equal(t, agentID, cfg.ID)
	assert.Equal(t, systemPrompt, cfg.SystemPrompt)
	assert.Equal(t, modelCfg, cfg.ModelConfig)
	assert.Equal(t, []string{"upper_caser", "pii_redactor"}, cfg.InputProcessors)
	assert.Equal(t, []string{"pii_redactor"}, cfg.OutputProcessors)
}

func TestAgentConfigAdapter_GetAgentForRun_NilSystemPrompt(t *testing.T) {
	agentID := uuid.New()
	repo := &stubAgentRepo{
		agents: map[uuid.UUID]agent.Agent{
			agentID: {ID: agentID, Name: "No Prompt"},
		},
	}

	adapter := &agentConfigAdapter{repo: repo}
	cfg, err := adapter.GetAgentForRun(context.Background(), agentID)

	require.NoError(t, err)
	assert.Equal(t, "", cfg.SystemPrompt)
}

func TestAgentConfigAdapter_GetAgentForRun_NotFound(t *testing.T) {
	adapter := &agentConfigAdapter{repo: &stubAgentRepo{agents: map[uuid.UUID]agent.Agent{}}}
	_, err := adapter.GetAgentForRun(context.Background(), uuid.New())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "agent config")
}

// --- tenant lister adapter tests ---

type stubTenantRepo struct {
	tenants []tenant.Tenant
	err     error
}

func (s *stubTenantRepo) Create(_ context.Context, t tenant.Tenant) (tenant.Tenant, error) {
	return t, nil
}

func (s *stubTenantRepo) FindByID(_ context.Context, id string) (tenant.Tenant, error) {
	for _, t := range s.tenants {
		if t.ID == id {
			return t, nil
		}
	}
	return tenant.Tenant{}, tenant.ErrNotFound
}

func (s *stubTenantRepo) FindAll(_ context.Context, _ pagination.PageRequest) ([]tenant.Tenant, int64, error) {
	if s.err != nil {
		return nil, 0, s.err
	}
	return s.tenants, int64(len(s.tenants)), nil
}

func (s *stubTenantRepo) Exists(_ context.Context, id string) (bool, error) {
	for _, t := range s.tenants {
		if t.ID == id {
			return true, nil
		}
	}
	return false, nil
}

func (s *stubTenantRepo) UpdateStatus(_ context.Context, _ string, _ tenant.Status) error {
	return nil
}

func (s *stubTenantRepo) UpdateName(_ context.Context, _ string, _ string) error {
	return nil
}

func (s *stubTenantRepo) Delete(_ context.Context, _ string) error {
	return nil
}

func TestTenantListerAdapters_ListOnlyActiveTenants(t *testing.T) {
	repo := &stubTenantRepo{tenants: []tenant.Tenant{
		{ID: "active-a", Status: tenant.StatusActive},
		{ID: "provisioning-b", Status: tenant.StatusProvisioning},
		{ID: "failed-b", Status: tenant.StatusProvisioningFailed},
		{ID: "active-c", Status: tenant.StatusActive},
	}}

	triggerIDs, err := (&triggerTenantListerAdapter{repo: repo}).ListAllIDs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"active-a", "active-c"}, triggerIDs)

	webhookIDs, err := (&webhookTenantListerAdapter{repo: repo}).ListAllIDs(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"active-a", "active-c"}, webhookIDs)
}

func TestAuditRetentionIntervalFromConfig(t *testing.T) {
	assert.Equal(t, 24*time.Hour, auditRetentionIntervalFromConfig(&config.Config{}))
	assert.Equal(t, 2*time.Second, auditRetentionIntervalFromConfig(&config.Config{AuditRetentionIntervalSecs: 2}))
}

// --- admin integration route wiring tests ---

type stubAdminIntegrationRegistrar struct{}

func (stubAdminIntegrationRegistrar) RegisterRoutes(r chi.Router) {
	r.Get("/api/integrations", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/api/integrations/database/test", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Post("/api/integrations/database", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
	r.Get("/api/integrations/database/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Put("/api/integrations/database/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Patch("/api/integrations/database/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	r.Delete("/api/integrations/database/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func TestRegisterAdminIntegrationRoutesRequiresAdminRole(t *testing.T) {
	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/integrations?type=DATABASE_QUERY"},
		{http.MethodPost, "/api/integrations/database/test"},
		{http.MethodPost, "/api/integrations/database"},
		{http.MethodGet, "/api/integrations/database/00000000-0000-0000-0000-000000000001"},
		{http.MethodPut, "/api/integrations/database/00000000-0000-0000-0000-000000000001"},
		{http.MethodPatch, "/api/integrations/database/00000000-0000-0000-0000-000000000001"},
		{http.MethodDelete, "/api/integrations/database/00000000-0000-0000-0000-000000000001"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			r := chi.NewRouter()
			r.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					next.ServeHTTP(w, req.WithContext(middleware.ContextWithRoles(req.Context(), "user")))
				})
			})
			registerAdminIntegrationRoutes(r, stubAdminIntegrationRegistrar{})

			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "missing required role")
		})
	}
}

func TestRegisterAdminIntegrationRoutesAllowsAdminRole(t *testing.T) {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(middleware.ContextWithRoles(req.Context(), "admin")))
		})
	})
	registerAdminIntegrationRoutes(r, stubAdminIntegrationRegistrar{})

	req := httptest.NewRequest(http.MethodPost, "/api/integrations/database/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

type stubMarketplaceRouteRegistrar struct{}

func (stubMarketplaceRouteRegistrar) RegisterReadRoutes(r chi.Router) {
	r.Get("/api/marketplace/listings", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
}

func (stubMarketplaceRouteRegistrar) RegisterWriteRoutes(r chi.Router) {
	r.Post("/api/marketplace/listings", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})
}

func TestMarketplaceReadRoutesArePublicAndWritesRequireJWT(t *testing.T) {
	r := chi.NewRouter()
	chain := middleware.New("", nil)
	registrar := stubMarketplaceRouteRegistrar{}
	registerMarketplaceReadRoutes(r, chain, registrar)
	registerMarketplaceWriteRoutes(r, chain, registrar)

	read := httptest.NewRecorder()
	r.ServeHTTP(read, httptest.NewRequest(http.MethodGet, "/api/marketplace/listings", nil))
	assert.Equal(t, http.StatusNoContent, read.Code)
	assert.Equal(t, "no-store", read.Header().Get("Cache-Control"))

	write := httptest.NewRecorder()
	r.ServeHTTP(write, httptest.NewRequest(http.MethodPost, "/api/marketplace/listings", nil))
	assert.Equal(t, http.StatusUnauthorized, write.Code)
}

// --- buildDefaultChatModel tests ---

func TestBuildDefaultChatModel_NoProviders(t *testing.T) {
	// Unset all provider env vars.
	for _, key := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY"} {
		t.Setenv(key, "")
	}
	// Reset Ollama to default (which is skipped).
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434")

	model := buildDefaultChatModel()
	assert.Nil(t, model)
}

func TestBuildDefaultChatModel_Anthropic(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	model := buildDefaultChatModel()
	require.NotNil(t, model)
	assert.Equal(t, "anthropic", model.GetProviderName())
}

func TestBuildDefaultChatModel_OpenAI(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "test-openai-key")
	t.Setenv("OPENROUTER_API_KEY", "")

	model := buildDefaultChatModel()
	require.NotNil(t, model)
	assert.Equal(t, "openai", model.GetProviderName())
}

func TestBuildDefaultChatModel_Ollama(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("OLLAMA_BASE_URL", "http://custom-ollama:11434")

	model := buildDefaultChatModel()
	require.NotNil(t, model)
	assert.Equal(t, "ollama", model.GetProviderName())
}

func TestBuildDefaultChatModel_OpenRouter(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434")
	t.Setenv("OPENROUTER_API_KEY", "test-or-key")

	model := buildDefaultChatModel()
	require.NotNil(t, model)
	assert.Equal(t, "openrouter", model.GetProviderName())
}

func TestBuildDefaultChatModel_PriorityOrder(t *testing.T) {
	// When multiple providers are set, Anthropic takes priority.
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
	t.Setenv("OPENAI_API_KEY", "openai-key")
	t.Setenv("OPENROUTER_API_KEY", "or-key")

	model := buildDefaultChatModel()
	require.NotNil(t, model)
	assert.Equal(t, "anthropic", model.GetProviderName())
}

// --- buildAgenticRunner tests ---

func TestBuildAgenticRunner_ReturnsRunnerWithoutEnvProvider(t *testing.T) {
	// Ensure no providers are configured.
	for _, key := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "OPENROUTER_API_KEY"} {
		t.Setenv(key, "")
	}
	t.Setenv("OLLAMA_BASE_URL", "http://localhost:11434")

	// The runner is still constructed; provider resolution may happen later via settings.
	runner := buildAgenticRunner(
		&config.Config{SkillRuntimeURL: "http://localhost:8083"},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	assert.NotNil(t, runner)
}

func TestBuildAgenticRunner_ReturnsRunnerWhenProviderConfigured(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "test-key")

	// Pass nil repos — buildAgenticRunner only needs the chatModel check to pass;
	// the repos are wrapped in adapters but not called at construction time.
	runner := buildAgenticRunner(
		&config.Config{SkillRuntimeURL: "http://localhost:8083"},
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	assert.NotNil(t, runner)
}

type stubSettingsRepo struct {
	values map[string]settings.Setting
}

type stubWorkloadCredentialResolver struct{}

func (stubWorkloadCredentialResolver) Resolve(context.Context, string) (workloadidentity.Credential, error) {
	return workloadidentity.Credential{ClientID: "agenthub-api", ClientSecret: "test-secret"}, nil
}

func (s stubSettingsRepo) FindAll(context.Context) ([]settings.Setting, error) {
	result := make([]settings.Setting, 0, len(s.values))
	for _, setting := range s.values {
		result = append(result, setting)
	}
	return result, nil
}

func (s stubSettingsRepo) FindByKey(_ context.Context, key string) (settings.Setting, error) {
	setting, ok := s.values[key]
	if !ok {
		return settings.Setting{}, settings.ErrNotFound
	}
	return setting, nil
}

func (s stubSettingsRepo) Upsert(_ context.Context, setting settings.Setting) (settings.Setting, error) {
	s.values[setting.Key] = setting
	return setting, nil
}

func (s stubSettingsRepo) Delete(_ context.Context, key string) error {
	if _, ok := s.values[key]; !ok {
		return settings.ErrNotFound
	}
	delete(s.values, key)
	return nil
}

func settingString(key, value string, updatedAt time.Time) settings.Setting {
	raw, _ := json.Marshal(value)
	return settings.Setting{Key: key, Value: raw, UpdatedAt: updatedAt}
}

func TestSettingsChatModelFactory_BuildUsesLatestDefaultProviderSetting(t *testing.T) {
	base := time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC)
	factory := &settingsChatModelFactory{settingsRepo: stubSettingsRepo{
		values: map[string]settings.Setting{
			"llm.defaultProvider":     settingString("llm.defaultProvider", "openrouter", base),
			"general.defaultProvider": settingString("general.defaultProvider", "openai", base.Add(time.Minute)),
			"openrouter.apiKey":       settingString("openrouter.apiKey", "test-openrouter-key", base),
			"openai.apiKey":           settingString("openai.apiKey", "test-openai-key", base),
		},
	}}

	model, err := factory.Build(context.Background(), "", "")

	require.NoError(t, err)
	require.NotNil(t, model)
	assert.Equal(t, "openai", model.GetProviderName())
	assert.Equal(t, "openai", factory.ResolveDefaultProvider(context.Background()))
}

// Prevent "imported and not used" for os.
var _ = os.Getenv
