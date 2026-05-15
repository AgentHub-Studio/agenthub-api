package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCoreTelemetrySink_SeedExpectedSlugs_NoDuplicates(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range SeedExpectedTelemetrySinkSlugs {
		assert.False(t, seen[s], "duplicate slug %q", s)
		seen[s] = true
	}
}

func TestCoreTelemetrySink_SeedExpectedSlugs_AllNonEmpty(t *testing.T) {
	for i, s := range SeedExpectedTelemetrySinkSlugs {
		assert.NotEmpty(t, s, "slug at %d must be non-empty", i)
	}
}

func TestCoreTelemetrySink_SeedExpectedSlugs_CanonicalCount(t *testing.T) {
	// 9 sinks = 3 traces + 2 metrics + 1 logs + 3 webhooks.
	assert.Equal(t, 9, len(SeedExpectedTelemetrySinkSlugs))
}

func TestCoreTelemetrySink_KindsAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range SeedExpectedTelemetrySinkKinds {
		assert.False(t, seen[k], "duplicate kind %q", k)
		seen[k] = true
	}
}

func TestCoreTelemetrySink_KindsCanonicalCount(t *testing.T) {
	// 6 kinds: traces / metrics / logs / events / quality_reports / audit.
	assert.Equal(t, 6, len(SeedExpectedTelemetrySinkKinds))
}

func TestCoreTelemetrySink_ProtocolsAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, p := range SeedExpectedTelemetrySinkProtocols {
		assert.False(t, seen[p], "duplicate protocol %q", p)
		seen[p] = true
	}
}

func TestCoreTelemetrySink_ProtocolsCoverObservabilityStack(t *testing.T) {
	allowed := map[string]bool{}
	for _, p := range SeedExpectedTelemetrySinkProtocols {
		allowed[p] = true
	}
	// Stack from CLAUDE.md: OpenTelemetry + Prometheus + Grafana.
	for _, want := range []string{"otlp_http", "otlp_grpc", "prometheus_remote_write", "loki_push", "webhook"} {
		assert.True(t, allowed[want], "protocol %q must be in seed", want)
	}
}

func TestCoreTelemetrySink_AuthTypesAreClosed(t *testing.T) {
	allowed := map[string]bool{}
	for _, a := range SeedExpectedTelemetrySinkAuthTypes {
		allowed[a] = true
	}
	for _, want := range []string{"none", "api_key", "bearer_token", "basic", "oauth2", "mtls"} {
		assert.True(t, allowed[want])
	}
}

func TestCoreTelemetrySink_SlugsAreFilesystemSafe(t *testing.T) {
	for _, s := range SeedExpectedTelemetrySinkSlugs {
		for _, r := range s {
			ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
			assert.True(t, ok, "slug %q has invalid char %q", s, r)
		}
		assert.False(t, strings.Contains(s, "_"),
			"slug %q must use kebab-case", s)
	}
}
