package agentic

import (
	"encoding/json"
	"strings"
)

// readOnlyOperationVerbs lists operation values that are always safe to run
// without user confirmation. Meta-tools that multiplex several underlying
// CRUD operations via an "operation" argument (e.g. agenthub_manage,
// agent-management) are marked IsDestructive=true at the skill level because
// some of their operations do mutate state — but "list", "get" and friends
// do not, and blocking them forces the LLM to keep retrying and eventually
// fall back to narrower tools, wasting tokens.
//
// Matching is case-insensitive and trimmed. The list intentionally stays
// conservative: only verbs that are universally read-only across the whole
// AgentHub CRUD surface are included. When in doubt, leave it out — the
// default remains "block and require confirmation".
var readOnlyOperationVerbs = map[string]bool{
	"list":     true,
	"get":      true,
	"read":     true,
	"show":     true,
	"describe": true,
	"view":     true,
	"find":     true,
	"search":   true,
	"query":    true,
	"count":    true,
	"fetch":    true,
	"stat":     true,
	"stats":    true,
	"inspect":  true,
	"exists":   true,
	"check":    true,
}

// isReadOnlyOperation inspects the JSON arguments of a tool call and reports
// whether the caller requested a known read-only operation via an "operation"
// (or equivalent) field. It is used to bypass the destructive-tool block for
// safe calls on meta-tools that otherwise carry a coarse destructive flag.
//
// Returns false when arguments are empty/invalid, when no operation field is
// present, or when the operation value is not in the allow-list. That ensures
// the default behavior (block + require confirmation) is preserved whenever
// we cannot prove the call is read-only.
func isReadOnlyOperation(argumentsJSON string) bool {
	s := strings.TrimSpace(argumentsJSON)
	if s == "" || (s[0] != '{' && s[0] != '[') {
		return false
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return false
	}

	// Probe common aliases. Stick to singular "operation" / "action" — we've
	// seen both in practice (agenthub_manage uses "operation"; some MCP tools
	// use "action").
	for _, key := range []string{"operation", "action", "op", "verb", "method"} {
		raw, ok := m[key]
		if !ok {
			continue
		}
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			continue
		}
		if readOnlyOperationVerbs[strings.ToLower(strings.TrimSpace(v))] {
			return true
		}
	}
	return false
}
