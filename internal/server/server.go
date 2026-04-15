package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/abtest"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/auth"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/device"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agenttemplate"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/analytics"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/approval"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/channel"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	chatTask "github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/task"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chatsession"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/document"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/execution"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledge"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/pipeline"
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
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skilleval"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/trigger"
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
	commonsmigratemulti "github.com/AgentHub-Studio/agenthub-go-commons/database/multitenant"
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
	presetSeeder := llmpreset.NewSeeder(pool)
	tenantSvc := tenant.NewService(tenant.NewRepository(pool), provisioner, presetSeeder)

	// Wire the schema migrator so that new tenants get their PostgreSQL schema
	// created and migrated immediately upon provisioning (same path used on startup).
	tenantSvc.WithSchemaMigrator(tenantSchemaMigratorFunc(func(ctx context.Context, tenantID string) error {
		return commonsmigratemulti.MigrateTenant(ctx, pool, tenantID, "/migrations/schemas")
	}))

	tenantHandler := tenant.NewHandler(tenantSvc)
	userHandler := user.NewHandler(user.NewService(keycloakClient))
	settingsRepo := settings.NewRepository(pool)
	settingsHandler := settings.NewHandler(settings.NewService(settingsRepo))
	llmpresetHandler := llmpreset.NewHandler(llmpreset.NewService(llmpreset.NewRepository(pool)))
	agentRepo := agent.NewRepository(pool)
	skillRepo := skill.NewRepository(pool)
	agentBindingRepo := agent.NewBindingRepository(pool)
	auditSvc := audit.NewService(audit.NewRepository(pool))
	agentSvc := agent.NewServiceWithAudit(agentRepo, agentBindingRepo, skillRepo, auditSvc)
	promptTemplateRepo := prompttemplate.NewRepository(pool)
	agentHandler := agent.NewHandler(agentSvc).
		WithTemplateGetter(&promptTemplateAdapter{repo: promptTemplateRepo}).
		WithPortableBindings(agentBindingRepo)
	agentVersionHandler := agent.NewVersionHandler(agent.NewVersionServiceWithAudit(agentRepo, agent.NewVersionRepository(pool), auditSvc))
	agentBindingHandler := agent.NewBindingHandler(agentRepo, agentBindingRepo)
	skillSvc := skill.NewService(skillRepo)
	skillHandler := skill.NewHandler(skillSvc).WithRepository(skillRepo)
	agentBundleHandler := agent.NewBundleHandler(
		agent.NewExporter(agentSvc, skillRepo, agentBindingRepo),
		agent.NewImporter(agentSvc, skillSvc, agentBindingRepo),
	)
	datasourceSvc := datasource.NewService(datasource.NewRepository(pool))
	toolRepo := tool.NewRepository(pool)
	// BUG-F1: wire toolRepo as ToolBinder so skill.Service.Create can auto-bind
	// a tool when {"toolId": "..."} is provided in POST /api/skills.
	skillSvc = skillSvc.WithToolBinder(toolRepo)
	toolSvc := tool.NewService(toolRepo).
		WithSettings(&toolSettingsAdapter{repo: settingsRepo}).
		WithDatasource(&toolDatasourceAdapter{svc: datasourceSvc}, tenantctx.FromContext)
	toolHandler := tool.NewHandler(toolSvc)
	memoryHandler := memory.NewHandler(memory.NewService(memory.NewRepository(pool)))
	promptTemplateHandler := prompttemplate.NewHandler(prompttemplate.NewService(prompttemplate.NewRepository(pool)))
	agentTemplateSvc := agenttemplate.NewService(agenttemplate.NewRepository(pool)).WithAgentCreator(agentSvc)
	agentTemplateHandler := agenttemplate.NewHandler(agentTemplateSvc)
	analyticsHandler := analytics.NewHandler(analytics.NewService(analytics.NewPostgresStore(pool)))
	executionHandler := execution.NewHandler(execution.NewService(execution.NewRepository(pool)))
	webhookHandler := webhook.NewHandler(webhook.NewService(webhook.NewRepository(pool)))
	// BUG-TRIGGER-NOT-MOUNTED: trigger domain was fully implemented but never wired.
	triggerHandler := trigger.NewHandler(trigger.NewService(trigger.NewRepository(pool), trigger.NewSimpleCronParser()))
	oauthSvc := oauth.NewServiceWithEncryption(oauth.NewRepository(pool), cfg.OAuthEncryptionKey)
	oauthHandler := oauth.NewHandler(oauthSvc)
	auditHandler := audit.NewHandler(auditSvc)
	metricsHandler := metrics.NewHandler(metrics.NewService(metrics.NewRepository(pool)))
	vpnSvc := vpnresource.NewService(vpnresource.NewRepository(pool))
	vpnHandler := vpnresource.NewHandler(vpnSvc)
	datasourceHandler := datasource.NewHandler(datasourceSvc)
	searchHandler := search.NewHandler(search.NewServiceWithPool(pool))
	mcpSvc := mcp.NewService(mcp.NewRepository(pool))
	mcpSvc.WithOAuthService(oauthSvc)
	if cfg.MCPRuntimeURL != "" {
		mcpSvc.WithRuntimeURL(cfg.MCPRuntimeURL)
	}
	integrationHandler := integration.NewHandler(integration.NewService(
		toolSvc,
		datasourceSvc,
		mcpSvc,
		vpnSvc,
	).WithHTTPManagement(skill.NewService(skillRepo), toolSvc, integration.NewRepository(pool)))
	kbRepo := knowledgebase.NewRepository(pool)
	pipelineHandler := pipeline.NewHandler(pipeline.NewRepository(pool))
	coreToolLoader := core.NewCoreToolLoader(pool)

	// Build agentic runner and wire it into the chat service.
	chatRepo := chat.NewRepository(pool)
	sessionRunner := buildAgenticRunner(cfg, pool, chatRepo, agentRepo, skillRepo, kbRepo, toolRepo, settingsRepo, mcpSvc.Repository(), integration.NewService(toolSvc, datasourceSvc, mcpSvc, vpnSvc), coreToolLoader, agentBindingRepo)

	var chatExecutor *chat.AsyncExecutor
	if cfg.RabbitMQURL != "" {
		chatExecutor = chat.NewAsyncExecutor(chatRepo, sessionRunner, cfg.RabbitMQURL)
		if agentRepo != nil {
			// P-C253-1: wire agent loader so background runs can enforce per-agent MCP bindings.
			chatExecutor = chatExecutor.WithAgentLoader(&agentConfigAdapter{
				repo:        agentRepo,
				bindingRepo: agent.NewBindingRepository(pool),
			})
		}
		go func() {
			if err := chatExecutor.StartWorker(context.Background()); err != nil {
				slog.Error("rabbitmq: chat worker failed", "err", err)
			}
		}()
	}

	chatSvc := chat.NewService(chatRepo, sessionRunner)
	if agentRepo != nil {
		chatSvc = chatSvc.WithAgentLoader(&agentConfigAdapter{
			repo:        agentRepo,
			bindingRepo: agent.NewBindingRepository(pool),
		})
	}
	permAuditRepo := agentic.NewPermissionAuditRepository(pool)
	chatHandler := chat.NewHandler(chatSvc, chatExecutor).
		WithTaskRepository(chatTask.NewRepository(pool)).
		WithPermissionAuditReader(&permissionAuditReaderAdapter{repo: permAuditRepo})

	// Channel adapter registry — adapters registered here handle inbound platform events.
	channelRegistry := channel.NewRegistry()
	channelRegistry.Register(channel.ChannelTypeSlack, &channel.SlackAdapter{})
	channelRegistry.Register(channel.ChannelTypeTelegram, &channel.TelegramAdapter{})
	channelRegistry.Register(channel.ChannelTypeDiscord, &channel.DiscordAdapter{})
	channelRegistry.Register(channel.ChannelTypeCustom, &channel.CustomAdapter{})
	channelSvc := channel.NewService(channel.NewRepository(pool), channelRegistry).
		WithDispatcher(&chatAgentDispatcher{chatSvc: chatSvc})
	channelHandler := channel.NewHandler(channelSvc)

	// A/B Testing — route sessions to challenger agent versions.
	abtestHandler := abtest.NewHandler(abtest.NewService(abtest.NewRepository(pool)))

	// Device Node Network — MCP-discoverable devices and agent bindings.
	deviceHandler := device.NewHandler(device.NewService(device.NewRepository(pool)))

	// Skill Evaluation Framework — runner wired with a no-op evaluator by default.
	// Production callers can inject a concrete Evaluator via the skilleval.Runner.
	skillevalRepo := skilleval.NewRepository(pool)
	skillevalRunner := skilleval.NewRunner(skillevalRepo, &noopSkillEvaluator{}, nil)
	skillevalSvc := skilleval.NewService(skillevalRepo).WithRunner(skillevalRunner)
	skillevalHandler := skilleval.NewHandler(skillevalSvc)
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
	kbHandler := knowledgebase.NewHandler(knowledgebase.NewService(kbRepo))
	if cfg.EmbeddingURL != "" {
		kbHandler.WithSearchClient(knowledge.NewPgDocumentSearchClient(pool, cfg.EmbeddingURL))
	}
	knowledgebaseHandler := kbHandler
	mcpHandler := mcp.NewHandler(mcpSvc)
	approvalHandler := approval.NewHandler(approval.NewService(approval.NewRepository(pool)))
	coreAgentLoader := core.NewCoreAgentLoader(pool)
	coreHandler := core.NewHandler(coreAgentLoader)

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
	// BUG-DEPR2 fix: removed r.Options("/*") wildcard — the CORS middleware now
	// intercepts ALL OPTIONS requests (with or without Origin header), so the
	// wildcard is redundant. Without it, chi returns 404 (not 405) for unregistered
	// routes, which is the correct HTTP semantics.
	r.Use(chain.CORSHandler())

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
		channelHandler.RegisterPublicRoutes(r)
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
		// MA-11: Capabilities endpoint — frontend queries this to adapt UI per user.
		// Extractor returns zeros in placeholder auth mode; wired to JWT claims once
		// agenthub-go-commons/auth lands.
		auth.NewHandler(
			func(*http.Request) (string, string, []string) { return "", "", nil },
			auth.FeatureFlags{RBAC: true},
		).RegisterRoutes(r)
		agent.NewHookHandler(pool).RegisterRoutes(r)
		agentVersionHandler.RegisterVersionRoutes(r)
		agentBindingHandler.RegisterBindingRoutes(r)
		agentBundleHandler.RegisterBundleRoutes(r)
		skillHandler.RegisterRoutes(r)
		toolHandler.RegisterRoutes(r)
		memoryHandler.RegisterRoutes(r)
		promptTemplateHandler.RegisterRoutes(r)
		agentTemplateHandler.RegisterRoutes(r)
		channelHandler.RegisterRoutes(r)
		abtestHandler.RegisterRoutes(r)
		deviceHandler.RegisterRoutes(r)
		skillevalHandler.RegisterRoutes(r)
		analyticsHandler.RegisterRoutes(r)
		executionHandler.RegisterRoutes(r)
		webhookHandler.RegisterRoutes(r)
		triggerHandler.RegisterRoutes(r)
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
		coreHandler.RegisterRoutes(r)
			// BUG-DEPR1: read-only deprecated pipeline endpoints (Sunset: 2026-07-01).
			pipelineHandler.RegisterRoutes(r)
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

