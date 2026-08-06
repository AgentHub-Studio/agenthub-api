package agentic_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode"

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

func TestResolveNamespacedUnknownPlaceholderReturnsWarning(t *testing.T) {
	in := "Olá {{user.unknown_field}} de {{tenant.unknown_field}}."
	out, warnings := agentic.ResolveSystemPromptPlaceholdersWithWarnings(in, agentic.RequestContext{})
	if out != "Olá  de ." {
		t.Fatalf("got %q", out)
	}
	want := []string{
		"unresolved_placeholder:user.unknown_field",
		"unresolved_placeholder:tenant.unknown_field",
	}
	if len(warnings) != len(want) {
		t.Fatalf("got warnings %v want %v", warnings, want)
	}
	for i := range want {
		if warnings[i] != want[i] {
			t.Fatalf("got warnings %v want %v", warnings, want)
		}
	}
}

func TestResolveSystemPromptPlaceholdersAllowsWhitespace(t *testing.T) {
	rc := agentic.RequestContext{
		UserEmail:  "ana@example.com",
		UserRoles:  []string{"admin", "viewer"},
		TenantName: "Acme",
	}
	in := "{{ user.email }}|{{ user.roles }}|{{ tenant.name }}"
	out := agentic.ResolveSystemPromptPlaceholders(in, rc)
	if out != "ana@example.com|admin,viewer|Acme" {
		t.Fatalf("got %q", out)
	}
}

func TestResolveEmptyPromptNoOp(t *testing.T) {
	if agentic.ResolveSystemPromptPlaceholders("", agentic.RequestContext{}) != "" {
		t.Fatal("empty prompt must stay empty")
	}
}

func FuzzResolveSystemPromptPlaceholdersGeneratedTokens(f *testing.F) {
	f.Add("ana@example.com", "admin", "tenant-1", "Acme", "missing", "note")
	f.Add("dev@example.org", "viewer", "acme", "Acme_Inc", "other_field", "memo")
	f.Add("ops@example.net", "support", "tenant42", "Tenant42", "flag-1", "body")

	f.Fuzz(func(t *testing.T, rawEmail string, rawRole string, rawTenantID string, rawTenantName string, rawReserved string, rawPublic string) {
		rc := agentic.RequestContext{
			UserID:     safePromptValue(rawTenantID, "user-1"),
			UserEmail:  safePromptValue(rawEmail, "ana@example.com"),
			UserRoles:  []string{safePromptValue(rawRole, "viewer")},
			TenantID:   safePromptValue(rawTenantID, "tenant-1"),
			TenantName: safePromptValue(rawTenantName, "Acme"),
		}
		reservedKey := "missing_" + safePlaceholderSegment(rawReserved, "field")
		publicKey := "note." + safePlaceholderSegment(rawPublic, "body")
		publicToken := "{{" + publicKey + "}}"
		prompt := fmt.Sprintf(
			"email={{ user.email }} roles={{user.roles}} tenant={{ tenant.name }} missing={{user.%s}} keep=%s",
			reservedKey,
			publicToken,
		)

		got, warnings := agentic.ResolveSystemPromptPlaceholdersWithWarnings(prompt, rc)
		for _, want := range []string{
			"email=" + rc.UserEmail,
			"roles=" + strings.Join(rc.UserRoles, ","),
			"tenant=" + rc.TenantName,
			"keep=" + publicToken,
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("resolved prompt %q missing %q", got, want)
			}
		}
		if strings.Contains(got, "{{ user.email }}") || strings.Contains(got, "{{user."+reservedKey+"}}") {
			t.Fatalf("resolved prompt leaked reserved placeholder: %q", got)
		}

		wantWarning := "unresolved_placeholder:user." + reservedKey
		if len(warnings) != 1 || warnings[0] != wantWarning {
			t.Fatalf("warnings = %v, want [%s]", warnings, wantWarning)
		}
	})
}

func safePromptValue(raw string, fallback string) string {
	var b strings.Builder
	for _, r := range raw {
		if b.Len() >= 40 {
			break
		}
		if r > unicode.MaxASCII {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '@' || r == '.' || r == '_' || r == '-' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	return b.String()
}

func safePlaceholderSegment(raw string, fallback string) string {
	var b strings.Builder
	for _, r := range raw {
		if b.Len() >= 24 {
			break
		}
		if r > unicode.MaxASCII {
			continue
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	if b.Len() == 0 {
		return fallback
	}
	return b.String()
}
