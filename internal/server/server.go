package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/approval"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chatsession"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/document"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/execution"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/llmpreset"
	mkplInstallation "github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/installation"
	mkplListing "github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/listing"
	mkplReview "github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/review"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/memory"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/metrics"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/prompttemplate"
	regDependency "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/dependency"
	regInstallation "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/installation"
	regPackage "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	regVersion "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/version"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/search"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/user"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/vpnresource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/webhook"
	apikc "github.com/AgentHub-Studio/agenthub-api/internal/keycloak"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/anthropic"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/ollama"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openai"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai/provider/openrouter"
)

// Server is the HTTP server for agenthub-api.
type Server struct {
	router http.Handler
	pool   *pgxpool.Pool
}

// New creates a new Server with all routes mounted.
func New(cfg *config.Config, pool *pgxpool.Pool) *Server {
	s := &Server{pool: pool}
	chain := middleware.New(cfg.KeycloakBaseURL, cfg.CORSOrigins)

	// Build Keycloak user client for user management.
	keycloakCfg := user.KeycloakClientConfig{
		BaseURL:        cfg.KeycloakBaseURL,
		AdminUsername:  cfg.KeycloakAdmin.AdminUsername,
		AdminPassword:  cfg.KeycloakAdmin.AdminPassword,
		AdminClientID:  cfg.KeycloakAdmin.AdminClientID,
		AdminRealm:     cfg.KeycloakAdmin.AdminRealm,
		FrontendClient: cfg.KeycloakAdmin.FrontendClient,
	}
	keycloakClient := user.NewKeycloakUserClient(keycloakCfg)

	// Build Keycloak realm provisioner for tenant creation.
	provisioner := apikc.NewProvisioner(apikc.Config{
		BaseURL:        cfg.KeycloakBaseURL,
		AdminUsername:  cfg.KeycloakAdmin.AdminUsername,
		AdminPassword:  cfg.KeycloakAdmin.AdminPassword,
		AdminClientID:  cfg.KeycloakAdmin.AdminClientID,
		AdminRealm:     cfg.KeycloakAdmin.AdminRealm,
		FrontendClient: cfg.KeycloakAdmin.FrontendClient,
	})

	// Instantiate domain handlers.
	tenantHandler := tenant.NewHandler(tenant.NewService(tenant.NewRepository(pool), provisioner))
	userHandler := user.NewHandler(user.NewService(keycloakClient))
	settingsRepo := settings.NewRepository(pool)
	settingsHandler := settings.NewHandler(settings.NewService(settingsRepo))
	llmpresetHandler := llmpreset.NewHandler(llmpreset.NewService(llmpreset.NewRepository(pool)))
	agentRepo := agent.NewRepository(pool)
	agentHandler := agent.NewHandler(agent.NewService(agentRepo))
	agentVersionHandler := agent.NewVersionHandler(agent.NewVersionService(agentRepo, agent.NewVersionRepository(pool)))
	agentBindingHandler := agent.NewBindingHandler(agentRepo, agent.NewBindingRepository(pool))
	skillRepo := skill.NewRepository(pool)
	skillHandler := skill.NewHandler(skill.NewService(skillRepo))
	datasourceSvc := datasource.NewService(datasource.NewRepository(pool))
	toolRepo := tool.NewRepository(pool)
	toolSvc := tool.NewService(toolRepo).
		WithSettings(&toolSettingsAdapter{repo: settingsRepo}).
		WithDatasource(&toolDatasourceAdapter{svc: datasourceSvc}, tenantctx.FromContext)
	toolHandler := tool.NewHandler(toolSvc)
	memoryHandler := memory.NewHandler(memory.NewService(memory.NewRepository(pool)))
	promptTemplateHandler := prompttemplate.NewHandler(prompttemplate.NewService(prompttemplate.NewRepository(pool)))
	executionHandler := execution.NewHandler(execution.NewService(execution.NewRepository(pool)))
	webhookHandler := webhook.NewHandler(webhook.NewService(webhook.NewRepository(pool)))
	oauthSvc := oauth.NewServiceWithEncryption(oauth.NewRepository(pool), cfg.OAuthEncryptionKey)
	oauthHandler := oauth.NewHandler(oauthSvc)
	auditHandler := audit.NewHandler(audit.NewService(audit.NewRepository(pool)))
	metricsHandler := metrics.NewHandler(metrics.NewService(metrics.NewRepository(pool)))
	vpnSvc := vpnresource.NewService(vpnresource.NewRepository(pool))
	vpnHandler := vpnresource.NewHandler(vpnSvc)
	datasourceHandler := datasource.NewHandler(datasourceSvc)
	searchHandler := search.NewHandler(search.NewServiceWithPool(pool))
	mcpSvc := mcp.NewService(mcp.NewRepository(pool))
	mcpSvc.WithOAuthService(oauthSvc)
	integrationHandler := integration.NewHandler(integration.NewService(
		toolSvc,
		datasourceSvc,
		mcpSvc,
		vpnSvc,
	).WithHTTPManagement(skill.NewService(skillRepo), toolSvc, integration.NewRepository(pool)))
	kbRepo := knowledgebase.NewRepository(pool)

	// Build agentic runner and wire it into the chat service.
	chatRepo := chat.NewRepository(pool)
	sessionRunner := buildAgenticRunner(cfg, pool, chatRepo, agentRepo, skillRepo, kbRepo, toolRepo, settingsRepo, mcpSvc.Repository(), integration.NewService(toolSvc, datasourceSvc, mcpSvc, vpnSvc))

	var chatExecutor *chat.AsyncExecutor
	if cfg.RabbitMQURL != "" {
		chatExecutor = chat.NewAsyncExecutor(chatRepo, sessionRunner, cfg.RabbitMQURL)
		go func() {
			if err := chatExecutor.StartWorker(context.Background()); err != nil {
				slog.Error("rabbitmq: chat worker failed", "err", err)
			}
		}()
	}

	chatHandler := chat.NewHandler(chat.NewService(chatRepo, sessionRunner), chatExecutor)
	var docStorage document.StorageClient
	if cfg.MinIO.IsConfigured() {
		ds, err := document.NewMinIOStorageClient(
			cfg.MinIO.Endpoint,
			cfg.MinIO.AccessKeyID,
			cfg.MinIO.SecretAccessKey,
			cfg.MinIO.UseSSL,
			cfg.MinIO.Region,
			cfg.MinIO.DocumentsBucket,
		)
		if err != nil {
			slog.Warn("minio: failed to create document storage client, using noop", "err", err)
			docStorage = &document.NoopStorageClient{}
		} else {
			docStorage = ds
			slog.Info("minio: document storage configured", "bucket", cfg.MinIO.DocumentsBucket)
		}
	} else {
		slog.Warn("minio: MINIO_ENDPOINT not set, document uploads will not be stored")
		docStorage = &document.NoopStorageClient{}
	}
	// Build document event publisher — optional; requires RABBITMQ_URL.
	var docPublisher document.EventPublisher = &document.NoopEventPublisher{}
	if cfg.RabbitMQURL != "" {
		pub, err := document.NewRabbitMQEventPublisher(cfg.RabbitMQURL)
		if err != nil {
			slog.Warn("rabbitmq: failed to connect for document events, using noop", "err", err)
		} else {
			docPublisher = pub
			slog.Info("rabbitmq: document event publisher connected")
		}
	}
	documentHandler := document.NewHandler(document.NewService(document.NewRepository(pool), docStorage, docPublisher))
	knowledgebaseHandler := knowledgebase.NewHandler(knowledgebase.NewService(kbRepo))
	mcpHandler := mcp.NewHandler(mcpSvc)
	approvalHandler := approval.NewHandler(approval.NewService(approval.NewRepository(pool)))

	// Marketplace handlers.
	mkplListingHandler := mkplListing.NewHandler(mkplListing.NewService(mkplListing.NewRepository(pool)))
	mkplReviewHandler := mkplReview.NewHandler(mkplReview.NewService(mkplReview.NewRepository(pool), mkplListing.NewRepository(pool)))
	mkplInstallationHandler := mkplInstallation.NewHandler(mkplInstallation.NewService(mkplInstallation.NewRepository(pool)))

	// Registry handlers — storage backend selected based on MinIO config.
	var regStorage regInstallation.StorageBackend
	if cfg.MinIO.IsConfigured() {
		storage, err := regInstallation.NewMinIOStorageFromConfig(regInstallation.MinIOConfig{
			Endpoint:        cfg.MinIO.Endpoint,
			AccessKeyID:     cfg.MinIO.AccessKeyID,
			SecretAccessKey: cfg.MinIO.SecretAccessKey,
			UseSSL:          cfg.MinIO.UseSSL,
			Region:          cfg.MinIO.Region,
			Bucket:          cfg.MinIO.Bucket,
		})
		if err != nil {
			slog.Warn("minio: failed to create storage client, using noop backend", "err", err)
			regStorage = &regInstallation.NoopStorage{}
		} else {
			regStorage = storage
			slog.Info("minio: storage client configured", "bucket", cfg.MinIO.Bucket)
		}
	} else {
		slog.Warn("minio: MINIO_ENDPOINT not set, package uploads will fail")
		regStorage = &regInstallation.NoopStorage{}
	}
	pkgRepo := regPackage.NewRepository(pool)
	regPackageHandler := regPackage.NewHandler(regPackage.NewService(pkgRepo))
	regVersionHandler := regVersion.NewHandler(regVersion.NewService(regVersion.NewRepository(pool), pkgRepo))
	regDependencyHandler := regDependency.NewHandler(regDependency.NewService(regDependency.NewRepository(pool), pkgRepo))
	regInstallationHandler := regInstallation.NewHandler(regInstallation.NewService(regInstallation.NewRepository(pool), regStorage))

	r := chi.NewRouter()
	r.Use(chiMiddleware.RealIP)

	// CORS must be at root level so OPTIONS preflight requests are handled
	// before chi's router can return 405 Method Not Allowed.
	r.Use(chain.CORSHandler())
	r.Options("/*", func(w http.ResponseWriter, r *http.Request) {})

	// Health endpoints — no auth.
	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)

	// Chat widget session relay — no JWT required (Angular passes token explicitly).
	chatSessionHandler := chatsession.NewHandler()
	chatSessionHandler.RegisterRoutes(r)

	// Public routes — no JWT required.
	r.Group(func(r chi.Router) {
		for _, m := range chain.Public() {
			r.Use(m)
		}
		tenantHandler.RegisterPublicRoutes(r)
		webhookHandler.RegisterPublicRoutes(r)
		regPackageHandler.RegisterPublicRoutes(r)
		regInstallationHandler.RegisterPublicRoutes(r)
	})

	// Protected routes — JWT required.
	r.Group(func(r chi.Router) {
		for _, m := range chain.Protected() {
			r.Use(m)
		}
		userHandler.RegisterProtectedRoutes(r)
		settingsHandler.RegisterProtectedRoutes(r)
		llmpresetHandler.RegisterProtectedRoutes(r)
		agentHandler.RegisterRoutes(r)
		agentVersionHandler.RegisterVersionRoutes(r)
		agentBindingHandler.RegisterBindingRoutes(r)
		skillHandler.RegisterRoutes(r)
		toolHandler.RegisterRoutes(r)
		memoryHandler.RegisterRoutes(r)
		promptTemplateHandler.RegisterRoutes(r)
		executionHandler.RegisterRoutes(r)
		webhookHandler.RegisterRoutes(r)
		r.Mount("/api/oauth-credentials", oauthHandler.Routes())
		r.Mount("/api/audit-logs", auditHandler.Routes())
		r.Mount("/api/metrics", metricsHandler.Routes())
		r.Route("/api/agents/{agentId}/metrics", func(r chi.Router) {
			r.Mount("/", metricsHandler.AgentRoutes())
		})
		r.Mount("/api/vpn-resources", vpnHandler.Routes())
		r.Mount("/api/datasources", datasourceHandler.Routes())
		// Proxy credentials endpoint — requires PROXY_SERVICE role.
		// ProxyServiceRequired is a pass-through while auth middleware is placeholder (dev mode).
		// Once agenthub-go-commons/auth is wired (PR #12), it enforces the role.
		r.With(middleware.ProxyServiceRequired).Mount("/api/proxy/datasources", datasourceHandler.ProxyRoutes())
		r.Mount("/api/search", searchHandler.Routes())
		chatHandler.RegisterRoutes(r)
		documentHandler.RegisterRoutes(r)
		knowledgebaseHandler.RegisterRoutes(r)
		integrationHandler.RegisterRoutes(r)
		mcpHandler.RegisterRoutes(r)
		approvalHandler.RegisterRoutes(r)
		// Marketplace
		mkplListingHandler.RegisterRoutes(r)
		mkplReviewHandler.RegisterRoutes(r)
		mkplInstallationHandler.RegisterRoutes(r)
		// Registry
		regPackageHandler.RegisterProtectedRoutes(r)
		regVersionHandler.RegisterRoutes(r)
		regDependencyHandler.RegisterRoutes(r)
		regInstallationHandler.RegisterProtectedRoutes(r)
	})

	s.router = r
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if err := s.pool.Ping(r.Context()); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "unavailable", "reason": "database unreachable"})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// --- tool service adapters ---

