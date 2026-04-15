package agentic_test

import (
	"testing"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestResolveSystemPromptPlaceholders(t *testing.T) {
	rc := agentic.RequestContext{
		UserID:     "u-1",
		UserEmail:  "ana@example.com",
		UserRoles:  []string{"admin", "user"},
		TenantID:   "acme",
		TenantName: "Acme Inc.",
	}
	in := "Hi {{user.email}} from {{tenant.name}}. Your roles: {{user.roles}}. ID {{user.id}}@{{tenant.id}}."
	out := agentic.ResolveSystemPromptPlaceholders(in, rc)
	want := "Hi ana@example.com from Acme Inc.. Your roles: admin,user. ID u-1@acme."
	if out != want {
		t.Fatalf("got %q want %q", out, want)
	}
}

func TestResolvePreservesUnknownTokens(t *testing.T) {
	in := "{{unknown}} {{user.email}}"
	out := agentic.ResolveSystemPromptPlaceholders(in, agentic.RequestContext{UserEmail: "x@y"})
	if out != "{{unknown}} x@y" {
		t.Fatalf("got %q", out)
	}
}

func TestResolveEmptyPromptNoOp(t *testing.T) {
	if agentic.ResolveSystemPromptPlaceholders("", agentic.RequestContext{}) != "" {
		t.Fatal("empty prompt must stay empty")
	}
}
