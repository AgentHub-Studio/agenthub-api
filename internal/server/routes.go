package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/datasource"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/experiment"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/metrics"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/oauth"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/search"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/vpnresource"
)

func mountPublicRoutes(r chi.Router) {
	// POST /public/tenants — tenant provisioning
	r.Post("/public/tenants", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})
	r.Get("/public/tenants", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})
	r.Get("/public/tenants/{tenantId}/exists", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})

	// Public marketplace browse
	r.Get("/api/marketplace/listings", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})
	r.Get("/api/marketplace/listings/{slug}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})

	// Public registry browse
	r.Get("/api/packages", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})
	r.Get("/api/packages/{id}", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})
}

func mountProtectedRoutes(r chi.Router, pool *pgxpool.Pool) {
	// Placeholder routes for future modules
	r.Get("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})
	r.Get("/api/skills", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})
	r.Get("/api/tools", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})

	// OAuth credentials
	oauthSvc := oauth.NewService(oauth.NewRepository(pool))
	r.Mount("/api/oauth-credentials", oauth.NewHandler(oauthSvc).Routes())

	// Audit logs (read-only HTTP; Record is called internally)
	auditSvc := audit.NewService(audit.NewRepository(pool))
	r.Mount("/api/audit-logs", audit.NewHandler(auditSvc).Routes())

	// Agent metrics
	metricsSvc := metrics.NewService(metrics.NewRepository(pool))
	metricsHandler := metrics.NewHandler(metricsSvc)
	r.Mount("/api/metrics", metricsHandler.Routes())
	// Agent-scoped metric sub-routes — mounted under /api/agents/{agentId}
	r.Route("/api/agents/{agentId}", func(r chi.Router) {
		r.Mount("/", metricsHandler.AgentRoutes())
	})

	// Prompt experiments
	experimentSvc := experiment.NewService(experiment.NewRepository(pool))
	r.Mount("/api/experiments", experiment.NewHandler(experimentSvc).Routes())

	// VPN resources
	vpnSvc := vpnresource.NewService(vpnresource.NewRepository(pool))
	r.Mount("/api/vpn-resources", vpnresource.NewHandler(vpnSvc).Routes())

	// DataSources (public API — no password)
	dsSvc := datasource.NewService(datasource.NewRepository(pool))
	dsHandler := datasource.NewHandler(dsSvc)
	r.Mount("/api/datasources", dsHandler.Routes())
	// Internal proxy endpoint — credentials including password (protect via network/auth in prod)
	r.Mount("/api/proxy/datasources", dsHandler.ProxyRoutes())

	// Global search
	searchSvc := search.NewService(pool)
	r.Mount("/api/search", search.NewHandler(searchSvc).Routes())
}
