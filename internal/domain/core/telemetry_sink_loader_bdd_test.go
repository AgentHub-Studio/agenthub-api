package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreTelemetrySinkSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsObservabilityStackCatalog", func(t *testing.T) {
		// Given a fresh tenant needs to wire OBS-001 events / OBS-009
		//       quality reports / GOV-005 audit somewhere,
		// When ah_core sinks are loaded,
		// Then ≥6 catalog entries appear so the tenant has options
		//      for traces, metrics, logs, events, quality, audit.
		assert.GreaterOrEqual(t, len(SeedExpectedTelemetrySinkSlugs), 6)
	})

	t.Run("Scenario_AllSixTelemetryKindsAreCovered", func(t *testing.T) {
		// Given OBS-001 envelope + OBS-009 reports + GOV-001 audit are
		//       distinct concerns,
		// When the seed kinds are inspected,
		// Then 6 distinct kinds exist: traces / metrics / logs /
		//      events / quality_reports / audit.
		set := map[string]bool{}
		for _, k := range SeedExpectedTelemetrySinkKinds {
			set[k] = true
		}
		for _, want := range []string{
			"traces", "metrics", "logs", "events", "quality_reports", "audit",
		} {
			assert.True(t, set[want], "kind %q must be in seed", want)
		}
	})

	t.Run("Scenario_OpenTelemetryFirstClassForTraces", func(t *testing.T) {
		// Given CLAUDE.md says OpenTelemetry is the trace stack,
		// When the seed traces are inspected,
		// Then both OTLP HTTP + OTLP gRPC + Jaeger native are present.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedTelemetrySinkSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["otel-collector-traces"], "OTel collector required")
		assert.True(t, seedSet["jaeger-traces"], "Jaeger required (CLAUDE.md stack)")
		assert.True(t, seedSet["honeycomb-traces"], "Honeycomb cloud option required")
	})

	t.Run("Scenario_PrometheusFirstClassForMetrics", func(t *testing.T) {
		// Given CLAUDE.md says Prometheus is the metric stack,
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedTelemetrySinkSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["prometheus-remote-write"], "Prometheus self-hosted required")
		assert.True(t, seedSet["grafana-cloud-metrics"], "Grafana Cloud option required")
	})

	t.Run("Scenario_LokiFirstClassForLogs", func(t *testing.T) {
		// Given CLAUDE.md says Grafana stack is the log destination,
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedTelemetrySinkSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["loki-push"], "Loki required for log shipping")
	})

	t.Run("Scenario_GenericWebhooksAvailableForCustomIntegrations", func(t *testing.T) {
		// Given tenants may want to integrate with a non-vendor system
		//       (compliance pipeline, internal alerting),
		// When the seed is inspected,
		// Then 3 webhook sinks exist (events / quality_reports / audit)
		//      so tenants can wire any HTTP endpoint.
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedTelemetrySinkSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["webhook-events"])
		assert.True(t, seedSet["webhook-quality-reports"])
		assert.True(t, seedSet["webhook-audit"])
	})

	t.Run("Scenario_QualityReportSinkBridgesOBS009", func(t *testing.T) {
		// Given OBS-009 produces QualityReports + tenants want them
		//       posted to compliance/alerting,
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedTelemetrySinkSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["webhook-quality-reports"],
			"OBS-009 quality reports need a sink path")
	})

	t.Run("Scenario_AuditSinkBridgesGOV001AndGOV005", func(t *testing.T) {
		// Given GOV-001 audit trail + GOV-005 governance report need
		//       to reach external compliance systems,
		seedSet := map[string]bool{}
		for _, s := range SeedExpectedTelemetrySinkSlugs {
			seedSet[s] = true
		}
		assert.True(t, seedSet["webhook-audit"],
			"audit sink required for external compliance pipeline")
	})

	t.Run("Scenario_AuthTypesCoverMajorVendorRequirements", func(t *testing.T) {
		// Given different vendors need different credentials
		//       (Grafana Cloud uses basic, Honeycomb uses api_key,
		//       custom webhooks use bearer_token),
		set := map[string]bool{}
		for _, a := range SeedExpectedTelemetrySinkAuthTypes {
			set[a] = true
		}
		for _, want := range []string{"none", "api_key", "bearer_token", "basic"} {
			assert.True(t, set[want], "auth type %q must be in seed", want)
		}
	})

	t.Run("Scenario_SeedCountIsStableAcrossRefactors", func(t *testing.T) {
		// Given external systems bind to count = 9,
		assert.Equal(t, 9, len(SeedExpectedTelemetrySinkSlugs))
	})
}
