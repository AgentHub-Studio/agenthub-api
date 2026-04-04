// Package otel configures OpenTelemetry tracing and metrics for AgentHub Go services.
// It sets up the OTLP exporter, resource detection, and trace propagation.
//
// Usage:
//
//	shutdown, err := otel.Setup(ctx, otel.Config{
//	    ServiceName:    "agenthub-api",
//	    OTLPEndpoint:   cfg.OTLPEndpoint,
//	})
//	defer shutdown(ctx)
package otel