// toolSettingsAdapter adapts the settings.Repository to tool.SettingsReader.
type toolSettingsAdapter struct {
	repo settings.Repository
}

func (a *toolSettingsAdapter) FindSettingByKey(ctx context.Context, key string) ([]byte, error) {
	s, err := a.repo.FindByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	return []byte(s.Value), nil
}

// toolDatasourceAdapter adapts datasource.Service to tool.DatasourceReader.
type toolDatasourceAdapter struct {
	svc *datasource.Service
}

func (a *toolDatasourceAdapter) GetDatasourceCreds(ctx context.Context, tenantID string, id uuid.UUID) (tool.DatasourceCreds, error) {
	creds, err := a.svc.GetCredentials(ctx, tenantID, id)
	if err != nil {
		return tool.DatasourceCreds{}, err
	}
	return tool.DatasourceCreds{
		Type:     string(creds.Type),
		Host:     creds.Host,
		Port:     creds.Port,
		Database: creds.Database,
		User:     creds.User,
		Password: creds.Password,
	}, nil
}

type promptTemplateAdapter struct {
	repo *prompttemplate.Repository
}

func (a *promptTemplateAdapter) ResolvePromptTemplate(ctx context.Context, agentID uuid.UUID, slug string) (string, bool, error) {
	tpl, err := a.repo.FindEffectiveByAgentAndSlug(ctx, agentID, slug)
	if err != nil {
		if err == prompttemplate.ErrNotFound {
			return "", false, nil
		}
		return "", false, err
	}
	return tpl.Content, true, nil
}

