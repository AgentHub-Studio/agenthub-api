package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
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

func mountProtectedRoutes(r chi.Router) {
	// Protected routes — implemented in domain modules
	r.Get("/api/agents", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})
	r.Get("/api/skills", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})
	r.Get("/api/tools", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not implemented"}`, http.StatusNotImplemented)
	})
}