// Get satisfies agent.TemplateGetter, allowing the agent handler to
// resolve a prompt template by ID for POST /api/agents/{id}/apply-template.
func (a *promptTemplateAdapter) Get(ctx context.Context, id uuid.UUID) (agent.TemplateContent, error) {
	tpl, err := a.repo.FindByID(ctx, id)
	if err != nil {
		return agent.TemplateContent{}, err
	}
	return agent.TemplateContent{
		Content:       tpl.Content,
		ModelOverride: tpl.ModelOverride,
		IsBuiltin:     tpl.AgentID == nil,
	}, nil
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
	coreToolLoader *core.CoreToolLoader,
	bindingRepo agent.BindingRepository,
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
	ptRepo := prompttemplate.NewRepository(pool)
	promptBuilder := agentic.NewPromptBuilder(skillRepo, kbRepo, chatRepo, agentic.DefaultPromptConfig()).
		WithPromptTemplateResolver(&promptTemplateAdapter{repo: ptRepo})
	toolSchemaBuilder := agentic.NewToolSchemaBuilder(skillRepo, toolRepo, kbRepo).
		WithCoreToolProvider(&coreToolAdapter{loader: coreToolLoader}).
		WithTokenBudgetProvider(&skillTokenBudgetAdapter{repo: bindingRepo})
	ctxManager := agentic.NewContextManager()
	hookRepo := agentic.NewHookRepository(pool)
	hookExecutor := agentic.NewHookExecutor(hookRepo)

	var concreteSkillRepo *skill.Repository
	if skillRepo != nil {
		concreteSkillRepo = skillRepo.(*skill.Repository)
	}
	var concreteToolRepo *tool.Repository
	if toolRepo != nil {
		concreteToolRepo = toolRepo.(*tool.Repository)
	}
	adapter := agentic.NewSessionRunnerAdapterWithFactory(
		factory,
		skillClient,
		promptBuilder,
		toolSchemaBuilder,
		ctxManager,
		nil, // MemoryBridge — requires Embedder, wired later
		hookExecutor,
		chatRepo,
		&agentConfigAdapter{repo: agentRepo, bindingRepo: agent.NewBindingRepository(pool)},
		agentRepo,
		concreteSkillRepo,
		concreteToolRepo,
		integSvc,
		mcpRepo,
	)

	// P-C102-1: apply server-level LLM call timeout from LLM_CALL_TIMEOUT_SECS.
	if cfg.LLMCallTimeoutSecs > 0 {
		adapter.WithLLMCallTimeout(time.Duration(cfg.LLMCallTimeoutSecs) * time.Second)
	}

	// Wire permission audit logger so every permission decision is persisted.
	adapter.WithPermissionAuditLogger(agentic.NewPermissionAuditRepository(pool))

	// P-C253-1: wire the MCP client so agents with bound MCP servers get their tools.
	// HTTPMCPClient calls the agenthub-mcp-client-runtime service which proxies
	// external MCP servers and exposes their tools over HTTP.
	if cfg.MCPRuntimeURL != "" {
		mcpHTTPClient := agentic.NewHTTPMCPClient(cfg.MCPRuntimeURL)
		cachedMCPClient := agentic.NewCachedMCPClient(mcpHTTPClient, 30*time.Second)
		adapter.WithMCPClient(cachedMCPClient)
		slog.Info("agentic: MCP client wired", "url", cfg.MCPRuntimeURL)
	}

	// P-E1-2: wire the document search client so the document_search builtin tool
	// executes locally via pgvector. Requires the embedding service to vectorize
	// the query before running cosine similarity search.
	if cfg.EmbeddingURL != "" {
		docSearchClient := knowledge.NewPgDocumentSearchClient(pool, cfg.EmbeddingURL)
		adapter.WithDocumentSearchClient(docSearchClient)
		slog.Info("agentic: document search client wired", "embeddingURL", cfg.EmbeddingURL)

		// Wire MemoryBridge so the memory_store builtin tool can persist and
		// recall memories. Shares the same embedding service as document search.
		embedder := agentic.NewHTTPEmbedder(cfg.EmbeddingURL)
		memSvc := memory.NewService(memory.NewRepository(pool))
		bridge := agentic.NewMemoryBridge(
			embedder,
			memSvc, // implements MemoryRecaller
			memSvc, // implements MemoryUpserter
			nil,    // MemoryEvaluator — background auto-store not wired yet
			agentic.DefaultMemoryBridgeConfig(),
		)
		// BUG-MEM7 fix: wire lister for recent-memory fallback when semantic recall
		// yields nothing (e.g. for exact codes or low-entropy values like "ALPHA-XYZ-42").
		bridge.WithLister(memSvc)
		adapter.WithMemoryBridge(bridge)
		slog.Info("agentic: memory bridge wired", "embeddingURL", cfg.EmbeddingURL)
	}

	return adapter
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

// agentConfigAdapter adapts agent.Repository to chat.AgentLoader.
type agentConfigAdapter struct {
	repo        agent.Repository
	bindingRepo agent.BindingRepository
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

	cfg := &chat.AgentRunConfig{
		ID:               ag.ID,
		SystemPrompt:     systemPrompt,
		ModelConfig:      ag.ModelConfig,
		PermissionRules:  ag.PermissionRules,
		EnableManagement: ag.EnableManagement,
		Status:           string(ag.Status),
	}

	// P-C115-1: include skill IDs so the chat service can snapshot them at session creation.
	// P-C253-1: include MCP server names so the runner can filter MCP tools.
	if a.bindingRepo != nil {
		if skillIDs, err := a.bindingRepo.ListSkillIDs(ctx, id); err == nil {
			cfg.SkillIDs = skillIDs
		}
		// P-C253-1: use ListMCPServerIDs (all, not filtered by enabled) to detect whether
		// the agent has ANY MCP bindings. If it does, pass only enabled names to the runner.
		// This distinguishes "no bindings → no filter" from "has bindings but all disabled → block all".
		if boundIDs, err := a.bindingRepo.ListMCPServerIDs(ctx, id); err == nil && len(boundIDs) > 0 {
			if enabledNames, err := a.bindingRepo.ListMCPServerNames(ctx, id); err == nil {
				// Ensure non-nil so runner knows to apply the filter even if all servers are disabled.
				if enabledNames == nil {
					enabledNames = []string{}
				}
				cfg.MCPServerNames = enabledNames
			}
		}
	}

	return cfg, nil
}

// coreToolAdapter adapts core.CoreToolLoader to agentic.CoreToolProvider.
// Converts CoreTool structs to LLMTool definitions with a minimal input schema.
type coreToolAdapter struct {
	loader *core.CoreToolLoader
}

// skillTokenBudgetAdapter adapts agent.BindingRepository to agentic.SkillTokenBudgetProvider.
type skillTokenBudgetAdapter struct {
	repo agent.BindingRepository
}

func (a *skillTokenBudgetAdapter) GetSkillTokenBudgets(ctx context.Context, agentID uuid.UUID) (map[uuid.UUID]*int, error) {
	return a.repo.GetSkillTokenBudgets(ctx, agentID)
}

// permissionAuditReaderAdapter bridges agentic.PermissionAuditRepository to chat.PermissionAuditReader.
type permissionAuditReaderAdapter struct {
	repo *agentic.PermissionAuditRepository
}

func (a *permissionAuditReaderAdapter) ListBySession(ctx context.Context, sessionID uuid.UUID, limit int) ([]chat.PermissionAuditEntryResponse, error) {
	entries, err := a.repo.ListBySession(ctx, sessionID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]chat.PermissionAuditEntryResponse, 0, len(entries))
	for _, e := range entries {
		resp := chat.PermissionAuditEntryResponse{
			SessionID:    e.SessionID.String(),
			ToolName:     e.ToolName,
			Decision:     string(e.Decision),
			MatchedRule:  e.MatchedRule,
			InputSnippet: e.InputSnippet,
			CreatedAt:    e.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if e.RunID != nil {
			s := e.RunID.String()
			resp.RunID = &s
		}
		out = append(out, resp)
	}
	return out, nil
}

func (a *coreToolAdapter) LoadCoreTools(ctx context.Context) ([]agentic.LLMTool, error) {
	tools, err := a.loader.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]agentic.LLMTool, 0, len(tools))
	for _, t := range tools {
		inputSchema := json.RawMessage(`{"type":"object","properties":{}}`)
		// Attempt to extract a richer schema from the tool config when available.
		if len(t.Config) > 2 {
			var cfg struct {
				InputSchema json.RawMessage `json:"inputSchema"`
			}
			if err := json.Unmarshal(t.Config, &cfg); err == nil && len(cfg.InputSchema) > 2 {
				inputSchema = cfg.InputSchema
			}
		}
		result = append(result, agentic.LLMTool{
			Name:        t.Slug,
			Description: t.Description,
			InputSchema: inputSchema,
		})
	}
	return result, nil
}