// --- agentic wiring ---

// buildAgenticRunner creates the SessionRunner that powers the agentic chat loop.
// Provider credentials are resolved per-request from the tenant's settings table,
// so all configured providers (OpenAI, Anthropic, Ollama, OpenRouter) are available
// to agents regardless of environment variables.
func buildAgenticRunner(
	cfg *config.Config,
	pool *pgxpool.Pool,
	chatRepo chat.Repository,
	agentRepo agent.Repository,
	skillRepo skill.SkillRepository,
	kbRepo knowledgebase.Repository,
	toolRepo tool.ToolRepository,
	settingsRepo settings.Repository,
	mcpRepo mcp.Repository,
	integSvc *integration.Service,
) chat.SessionRunner {
	// Build an env-based fallback for agents that have no provider configured.
	// This keeps backward-compatibility with existing deployments that set env vars.
	fallback := buildDefaultChatModel()
	if fallback != nil {
		slog.Info("agentic: env-based fallback provider configured", "provider", fallback.GetProviderName())
	}

	factory := &settingsChatModelFactory{
		settingsRepo: settingsRepo,
		fallback:     fallback,
	}

	skillClient := agentic.NewSkillRuntimeClient(cfg.SkillRuntimeURL)
	promptTemplateRepo := prompttemplate.NewRepository(pool)
	promptBuilder := agentic.NewPromptBuilder(skillRepo, kbRepo, chatRepo, agentic.DefaultPromptConfig()).
		WithPromptTemplateResolver(&promptTemplateAdapter{repo: promptTemplateRepo})
	toolSchemaBuilder := agentic.NewToolSchemaBuilder(skillRepo, toolRepo, kbRepo)
	ctxManager := agentic.NewContextManager()
	hookRepo := agentic.NewHookRepository(pool)
	hookExecutor := agentic.NewHookExecutor(hookRepo)

	return agentic.NewSessionRunnerAdapterWithFactory(
		factory,
		skillClient,
		promptBuilder,
		toolSchemaBuilder,
		ctxManager,
		nil, // MemoryBridge — requires Embedder, wired later
		hookExecutor,
		chatRepo,
		&agentConfigAdapter{repo: agentRepo},
		agentRepo,
		skillRepo.(*skill.Repository),
		toolRepo.(*tool.Repository),
		integSvc,
		mcpRepo,
	)
}

