package agentic

import "strings"

// RequestContext holds per-request identity used to resolve placeholders in an
// agent's system prompt at runtime. Mastra exposes the same concept via
// `DynamicArgument` — AgentHub mirrors only the subset of fields that exist
// in this repo's JWT/tenant plumbing.
//
// Placeholder grammar (double-mustache):
//
//	{{user.email}}        → UserEmail
//	{{user.id}}           → UserID
//	{{user.roles}}        → comma-joined UserRoles (preserves order)
//	{{tenant.id}}         → TenantID
//	{{tenant.name}}       → TenantName
//
// Unknown placeholders are left untouched (so prose like "{{ note }}" is safe).
type RequestContext struct {
	UserID     string
	UserEmail  string
	UserRoles  []string
	TenantID   string
	TenantName string
}

// ResolveSystemPromptPlaceholders replaces known {{...}} tokens in prompt using rc.
// Empty RequestContext fields resolve to "" (the token is removed).
func ResolveSystemPromptPlaceholders(prompt string, rc RequestContext) string {
	if prompt == "" {
		return prompt
	}
	replacements := []struct{ token, value string }{
		{"{{user.id}}", rc.UserID},
		{"{{user.email}}", rc.UserEmail},
		{"{{user.roles}}", strings.Join(rc.UserRoles, ",")},
		{"{{tenant.id}}", rc.TenantID},
		{"{{tenant.name}}", rc.TenantName},
	}
	out := prompt
	for _, r := range replacements {
		out = strings.ReplaceAll(out, r.token, r.value)
	}
	return out
}
