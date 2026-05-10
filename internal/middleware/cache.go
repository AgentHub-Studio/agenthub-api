package middleware

import "net/http"

// NoStoreCache adiciona Cache-Control: no-store em respostas autenticadas.
// Bug 280: sem esse header, dados privados de tenant podem ser servidos do
// browser back/forward cache após logout ou troca de conta — o usuário B
// vê listings que pertenciam ao usuário A em /api/agents.
//
// `no-store` é mais restritivo que `private` ou `no-cache`:
//   - no-store: nada é armazenado em qualquer cache
//   - private: pode ser cacheado por browser, mas não por proxy
//   - no-cache: pode ser cacheado mas exige revalidação
//
// Para listagens GET autenticadas em multi-tenant SaaS, no-store é a
// escolha segura — bem alinhado com as práticas de OWASP A05:2021.
//
// Aplicado no Protected stack (authenticated routes); endpoints Public
// como /api/healthz e /public/tenants não recebem (são genuinely cacheable).
func NoStoreCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
