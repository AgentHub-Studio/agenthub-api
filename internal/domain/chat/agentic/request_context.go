package agentic

import (
	"regexp"
	"strings"
)

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
// Unknown placeholders outside known namespaces are left untouched (so prose
// like "{{ note }}" is safe). Unknown user.* and tenant.* placeholders resolve
// to an empty string and emit a warning.
type RequestContext struct {
	UserID     string
	UserEmail  string
	UserRoles  []string
	TenantID   string
	TenantName string
}

var systemPromptPlaceholderRe = regexp.MustCompile(`\{\{\s*([A-Za-z][A-Za-z0-9_.-]*)\s*\}\}`)

// ResolveSystemPromptPlaceholders replaces known {{...}} tokens in prompt using rc.
// Empty RequestContext fields resolve to "" (the token is removed).
func ResolveSystemPromptPlaceholders(prompt string, rc RequestContext) string {
	out, _ := ResolveSystemPromptPlaceholdersWithWarnings(prompt, rc)
	return out
}

// ResolveSystemPromptPlaceholdersWithWarnings resolves known placeholders and
// returns warnings for unknown placeholders under the reserved user.* and
// tenant.* namespaces.
func ResolveSystemPromptPlaceholdersWithWarnings(prompt string, rc RequestContext) (string, []string) {
	if prompt == "" {
		return prompt, nil
	}
	replacements := map[string]string{
		"user.id":     rc.UserID,
		"user.email":  rc.UserEmail,
		"user.roles":  strings.Join(rc.UserRoles, ","),
		"tenant.id":   rc.TenantID,
		"tenant.name": rc.TenantName,
	}
	var warnings []string
	out := systemPromptPlaceholderRe.ReplaceAllStringFunc(prompt, func(match string) string {
		sub := systemPromptPlaceholderRe.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		key := sub[1]
		if value, ok := replacements[key]; ok {
			return value
		}
		if strings.HasPrefix(key, "user.") || strings.HasPrefix(key, "tenant.") {
			warnings = append(warnings, "unresolved_placeholder:"+key)
			return ""
		}
		return match
	})
	return out, warnings
}