// buildDefaultChatModel creates a ChatModel from environment variables as a fallback.
// Tries providers in order: Anthropic, OpenAI, Ollama, OpenRouter.
// Returns nil if no provider is configured.
func buildDefaultChatModel() ai.ChatModel {
	envCfg := ai.EnvConfigFromEnvironment()
	if envCfg.AnthropicAPIKey != "" {
		return anthropic.New(envCfg.AnthropicAPIKey, envCfg.AnthropicBaseURL)
	}
	if envCfg.OpenAIAPIKey != "" {
		return openai.New(envCfg.OpenAIAPIKey, envCfg.OpenAIBaseURL)
	}
	if envCfg.OllamaBaseURL != "" && envCfg.OllamaBaseURL != "http://localhost:11434" {
		return ollama.New(envCfg.OllamaBaseURL)
	}
	if envCfg.OpenRouterAPIKey != "" {
		return openrouter.New(envCfg.OpenRouterAPIKey, envCfg.OpenRouterBaseURL, "agenthub")
	}
	return nil
}

// agentConfigAdapter adapts agent.Repository to agentic.AgentConfigLoader.
type agentConfigAdapter struct {
	repo agent.Repository
}

func (a *agentConfigAdapter) GetAgentForRun(ctx context.Context, id uuid.UUID) (*chat.AgentRunConfig, error) {
	ag, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("agent config: %w", err)
	}

	systemPrompt := ""
	if ag.SystemPrompt != nil {
		systemPrompt = *ag.SystemPrompt
	}

	return &chat.AgentRunConfig{
		ID:              ag.ID,
		SystemPrompt:    systemPrompt,
		ModelConfig:     ag.ModelConfig,
		PermissionRules: ag.PermissionRules,
	}, nil
}
