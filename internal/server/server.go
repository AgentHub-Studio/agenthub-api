package server

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/document"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/execution"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/experiment"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/llmpreset"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/memory"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/metrics"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/pipeline"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/search"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/settings"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tenant"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/user"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/vpnresource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/webhook"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
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

	// Instantiate domain handlers.
	tenantHandler := tenant.NewHandler(tenant.NewService(tenant.NewRepository(pool), nil))
	userHandler := user.NewHandler(user.NewService(keycloakClient))
	settingsHandler := settings.NewHandler(settings.NewService(settings.NewRepository(pool)))
	llmpresetHandler := llmpreset.NewHandler(llmpreset.NewService(llmpreset.NewRepository(pool)))
	agentHandler := agent.NewHandler(agent.NewService(agent.NewRepository(pool)))
	pipelineHandler := pipeline.NewHandler(pipeline.NewService(pipeline.NewRepository(pool)))
	skillHandler := skill.NewHandler(skill.NewService(skill.NewRepository(pool)))
	toolHandler := tool.NewHandler(tool.NewService(tool.NewRepository(pool)))
	memoryHandler := memory.NewHandler(memory.NewService(memory.NewRepository(pool)))
	executionHandler := execution.NewHandler(execution.NewService(execution.NewRepository(pool)))
	webhookHandler := webhook.NewHandler(webhook.NewService(webhook.NewRepository(pool)))
	oauthHandler := oauth.NewHandler(oauth.NewService(oauth.NewRepository(pool)))
	auditHandler := audit.NewHandler(audit.NewService(audit.NewRepository(pool)))
	metricsHandler := metrics.NewHandler(metrics.NewService(metrics.NewRepository(pool)))
	experimentHandler := experiment.NewHandler(experiment.NewService(experiment.NewRepository(pool)))
	vpnHandler := vpnresource.NewHandler(vpnresource.NewService(vpnresource.NewRepository(pool)))
	datasourceHandler := datasource.NewHandler(datasource.NewService(datasource.NewRepository(pool)))
	searchHandler := search.NewHandler(search.NewService(pool))
	chatHandler := chat.NewHandler(chat.NewService(chat.NewRepository(pool)))
	documentHandler := document.NewHandler(document.NewService(document.NewRepository(pool)))
	knowledgebaseHandler := knowledgebase.NewHandler(knowledgebase.NewService(knowledgebase.NewRepository(pool)))
	mcpHandler := mcp.NewHandler(mcp.NewService(mcp.NewRepository(pool)))

	r := chi.NewRouter()
	r.Use(chiMiddleware.RealIP)

	// Health endpoints — no auth.
	r.Get("/health", s.handleHealth)
	r.Get("/ready", s.handleReady)

	// Public routes — no JWT required.
	r.Group(func(r chi.Router) {
		for _, m := range chain.Public() {
			r.Use(m)
		}
		tenantHandler.RegisterPublicRoutes(r)
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
		pipelineHandler.RegisterRoutes(r)
		skillHandler.RegisterRoutes(r)
		toolHandler.RegisterRoutes(r)
		memoryHandler.RegisterRoutes(r)
		executionHandler.RegisterRoutes(r)
		webhookHandler.RegisterRoutes(r)
		r.Mount("/api/oauth-credentials", oauthHandler.Routes())
		r.Mount("/api/audit-logs", auditHandler.Routes())
		r.Mount("/api/metrics", metricsHandler.Routes())
		r.Route("/api/agents/{agentId}/metrics", func(r chi.Router) {
			r.Mount("/", metricsHandler.AgentRoutes())
		})
		r.Mount("/api/experiments", experimentHandler.Routes())
		r.Mount("/api/vpn-resources", vpnHandler.Routes())
		r.Mount("/api/datasources", datasourceHandler.Routes())
		r.Get("/api/proxy/datasources/{id}", datasourceHandler.ProxyRoutes().ServeHTTP)
		r.Mount("/api/search", searchHandler.Routes())
		chatHandler.RegisterRoutes(r)
		documentHandler.RegisterRoutes(r)
		knowledgebaseHandler.RegisterRoutes(r)
		mcpHandler.RegisterRoutes(r)
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
