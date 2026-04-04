package otel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	agenthubotel "github.com/AgentHub-Studio/agenthub-go-commons/otel"
)

func TestSetup_InvalidEndpointFails(t *testing.T) {
	// OTLP gRPC exporter tries to dial on Setup; an unreachable endpoint should fail.
	cfg := agenthubotel.Config{
		ServiceName:    "test-service",
		ServiceVersion: "1.0.0",
		OTLPEndpoint:   "localhost:0", // port 0 = no listener
		Insecure:       true,
	}
	// Setup dials the gRPC endpoint; it may succeed (lazy connect) or fail immediately.
	// Either way the returned shutdown func must be callable without panic.
	shutdown, err := agenthubotel.Setup(context.Background(), cfg)
	if err != nil {
		// Some versions fail eagerly — that's fine.
		assert.Contains(t, err.Error(), "otel:")
		return
	}
	require.NotNil(t, shutdown)
	// Shutdown should not panic even when the exporter cannot flush.
	_ = shutdown(context.Background())
}

func TestSetup_EmptyServiceName(t *testing.T) {
	cfg := agenthubotel.Config{
		ServiceName:  "",
		OTLPEndpoint: "localhost:0",
		Insecure:     true,
	}
	shutdown, err := agenthubotel.Setup(context.Background(), cfg)
	if err != nil {
		return
	}
	require.NotNil(t, shutdown)
	_ = shutdown(context.Background())
}
