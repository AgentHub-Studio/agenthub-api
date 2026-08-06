package mcp

import (
	"net/url"
	"strings"
	"testing"
)

func TestValidateRedirectURL(t *testing.T) {
	origins := []string{
		"https://app.cezar.dev",
		"https://*.cezar.dev",
		"http://localhost:4200",
	}

	for _, rawURL := range []string{
		"https://app.cezar.dev/(admin:mcp-server-configs)?reconnect=1",
		"https://tenant.cezar.dev/oauth/callback",
		"http://localhost:4200/mcp/callback",
		"https://app.cezar.dev:443/mcp/callback",
		"HTTPS://app.cezar.dev/mcp/callback",
	} {
		if err := validateRedirectURL(rawURL, origins); err != nil {
			t.Fatalf("redirect %q should be allowed: %v", rawURL, err)
		}
	}

	for _, rawURL := range []string{
		"https://attacker.example/callback",
		"https://tenant.nested.cezar.dev/callback",
		"https://cezar.dev/callback",
		"https://app.cezar.dev.attacker.example/callback",
		"https://app.cezar.dev@attacker.example/callback",
		"https://app.cezar.dev/callback#fragment",
		"ftp://app.cezar.dev/callback",
		"https://[::1",
		"/relative/callback",
		"https://app.cezar.dev/" + strings.Repeat("a", maxRedirectURLLen),
	} {
		if err := validateRedirectURL(rawURL, origins); err != ErrRedirectURLNotAllowed {
			t.Fatalf("redirect %q should be rejected, got %v", rawURL, err)
		}
	}
}

func FuzzValidateRedirectURL(f *testing.F) {
	for _, rawURL := range []string{
		"https://app.cezar.dev/callback",
		"https://tenant.cezar.dev/oauth/callback",
		"https://attacker.example/callback",
		"https://app.cezar.dev@attacker.example/callback",
		"https://[::1",
		"https://app.cezar.dev/" + strings.Repeat("a", maxRedirectURLLen),
	} {
		f.Add(rawURL)
	}

	origins := []string{
		"https://app.cezar.dev",
		"https://*.cezar.dev",
		"http://localhost:4200",
	}
	f.Fuzz(func(t *testing.T, rawURL string) {
		err := validateRedirectURL(rawURL, origins)
		if err == ErrRedirectURLNotAllowed {
			return
		}
		if err != nil {
			t.Fatalf("unexpected validation error for %q: %v", rawURL, err)
		}

		parsed, parseErr := url.Parse(rawURL)
		if parseErr != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
			t.Fatalf("accepted malformed or unsafe redirect %q", rawURL)
		}
		if !matchesFuzzAllowlist(parsed) {
			t.Fatalf("accepted redirect outside the allowlist: %q", rawURL)
		}
	})
}

func matchesFuzzAllowlist(parsed *url.URL) bool {
	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())
	port := fuzzEffectivePort(parsed)

	if scheme == "https" && host == "app.cezar.dev" && port == "443" {
		return true
	}
	if scheme == "http" && host == "localhost" && port == "4200" {
		return true
	}
	if scheme != "https" || port != "443" || !strings.HasSuffix(host, ".cezar.dev") {
		return false
	}

	label := strings.TrimSuffix(host, ".cezar.dev")
	return label != host && label != "" && !strings.Contains(label, ".")
}

func fuzzEffectivePort(parsed *url.URL) string {
	if port := parsed.Port(); port != "" {
		return port
	}
	if strings.EqualFold(parsed.Scheme, "https") {
		return "443"
	}
	if strings.EqualFold(parsed.Scheme, "http") {
		return "80"
	}
	return ""
}
