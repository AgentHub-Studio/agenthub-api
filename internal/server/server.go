package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/abtest"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/admintenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agenttemplate"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/analytics"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/approval"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/auth"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/channel"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/suggest"
	chatTask "github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/task"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chatsession"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/copilot"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/core"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/device"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/document"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/execution"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/experiment"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration/probe"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledge"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/llmpreset"
	mkplInstallation "github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/installation"
	mkplListing "github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/listing"
	mkplReview "github.com/AgentHub-Studio/agenthub-api/internal/domain/marketplace/review"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/memory"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/metrics"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/pipeline"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/prompttemplate"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/provider"
	regDependency "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/dependency"
	regInstallation "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/installation"
	regPackage "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/package"
	regVersion "github.com/AgentHub-Studio/agenthub-api/internal/domain/registry/version"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/search"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skilleval"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenantsignup"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/trigger"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/user"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/vpnresource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/webhook"
	apikc "github.com/AgentHub-Studio/agenthub-api/internal/keycloak"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
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
	chain := middleware.New(cfg.KeycloakBaseURL, cfg.CORSOrigins).WithIssuerURL(cfg.KeycloakIssuerURL)

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
	tenantSignupHandler := tenantsignup.NewHandler(tenantsignup.NewService(
		tenantSvc, keycloakClient, cfg.KeycloakBaseURL, os.Getenv("FRONTEND_BASE_URL"),
	))
	adminTenantHandler := admintenant.NewHandler(admintenant.NewService(tenantSvc, tenant.NewRepository(pool), keycloakClient, provisioner))
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
	agentVersionHandler := agent.NewVersionHandler(agent.NewVersionServiceWithAudit(agentRepo, agent.NewVersionRepository(pool), auditSvc)).
		WithAgentService(agentSvc)
	agentBindingHandler := agent.NewBindingHandler(agentRepo, agentBindingRepo)
	skillSvc := skill.NewService(skillRepo)
	skillHandler := skill.NewHandler(skillSvc).WithRepository(skillRepo)
	// agentBundleHandler is wired after toolSvc — bug 294 needs both the tool
	// repo (export) and the tool service (import) to ship/restore tool defs.
	datasourceSvc := datasource.NewService(datasource.NewRepository(pool))
	toolRepo := tool.NewRepository(pool)
	// kbRepo é usado pelo toolSvc (KBExister, bug 231) e pelo kbHandler;
	// declarado aqui pra ficar disponível antes do toolSvc.
	kbRepo := knowledgebase.NewRepository(pool)
	// BUG-F1: wire toolRepo as ToolBinder so skill.Service.Create can auto-bind
	// a tool when {"toolId": "..."} is provided in POST /api/skills.
	skillSvc = skillSvc.WithToolBinder(toolRepo)
	toolSvc := tool.NewService(toolRepo).
		WithSettings(&toolSettingsAdapter{repo: settingsRepo}).
		WithDatasource(&toolDatasourceAdapter{svc: datasourceSvc}, tenantctx.FromContext).
		WithKBExister(&kbExisterAdapter{repo: kbRepo})
	toolHandler := tool.NewHandler(toolSvc).
		WithSkillExister(&skillExisterAdapter{repo: skillRepo})
	agentBundleHandler := agent.NewBundleHandler(
		agent.NewExporter(agentSvc, skillRepo, agentBindingRepo).WithToolRepo(toolRepo),
		agent.NewImporter(agentSvc, skillSvc, agentBindingRepo).WithToolSvc(toolSvc),
	)
	memoryHandler := memory.NewHandler(memory.NewService(memory.NewRepository(pool))).
		WithAgentExister(&agentExisterAdapter{svc: agentSvc})
	promptTemplateHandler := prompttemplate.NewHandler(prompttemplate.NewService(prompttemplate.NewRepository(pool)))
	agentTemplateSvc := agenttemplate.NewService(agenttemplate.NewRepository(pool)).WithAgentCreator(agentSvc)
	agentTemplateHandler := agenttemplate.NewHandler(agentTemplateSvc)
	providerHandler := provider.NewHandler(provider.NewService(provider.NewRepository(pool)))
	probeHandler := probe.NewHandler(probe.NewService())
	analyticsStore := analytics.AnalyticsStore(analytics.NewPostgresStore(pool))
	var runMetricsFactory chat.RunMetricsCollectorFactory
	if cfg.ClickHouse.IsConfigured() {
		clickHouseClient := analytics.NewClickHouseClient(cfg.ClickHouse)
		if err := clickHouseClient.EnsureSchema(context.Background()); err != nil {
			slog.Warn("analytics: clickhouse disabled after schema bootstrap failure", "err", err)
		} else {
			analyticsStore = analytics.NewClickHouseStore(clickHouseClient)
			runMetricsFactory = &runMetricsCollectorFactory{
				sink: analytics.NewAsyncSink(analytics.NewClickHouseSink(clickHouseClient), 100),
			}
			slog.Info("analytics: clickhouse pipeline enabled")
		}
	}
	analyticsHandler := analytics.NewHandler(analytics.NewService(analyticsStore))
	executionHandler := execution.NewHandler(execution.NewService(execution.NewRepository(pool)))
	webhookSvc := webhook.NewService(webhook.NewRepository(pool)).
		WithTenantLister(&webhookTenantListerAdapter{repo: tenant.NewRepository(pool)})
	webhookHandler := webhook.NewHandler(webhookSvc)
	// BUG-TRIGGER-NOT-MOUNTED: trigger domain was fully implemented but never wired.
	triggerRepo := trigger.NewRepository(pool)
	triggerCron := trigger.NewSimpleCronParser()
	triggerHandler := trigger.NewHandler(trigger.NewService(triggerRepo, triggerCron)).
		WithAgentExister(&agentExisterAdapter{svc: agentSvc})
	// Bug 237 (fase 2): scheduler firará chat session + run para cada
	// trigger due. Wireado abaixo após chatSvc + chatExecutor.
	triggerScheduler := trigger.NewScheduler(pool, &triggerTenantListerAdapter{repo: tenant.NewRepository(pool)}, triggerRepo, triggerCron)
	oauthSvc := oauth.NewServiceWithEncryption(oauth.NewRepository(pool), cfg.OAuthEncryptionKey)
	oauthHandler := oauth.NewHandler(oauthSvc)
	auditHandler := audit.NewHandler(auditSvc)
	metricsSvc := metrics.NewService(metrics.NewRepository(pool))
	metricsHandler := metrics.NewHandler(metricsSvc).
		WithAgentExister(&agentExisterAdapter{svc: agentSvc})
	vpnSvc := vpnresource.NewService(vpnresource.NewRepository(pool))
	vpnHandler := vpnresource.NewHandler(vpnSvc)
	datasourceHandler := datasource.NewHandler(datasourceSvc)
	searchHandler := search.NewHandler(search.NewServiceWithPool(pool))
	mcpSvc := mcp.NewService(mcp.NewRepository(pool))
	mcpSvc.WithOAuthService(oauthSvc)
	if cfg.MCPRuntimeURL != "" {
		mcpSvc.WithRuntimeURL(cfg.MCPRuntimeURL)
	}
	integrationSvc := integration.NewService(
		toolSvc,
		datasourceSvc,
		mcpSvc,
		vpnSvc,
	).WithHTTPManagement(skill.NewService(skillRepo), toolSvc, integration.NewRepository(pool))
	integrationHandler := integration.NewHandler(integrationSvc)
	suggestHandler := suggest.NewHandler(suggest.NewService(integrationSvc))
	pipelineHandler := pipeline.NewHandler(pipeline.NewRepository(pool))

	// CopilotKit Phase 5 — completions endpoint (autocompletion ghost-text).
	// Reuses the same per-tenant settings-driven model factory as the agentic
	// runner, but without any session/SSE state.
	copilotHandler := copilot.NewHandler(copilot.NewService(&settingsChatModelFactory{
		settingsRepo: settingsRepo,
		fallback:     buildDefaultChatModel(),
	}))
	coreToolLoader := core.NewCoreToolLoader(pool)

	// Build agentic runner and wire it into the chat service.
	chatRepo := chat.NewRepository(pool)
	sessionRunner := buildAgenticRunner(cfg, pool, chatRepo, agentRepo, skillRepo, kbRepo, toolRepo, settingsRepo, mcpSvc.Repository(), integration.NewService(toolSvc, datasourceSvc, mcpSvc, vpnSvc), coreToolLoader, agentBindingRepo, agentSvc)
	voiceSvc := chat.NewOpenAIVoiceServiceFromEnv()

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
		// Persist agent_metrics rows after every completed async run.
		chatExecutor = chatExecutor.WithMetricsRecorder(&metricsRecorderAdapter{svc: metricsSvc})
		chatExecutor = chatExecutor.WithVoiceService(voiceSvc)
		if runMetricsFactory != nil {
			chatExecutor = chatExecutor.WithRunMetricsCollectorFactory(runMetricsFactory)
		}
		// Bug 244: validate agent existence before accepting runs (sessions
		// outlive their agents when DELETE /api/agents/{id} runs).
		if agentRepo != nil {
			chatExecutor = chatExecutor.WithAgentExister(&chatAgentExisterAdapter{repo: agentRepo})
		}
		// Bug 291: close trigger_run rows when their chat run terminates so
		// status reflects the actual outcome (completed/failed/cancelled)
		// instead of forever stuck at "running".
		chatExecutor = chatExecutor.WithCompletionHook(func(ctx context.Context, sessionID, runID uuid.UUID, status chat.ChatRunStatus, turns, tokens int, errMsg string) {
			triggerStatus := trigger.RunStatusFailed
			switch status {
			case chat.ChatRunStatusCompleted:
				triggerStatus = trigger.RunStatusCompleted
			case chat.ChatRunStatusCancelled:
				triggerStatus = trigger.RunStatusFailed // no Cancelled enum on trigger run; surface as failed
			}
			var errPtr *string
			if errMsg != "" {
				errPtr = &errMsg
			}
			var turnsPtr, tokensPtr *int
			if turns > 0 {
				turnsPtr = &turns
			}
			if tokens > 0 {
				tokensPtr = &tokens
			}
			triggerRun, parentTrigger, updated, err := triggerRepo.CompleteRunBySession(ctx, sessionID, triggerStatus, turnsPtr, tokensPtr, errPtr)
			if err != nil {
				slog.Debug("trigger.completion: failed to complete trigger run", "sessionId", sessionID, "err", err)
				return
			}
			if !updated || parentTrigger.NotificationWebhookID == nil {
				return
			}
			payload := map[string]any{
				"event":       "trigger.run.completed",
				"triggerId":   parentTrigger.ID,
				"triggerName": parentTrigger.Name,
				"agentId":     parentTrigger.AgentID,
				"runId":       triggerRun.ID,
				"sessionId":   triggerRun.SessionID,
				"chatRunId":   runID,
				"status":      triggerRun.Status,
				"startedAt":   triggerRun.StartedAt,
				"completedAt": triggerRun.CompletedAt,
				"totalTurns":  triggerRun.TotalTurns,
				"totalTokens": triggerRun.TotalTokens,
				"error":       triggerRun.Error,
			}
			if _, err := webhookSvc.DispatchEvent(ctx, *parentTrigger.NotificationWebhookID, "trigger.run.completed", payload); err != nil {
				slog.Warn("trigger.notification: dispatch failed",
					"triggerID", parentTrigger.ID,
					"webhookID", *parentTrigger.NotificationWebhookID,
					"err", err)
			}
		})
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
		WithPermissionAuditReader(&permissionAuditReaderAdapter{repo: permAuditRepo}).
		WithVoiceService(voiceSvc)

	// Bug 237 fase 2: wire trigger Firer agora que chatSvc + chatExecutor
	// existem, e starta o scheduler.
	if chatExecutor != nil {
		triggerScheduler.WithFirer(&triggerFirerAdapter{chatSvc: chatSvc, chatExecutor: chatExecutor})
	}
	triggerScheduler.Start(context.Background())

	// Memory pruner — clears expired (ExpiresAt < NOW) and stale "general" memories
	// every 6h across all tenants. Issue #138.
	memoryPruner := memory.NewPruner(
		&triggerTenantListerAdapter{repo: tenant.NewRepository(pool)},
		memory.NewRepository(pool),
		memory.DefaultPrunerConfig(),
	)
	memoryPruner.Start(context.Background())

	// Channel adapter registry — adapters registered here handle inbound platform events.
	channelRegistry := channel.NewRegistry()
	channelRegistry.Register(channel.ChannelTypeSlack, &channel.SlackAdapter{})
	channelRegistry.Register(channel.ChannelTypeTelegram, &channel.TelegramAdapter{})
	channelRegistry.Register(channel.ChannelTypeDiscord, &channel.DiscordAdapter{})
	channelRegistry.Register(channel.ChannelTypeCustom, &channel.CustomAdapter{})
	// Bug 240: WEBHOOK type was registered no adapter, fazendo todos
	// os 20+ channels WEBHOOK criados retornarem 401 inbound. Reusa o
	// CustomAdapter que valida apenas via token na URL.
	channelRegistry.Register(channel.ChannelTypeWebhook, &channel.CustomAdapter{})
	channelSvc := channel.NewService(channel.NewRepository(pool), channelRegistry).
		WithDispatcher(&chatAgentDispatcher{chatSvc: chatSvc}).
		WithTenantLister(&triggerTenantListerAdapter{repo: tenant.NewRepository(pool)}) // bug 240b
	channelHandler := channel.NewHandler(channelSvc)

	// A/B Testing — route sessions to challenger agent versions.
	abtestHandler := abtest.NewHandler(abtest.NewService(abtest.NewRepository(pool)))
	// Prompt experiments remain available for the existing experiment result API.
	experimentHandler := experiment.NewHandler(experiment.NewService(experiment.NewRepository(pool)))

	// Device Node Network — MCP-discoverable devices and agent bindings.
	deviceHandler := device.NewHandler(device.NewService(device.NewRepository(pool)))

	// Skill Evaluation Framework — runner wired with a no-op evaluator by default.
	// Production callers can inject a concrete Evaluator via the skilleval.Runner.
	skillevalRepo := skilleval.NewRepository(pool)
	skillevalRunner := skilleval.NewRunner(skillevalRepo, &noopSkillEvaluator{}, nil)
	skillevalSvc := skilleval.NewService(skillevalRepo).
		WithRunner(skillevalRunner).
		WithSkillExister(&skillExisterAdapter{repo: skillRepo})
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
	chatHandler.WithAttachmentStorage(docStorage)
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
	documentHandler := document.NewHandler(document.NewService(document.NewRepository(pool), docStorage, docPublisher, cfg.MinIO.DocumentsBucket)).
		WithKBExister(&kbExisterAdapter{repo: kbRepo})
	kbHandler := knowledgebase.NewHandler(knowledgebase.NewService(kbRepo))
	if cfg.EmbeddingURL != "" {
		kbHandler.WithSearchClient(knowledge.NewPgDocumentSearchClient(pool, cfg.EmbeddingURL))
	}
	knowledgebaseHandler := kbHandler
	mcpHandler := mcp.NewHandler(mcpSvc)
	approvalHandler := approval.NewHandler(approval.NewService(approval.NewRepository(pool)))
	coreAgentLoader := core.NewCoreAgentLoader(pool)
	coreOnboardingService := core.NewOnboardingService(
		core.NewCoreCapabilityOnboardingChecklistLoader(pool),
		core.NewOnboardingStatusRepository(pool),
	)
	coreHandler := core.NewHandler(coreAgentLoader, coreOnboardingService)

	// Marketplace handlers.
	mkplListingHandler := mkplListing.NewHandler(mkplListing.NewService(mkplListing.NewRepository(pool)))
	mkplListingRepo := mkplListing.NewRepository(pool)
	mkplReviewHandler := mkplReview.NewHandler(mkplReview.NewService(mkplReview.NewRepository(pool), mkplListingRepo)).
		WithListingExister(&listingExisterAdapter{repo: mkplListingRepo})

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
	pkgExister := &packageExisterAdapter{repo: pkgRepo}
	regPackageHandler := regPackage.NewHandler(regPackage.NewService(pkgRepo))
	regVersionHandler := regVersion.NewHandler(regVersion.NewService(regVersion.NewRepository(pool), pkgRepo)).
		WithPackageExister(pkgExister)
	regDependencyHandler := regDependency.NewHandler(regDependency.NewService(regDependency.NewRepository(pool), pkgRepo)).
		WithPackageExister(pkgExister)
	regInstallationHandler := regInstallation.NewHandler(regInstallation.NewService(regInstallation.NewRepository(pool), regStorage)).
		WithPackageExister(pkgExister)
	// Bug 238: marketplace installation valida package existence (após pkgExister wireado).
	mkplInstallationHandler := mkplInstallation.NewHandler(
		mkplInstallation.NewService(mkplInstallation.NewRepository(pool)).
			WithPackageExister(pkgExister))

	r := chi.NewRouter()
	r.Use(chiMiddleware.RealIP)
	// Bug 274: RequestID em root garante que 404/405 catch-all também
	// ecoa o X-Request-ID (antes só rotas em Public/Protected groups
	// passavam pelo middleware). Frontend de tracing precisa do header
	// em qualquer resposta para correlacionar logs server-side.
	r.Use(middleware.RequestID)
	// Bug 266: limita body size global pra evitar DoS via JSON gigante.
	// Endpoints que precisam mais (uploads VPN .ovpn, portable YAML)
	// usam seu próprio MaxBytesReader específico.
	r.Use(middleware.MaxBodyBytes(middleware.DefaultMaxBodyBytes))
	// Bug 276: security headers globais (nosniff/X-Frame-Options/Referrer-Policy).
	// Defesa em profundidade contra MIME sniffing e clickjacking.
	r.Use(middleware.SecurityHeaders)
	// Bug 279: cap em URL (path+query) previne DoS via URL gigante.
	r.Use(middleware.MaxURLBytes(middleware.DefaultMaxURLBytes))

	// JSON 404/405 handlers para consistência. Sem isso, chi default
	// retorna text/plain "404 page not found" que quebra clientes que
	// só lidam com JSON.
	// Bug 265: padroniza chi 404/405 com o shape `{"error": "..."}` usado
	// por todos os outros handlers (httputil/respond), em vez do shape
	// `{"status": N, "message": "..."}` que destoava. Frontend e clientes
	// que dependem de `body.error` agora têm comportamento consistente.
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"error":"method not allowed"}`))
	})

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
		tenantSignupHandler.RegisterPublicRoutes(r)
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
		// Bug 296: extractor pulls subject + tenant + roles from the JWT context
		// populated by the auth middleware so the frontend's capabilities endpoint
		// reflects the real session (was hard-coded to zeros).
		auth.NewHandler(
			func(req *http.Request) (string, string, []string) {
				ctx := req.Context()
				return middleware.SubjectFromContext(ctx),
					tenantctx.FromContext(ctx),
					middleware.RolesFromContext(ctx)
			},
			auth.FeatureFlags{RBAC: true},
		).RegisterRoutes(r)
		agent.NewHookHandler(pool).WithAgentService(agentSvc).RegisterRoutes(r)
		agentVersionHandler.RegisterVersionRoutes(r)
		agentBindingHandler.RegisterBindingRoutes(r)
		agentBundleHandler.RegisterBundleRoutes(r)
		skillHandler.RegisterRoutes(r)
		toolHandler.RegisterRoutes(r)
		memoryHandler.RegisterRoutes(r)
		promptTemplateHandler.RegisterRoutes(r)
		agentTemplateHandler.RegisterRoutes(r)
		providerHandler.RegisterRoutes(r)
		probeHandler.RegisterRoutes(r)
		channelHandler.RegisterRoutes(r)
		abtestHandler.RegisterRoutes(r)
		r.Mount("/api/experiments", experimentHandler.Routes())
		deviceHandler.RegisterRoutes(r)
		skillevalHandler.RegisterRoutes(r)
		analyticsHandler.RegisterRoutes(r)
		executionHandler.RegisterRoutes(r)
		webhookHandler.RegisterRoutes(r)
		triggerHandler.RegisterRoutes(r)
		r.Mount("/api/oauth-credentials", oauthHandler.Routes())
		r.Mount("/api/audit-logs", auditHandler.Routes())
		r.Mount("/api/metrics", metricsHandler.Routes())
		r.Mount("/api/agents/{id}/metrics", metricsHandler.AgentRoutes())
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
		suggestHandler.RegisterRoutes(r)
		copilotHandler.RegisterRoutes(r)
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

	// Core-tenant admin routes — only callable from the `core` tenant by an
	// authenticated user holding the `admin` realm role.
	r.Group(func(r chi.Router) {
		for _, m := range chain.Protected() {
			r.Use(m)
		}
		r.Use(middleware.RequireCoreTenant)
		r.Use(middleware.RequireRole("admin"))
		adminTenantHandler.RegisterRoutes(r)
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

// skillExisterAdapter wraps skill.Repository so tool.Handler can validate
// parent-skill existence before listing skill→tool bindings (bug 209).
type skillExisterAdapter struct {
	repo *skill.Repository
}

func (a *skillExisterAdapter) GetByID(ctx context.Context, id uuid.UUID) error {
	_, err := a.repo.GetByID(ctx, id)
	return err
}

// kbExisterAdapter wraps knowledgebase.Repository so document.Handler can
// validate parent-KB existence before listing documents (bug 209 batch).
type kbExisterAdapter struct {
	repo knowledgebase.Repository
}

func (a *kbExisterAdapter) GetByID(ctx context.Context, id uuid.UUID) error {
	_, err := a.repo.GetByID(ctx, id)
	return err
}

// agentExisterAdapter wraps agent.Service so handlers in other domains
// (trigger, hooks) can validate parent-agent existence (bug 209 batch).
type agentExisterAdapter struct {
	svc agent.Service
}

func (a *agentExisterAdapter) GetByID(ctx context.Context, id uuid.UUID) error {
	_, err := a.svc.Get(ctx, id)
	return err
}

// triggerTenantListerAdapter expõe um subset minimal de tenant.Repository
// para o trigger.Scheduler: apenas listAll IDs (bug 237 fase 1).
type triggerTenantListerAdapter struct {
	repo tenant.Repository
}

func (a *triggerTenantListerAdapter) ListAllIDs(ctx context.Context) ([]string, error) {
	// Page grande o suficiente p/ ambientes desenvolvimento; tenant count
	// real em produção provavelmente exigirá paginação.
	tenants, _, err := a.repo.FindAll(ctx, pagination.PageRequest{Page: 0, Size: 500})
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(tenants))
	for i, t := range tenants {
		ids[i] = t.ID
	}
	return ids, nil
}

// webhookTenantListerAdapter expõe ListAllIDs para o webhook.Service
// permitir cross-tenant lookup do token no endpoint público de ingest
// (bug 241).
type webhookTenantListerAdapter struct {
	repo tenant.Repository
}

func (a *webhookTenantListerAdapter) ListAllIDs(ctx context.Context) ([]string, error) {
	tenants, _, err := a.repo.FindAll(ctx, pagination.PageRequest{Page: 0, Size: 500})
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(tenants))
	for i, t := range tenants {
		ids[i] = t.ID
	}
	return ids, nil
}

// triggerFirerAdapter implementa trigger.Firer agregando chat.Service e
// chat.AsyncExecutor para o scheduler poder criar sessions e enfileirar
// runs (bug 237 fase 2).
type triggerFirerAdapter struct {
	chatSvc      *chat.Service
	chatExecutor *chat.AsyncExecutor
}

func (a *triggerFirerAdapter) CreateSessionForTrigger(ctx context.Context, tenantID string, agentID uuid.UUID, title string) (uuid.UUID, error) {
	// Garantir tenant context para search_path correto.
	ctx = tenantctx.NewContext(ctx, tenantID)
	resp, err := a.chatSvc.CreateSession(ctx, chat.CreateSessionRequest{
		AgentID: &agentID,
		Title:   title,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return resp.ID, nil
}

func (a *triggerFirerAdapter) EnqueueRun(ctx context.Context, sessionID uuid.UUID, tenantID, message string) (uuid.UUID, error) {
	ctx = tenantctx.NewContext(ctx, tenantID)
	return a.chatExecutor.EnqueueRun(ctx, sessionID, tenantID, message)
}

// packageExisterAdapter wraps regPackage.Repository so version/dependency/
// installation handlers can validate parent package existence (bug 210).
type packageExisterAdapter struct {
	repo *regPackage.Repository
}

func (a *packageExisterAdapter) GetByID(ctx context.Context, id uuid.UUID) error {
	_, err := a.repo.GetByID(ctx, id)
	return err
}

// listingExisterAdapter wraps mkplListing.Repository so review.Handler can
// validate parent marketplace listing existence (bug 211).
type listingExisterAdapter struct {
	repo *mkplListing.Repository
}

func (a *listingExisterAdapter) GetByID(ctx context.Context, id uuid.UUID) error {
	_, err := a.repo.FindByID(ctx, id)
	return err
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
	agentDeleter agent.Deleter,
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
	adapter.WithAgentDeleter(agentDeleter)

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
		return ollama.New(envCfg.OllamaBaseURL, envCfg.OllamaAPIKey)
	}
	if envCfg.OpenRouterAPIKey != "" {
		return openrouter.New(envCfg.OpenRouterAPIKey, envCfg.OpenRouterBaseURL, "agenthub")
	}
	return nil
}

// metricsRecorderAdapter bridges chat.MetricsRecorder to metrics.Service
// without dragging the metrics package into chat (avoids import cycle).
type metricsRecorderAdapter struct {
	svc *metrics.Service
}

func (m *metricsRecorderAdapter) Record(ctx context.Context, tenantID string, req chat.MetricsRecord) error {
	_, err := m.svc.Record(ctx, tenantID, metrics.RecordRequest{
		AgentID:          req.AgentID,
		SessionID:        req.SessionID,
		ModelName:        req.ModelName,
		Provider:         req.Provider,
		PromptTokens:     req.PromptTokens,
		CompletionTokens: req.CompletionTokens,
		TotalTokens:      req.TotalTokens,
		EstimatedCostUSD: req.EstimatedCostUSD, // Bug 221
		LatencyMs:        req.LatencyMs,
	})
	return err
}

// runMetricsCollectorFactory bridges chat async events to the agentic analytics
// collector without importing ClickHouse details into the chat package.
type runMetricsCollectorFactory struct {
	sink agentic.AnalyticsSink
}

func (f *runMetricsCollectorFactory) NewCollector(tenantID string, agentID, sessionID uuid.UUID, runID, provider, model string) chat.RunMetricsCollector {
	return &runMetricsCollectorAdapter{
		collector: agentic.NewMetricsCollector(f.sink, tenantID, agentID, sessionID, runID, provider, model),
	}
}

type runMetricsCollectorAdapter struct {
	collector *agentic.MetricsCollector
}

func (a *runMetricsCollectorAdapter) Collect(event chat.RunEvent) {
	a.collector.Collect(agentic.RunEvent{
		Type: agentic.RunEventType(event.Type),
		Data: event.Data,
	})
}

func (a *runMetricsCollectorAdapter) Flush() error {
	return a.collector.Flush()
}

// agentConfigAdapter adapts agent.Repository to chat.AgentLoader.
type agentConfigAdapter struct {
	repo        agent.Repository
	bindingRepo agent.BindingRepository
}

// chatAgentExisterAdapter wraps agent.Repository so chat.AsyncExecutor
// can validate that an agent still exists before accepting a run on its
// session (bug 244).
type chatAgentExisterAdapter struct {
	repo agent.Repository
}

func (a *chatAgentExisterAdapter) GetByID(ctx context.Context, id uuid.UUID) error {
	_, err := a.repo.FindByID(ctx, id)
	return err
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

	disableAskUser, disableAgentDelegation := parseAgentToolFlags(ag.Config)
	cfg := &chat.AgentRunConfig{
		ID:                     ag.ID,
		SystemPrompt:           systemPrompt,
		ModelConfig:            ag.ModelConfig,
		PermissionRules:        ag.PermissionRules,
		EnableManagement:       ag.EnableManagement,
		DisableAskUser:         disableAskUser,
		DisableAgentDelegation: disableAgentDelegation,
		Status:                 string(ag.Status),
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

// parseAgentToolFlags extracts per-agent builtin tool opt-outs from agent.Config JSONB.
// Recognized keys (all optional, default false):
//   - disableAskUser        → removes `ask_user` from the agent's tool set
//   - disableAgentDelegation → removes `agent` (sub-agent spawner)
//
// Invalid/missing JSON is treated as "both flags false" so legacy agents retain
// their existing tool surface.
func parseAgentToolFlags(raw json.RawMessage) (disableAskUser, disableAgentDelegation bool) {
	if len(raw) == 0 {
		return false, false
	}
	var m struct {
		DisableAskUser         *bool `json:"disableAskUser"`
		DisableAgentDelegation *bool `json:"disableAgentDelegation"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		return false, false
	}
	if m.DisableAskUser != nil {
		disableAskUser = *m.DisableAskUser
	}
	if m.DisableAgentDelegation != nil {
		disableAgentDelegation = *m.DisableAgentDelegation
	}
	return disableAskUser, disableAgentDelegation
}