// chatAgentDispatcher implements channel.AgentDispatcher by delegating to chat.Service.
// It creates a transient session bound to the given agent, runs the agentic loop,
// drains all SSE events, and returns the final assistant text.
type chatAgentDispatcher struct {
	chatSvc *chat.Service
}

func (d *chatAgentDispatcher) Dispatch(ctx context.Context, agentID uuid.UUID, userText, tenantID string) (string, error) {
	// Create a transient session for this inbound message.
	session, err := d.chatSvc.CreateSession(ctx, chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   "inbound-" + agentID.String()[:8],
	})
	if err != nil {
		return "", fmt.Errorf("channel dispatch: create session: %w", err)
	}

	events, err := d.chatSvc.RunSession(ctx, session.ID, userText, tenantID)
	if err != nil {
		return "", fmt.Errorf("channel dispatch: run session: %w", err)
	}

	// Drain events and collect assistant text deltas.
	var textBuf []byte
	for ev := range events {
		if ev.Type == "text_delta" {
			var delta struct {
				Content string `json:"content"`
			}
			if err := json.Unmarshal(ev.Data, &delta); err == nil {
				textBuf = append(textBuf, delta.Content...)
			}
		}
	}
	return string(textBuf), nil
}

// noopSkillEvaluator is the default skilleval.Evaluator used at startup.
// It returns empty output for every input, causing all cases to fail gracefully.
// Replace with a real evaluator that calls the agentic runner when ready.
type noopSkillEvaluator struct{}

func (e *noopSkillEvaluator) Evaluate(_ context.Context, _ uuid.UUID, _ string) (skilleval.EvalOutput, error) {
	return skilleval.EvalOutput{}, nil
}

// tenantSchemaMigratorFunc is a function adapter for the tenant.SchemaMigrator interface.
type tenantSchemaMigratorFunc func(ctx context.Context, tenantID string) error

func (f tenantSchemaMigratorFunc) MigrateTenant(ctx context.Context, tenantID string) error {
	return f(ctx, tenantID)
}
