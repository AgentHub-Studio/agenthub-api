// Package auth implements RBAC capabilities and wildcard permission matching.
//
// Permission grammar: "<resource>:<action>[:<id>]".
// Wildcards: "*" matches any single segment. Examples:
//   granted=agents:*       required=agents:read   → match
//   granted=*:read         required=agents:read   → match
//   granted=agents:read    required=agents:write  → no match
//   granted=agents:*:my-a  required=agents:read:my-a → match
package auth

import "strings"

// MatchesPermission reports whether granted satisfies required.
// Both are colon-delimited strings of equal segment count (2 or 3). "*" is a
// single-segment wildcard. A granted shorter than required implicitly matches
// trailing segments of required; e.g. "agents:*" matches "agents:read:my-a".
func MatchesPermission(granted, required string) bool {
	g := strings.Split(granted, ":")
	r := strings.Split(required, ":")
	if len(g) > len(r) {
		return false
	}
	for i, seg := range g {
		if seg == "*" {
			continue
		}
		if seg != r[i] {
			return false
		}
	}
	return true
}

// HasAny reports whether any granted permission matches the required one.
func HasAny(granted []string, required string) bool {
	for _, g := range granted {
		if MatchesPermission(g, required) {
			return true
		}
	}
	return false
}

// DefaultPermissionsForRoles expands coarse Keycloak realm roles into the
// fine-grained permission set used by the capabilities endpoint and the
// Angular `*ahHasPermission` directive.
//
// Tables in the Mastra reference use the same shape; AgentHub mirrors only
// the subset that exists in this repo (agents, skills, knowledgeBases, audit).
func DefaultPermissionsForRoles(roles []string) []string {
	perms := map[string]struct{}{}
	add := func(p ...string) {
		for _, x := range p {
			perms[x] = struct{}{}
		}
	}
	for _, role := range roles {
		switch role {
		case "admin":
			add(
				"agents:*", "skills:*", "knowledgeBases:*",
				"audit:read", "users:*", "acl:*",
				"settings:*", "mcpServers:*", "dataSources:*",
			)
		case "user":
			add(
				"agents:read", "agents:write",
				"skills:read",
				"knowledgeBases:read",
				"chat:*",
			)
		case "mcp-client-runtime":
			add("mcpServers:read", "mcpServers:configs:read")
		case "PROXY_SERVICE":
			add("dataSources:read:credentials")
		}
	}
	out := make([]string, 0, len(perms))
	for p := range perms {
		out = append(out, p)
	}
	return out
}
