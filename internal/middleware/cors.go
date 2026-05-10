package middleware

import (
	"net/http"
	"strings"
)

// CORS returns a middleware that sets CORS headers for the given allowed origins.
// Supports literal entries (e.g. "https://app.cezar.dev") and wildcard subdomain
// entries (e.g. "https://*.cezar.dev") that match any single-label subdomain.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	originsMap := make(map[string]bool, len(allowedOrigins))
	wildcardSuffixes := make([]string, 0)
	for _, o := range allowedOrigins {
		origin := strings.TrimSpace(o)
		if origin == "" || origin == "*" {
			continue
		}
		// Wildcard form: scheme://*.domain → match scheme://<sub>.domain.
		if strings.Contains(origin, "://*.") {
			wildcardSuffixes = append(wildcardSuffixes, strings.Replace(origin, "://*.", "://.", 1))
			continue
		}
		originsMap[origin] = true
	}

	allowed := func(origin string) bool {
		if originsMap[origin] {
			return true
		}
		for _, suf := range wildcardSuffixes {
			// suf = "https://.cezar.dev"; require exactly one leading subdomain label.
			schemeIdx := strings.Index(suf, "://")
			if schemeIdx < 0 || !strings.HasPrefix(origin, suf[:schemeIdx+3]) {
				continue
			}
			host := origin[schemeIdx+3:]
			suffix := suf[schemeIdx+3:] // ".cezar.dev"
			if !strings.HasSuffix(host, suffix) {
				continue
			}
			label := strings.TrimSuffix(host, suffix)
			if label != "" && !strings.ContainsAny(label, "./") {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			// Bug 277: Vary: Origin é obrigatório sempre que a resposta DEPENDE do
			// header Origin (independente de allowed/rejected). Sem isso, proxies
			// compartilhados podem servir resposta CORS de uma origem para outra.
			// Add (não Set) preserva qualquer Vary já setado downstream.
			w.Header().Add("Vary", "Origin")
			if origin != "" && allowed(origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Max-Age", "86400")

				// Mirror requested headers so any custom header the client sends is allowed.
				// This eliminates the need to maintain a fixed allowlist and handles
				// headers like X-OpenAI-API-Key, X-Tenant-ID, X-Request-ID, etc.
				if requested := r.Header.Get("Access-Control-Request-Headers"); requested != "" {
					w.Header().Set("Access-Control-Allow-Headers", requested)
				} else {
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, X-Request-ID, Cache-Control")
				}
			}
			// BUG-DEPR2 fix: intercept ALL OPTIONS requests here so the router
			// wildcard r.Options("/*") can be removed. Without that wildcard,
			// chi no longer registers a method for unregistered paths, meaning
			// GET/POST to non-existent routes correctly returns 404 (not 405).
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
