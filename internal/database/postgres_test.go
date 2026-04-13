package database

import "testing"

func TestTenantSearchPath_QuotesHyphenatedTenant(t *testing.T) {
	got := tenantSearchPath("qa-verify-final")
	want := `"ah_qa-verify-final", public`
	if got != want {
		t.Fatalf("tenantSearchPath() = %q, want %q", got, want)
	}
}

func TestTenantSearchPath_EscapesQuotes(t *testing.T) {
	got := tenantSearchPath(`foo"bar`)
	want := `"ah_foo""bar", public`
	if got != want {
		t.Fatalf("tenantSearchPath() = %q, want %q", got, want)
	}
}
