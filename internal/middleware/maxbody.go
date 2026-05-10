package middleware

import (
	"net/http"
)

// DefaultMaxBodyBytes is the default per-request body size cap applied to
// every authenticated route. Bug 266: sem limite global, requests com 100MB
// JSON consumiriam memória do servidor durante o decode.
//
// O limite é amplo o suficiente para suportar uploads YAML/JSON de
// configuração de agent (até ~1MB) com folga de 4× para evitar regressões
// em fluxos legítimos. Endpoints que precisam de mais (uploads VPN .ovpn,
// portable agent YAML) já aplicam seu próprio MaxBytesReader com limite
// específico antes de ler o body.
const DefaultMaxBodyBytes = 4 << 20 // 4 MB

// MaxBodyBytes returns a middleware that caps Request.Body at the provided
// number of bytes. Reading past the limit fails with `http.MaxBytesError`,
// which surfaces as a 400 Bad Request when the handler tries to decode JSON.
func MaxBodyBytes(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, limit)
			}
			next.ServeHTTP(w, r)
		})
	}
}
