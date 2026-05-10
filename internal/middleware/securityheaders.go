package middleware

import "net/http"

// SecurityHeaders adiciona cabeçalhos de segurança defensivos em toda resposta.
// Bug 276: defesa em profundidade contra MIME sniffing e clickjacking.
//
// Cabeçalhos adicionados:
//   - X-Content-Type-Options: nosniff   — impede MIME sniffing (RFC ietf-httpapi)
//   - X-Frame-Options: DENY              — impede embedding em <iframe> (clickjacking)
//   - Referrer-Policy: no-referrer       — não vaza URL completa em links externos
//
// Não adiciona Content-Security-Policy ou HSTS porque:
//   - CSP é mais útil em frontend (que tem seu próprio servidor); API JSON não renderiza UI
//   - HSTS é responsabilidade do ingress/gateway TLS-terminating
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}
