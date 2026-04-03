package agentic_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestParsePermissionRules_Empty(t *testing.T) {
	assert.Nil(t, agentic.ParsePermissionRules(nil))
	assert.Nil(t, agentic.ParsePermissionRules(json.RawMessage(`{}`)))
	assert.Nil(t, agentic.ParsePermissionRules(json.RawMessage(`invalid`)))
}

func TestParsePermissionRules_Valid(t *testing.T) {
	raw := json.RawMessage(`{"allow":["execute-sql"],"deny":["drop-*"]}`)
	rules := agentic.ParsePermissionRules(raw)
	assert.NotNil(t, rules)
	assert.Equal(t, []string{"execute-sql"}, rules.Allow)
	assert.Equal(t, []string{"drop-*"}, rules.Deny)
	assert.Empty(t, rules.Confirm)
}

func TestEvaluatePermission_NilRules(t *testing.T) {
	assert.Equal(t, agentic.PermissionAllow, agentic.EvaluatePermission(nil, "anything", ""))
}

func TestEvaluatePermission_DenyTakesPriority(t *testing.T) {
	rules := &agentic.PermissionRules{
		Allow: []string{"*"},
		Deny:  []string{"execute-sql"},
	}
	assert.Equal(t, agentic.PermissionDeny, agentic.EvaluatePermission(rules, "execute-sql", ""))
	assert.Equal(t, agentic.PermissionAllow, agentic.EvaluatePermission(rules, "document-search", ""))
}

func TestEvaluatePermission_ConfirmBeforeAllow(t *testing.T) {
	rules := &agentic.PermissionRules{
		Allow:   []string{"*"},
		Confirm: []string{"execute-sql(DELETE)"},
	}
	assert.Equal(t, agentic.PermissionConfirm, agentic.EvaluatePermission(rules, "execute-sql", `{"query":"DELETE FROM users"}`))
	assert.Equal(t, agentic.PermissionAllow, agentic.EvaluatePermission(rules, "execute-sql", `{"query":"SELECT 1"}`))
}

func TestEvaluatePermission_AllowListRestricts(t *testing.T) {
	rules := &agentic.PermissionRules{
		Allow: []string{"document-search", "memory-store"},
	}
	assert.Equal(t, agentic.PermissionAllow, agentic.EvaluatePermission(rules, "document-search", ""))
	assert.Equal(t, agentic.PermissionAllow, agentic.EvaluatePermission(rules, "memory-store", ""))
	assert.Equal(t, agentic.PermissionDeny, agentic.EvaluatePermission(rules, "execute-sql", ""))
}

func TestEvaluatePermission_GlobPatterns(t *testing.T) {
	rules := &agentic.PermissionRules{
		Allow: []string{"http-*", "document-search"},
	}
	assert.Equal(t, agentic.PermissionAllow, agentic.EvaluatePermission(rules, "http-get", ""))
	assert.Equal(t, agentic.PermissionAllow, agentic.EvaluatePermission(rules, "http-post", ""))
	assert.Equal(t, agentic.PermissionDeny, agentic.EvaluatePermission(rules, "execute-sql", ""))
}

func TestEvaluatePermission_InputPatternMatch(t *testing.T) {
	rules := &agentic.PermissionRules{
		Deny: []string{"execute-sql(DROP TABLE)"},
	}
	// Should deny when input contains the pattern.
	assert.Equal(t, agentic.PermissionDeny, agentic.EvaluatePermission(rules, "execute-sql", `{"query":"DROP TABLE users"}`))
	// Should allow when input doesn't contain pattern.
	assert.Equal(t, agentic.PermissionAllow, agentic.EvaluatePermission(rules, "execute-sql", `{"query":"SELECT * FROM users"}`))
}

func TestEvaluatePermission_CaseInsensitiveInput(t *testing.T) {
	rules := &agentic.PermissionRules{
		Deny: []string{"execute-sql(drop table)"},
	}
	assert.Equal(t, agentic.PermissionDeny, agentic.EvaluatePermission(rules, "execute-sql", `DROP TABLE users`))
}

func TestEvaluatePermission_NoAllowRulesDefaultAllow(t *testing.T) {
	rules := &agentic.PermissionRules{}
	assert.Equal(t, agentic.PermissionAllow, agentic.EvaluatePermission(rules, "anything", ""))
}

func TestEvaluatePermission_WildcardAllow(t *testing.T) {
	rules := &agentic.PermissionRules{
		Allow: []string{"*"},
		Deny:  []string{"dangerous-tool"},
	}
	assert.Equal(t, agentic.PermissionAllow, agentic.EvaluatePermission(rules, "safe-tool", ""))
	assert.Equal(t, agentic.PermissionDeny, agentic.EvaluatePermission(rules, "dangerous-tool", ""))
}

func TestFormatDeniedError(t *testing.T) {
	msg := agentic.FormatDeniedError("execute-sql")
	assert.Contains(t, msg, "execute-sql")
	assert.Contains(t, msg, "not permitted")
}
