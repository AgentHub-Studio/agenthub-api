package agentic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BDD: EXT-008a MCP multi-transport (Streamable HTTP spec 2025-03-26)

func TestBDD_EXT008a_StdioSubprocessConstraint(t *testing.T) {
	// GIVEN a stdio transport config
	// WHEN capabilities are queried
	// THEN subprocess is required and concurrency is 1 (sequential by design)
	caps := CapabilitiesFor(MCPTransportStdio)
	assert.True(t, caps.RequiresSubprocess, "stdio must require subprocess")
	assert.Equal(t, 1, caps.MaxConcurrentRequests, "stdio allows only 1 concurrent request")
	assert.False(t, caps.SupportsSSE, "stdio does not support SSE streaming")
}

func TestBDD_EXT008a_StreamableHTTPSupportsSSE(t *testing.T) {
	// GIVEN a streamable_http transport
	// WHEN an agent chat session requires incremental text delivery
	// THEN the transport declares SSE and streaming support
	caps := CapabilitiesFor(MCPTransportStreamableHTTP)
	assert.True(t, caps.SupportsSSE)
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.RequiresSubprocess)
}

func TestBDD_EXT008a_StreamableHTTPDefaultsToSSEAcceptHeader(t *testing.T) {
	// GIVEN a default HTTP transport config
	// WHEN no negotiation is explicitly set
	// THEN SSE is preferred over plain JSON
	cfg := DefaultStreamableHTTPConfig("https://mcp.example.com")
	assert.Equal(t, MCPContentSSE, cfg.DefaultNegotiation,
		"streamable_http should default to SSE for incremental delivery")
}

func TestBDD_EXT008a_StdioAutoStartEnabled(t *testing.T) {
	// GIVEN a stdio transport for a subprocess-based MCP server
	// WHEN using default config
	// THEN AutoStart is true so the subprocess starts on first use
	cfg := DefaultStdioConfig("npx", "-y", "@modelcontextprotocol/server-github")
	assert.True(t, cfg.AutoStart)
}

func TestBDD_EXT008a_HTTPAutoStartDisabled(t *testing.T) {
	// GIVEN a streamable_http transport pointing at a remote server
	// WHEN using default config
	// THEN AutoStart is false because the remote server manages its own lifecycle
	cfg := DefaultStreamableHTTPConfig("https://mcp.remote.example.com")
	assert.False(t, cfg.AutoStart)
}

func TestBDD_EXT008a_InvalidConfigRejected(t *testing.T) {
	cases := []struct {
		name    string
		cfg     *MCPTransportConfig
		wantErr error
	}{
		{
			name: "unknown transport type",
			cfg: &MCPTransportConfig{
				TransportType: "websocket", ConnectTimeout: 5 * time.Second,
			},
			wantErr: ErrMCPTransportTypeInvalid,
		},
		{
			name:    "stdio without command",
			cfg:     DefaultStdioConfig(""),
			wantErr: ErrMCPTransportCommandEmpty,
		},
		{
			name:    "http with ftp scheme",
			cfg:     DefaultStreamableHTTPConfig("ftp://bad.example.com"),
			wantErr: ErrMCPTransportBaseURLScheme,
		},
		{
			name:    "http without url",
			cfg:     DefaultStreamableHTTPConfig(""),
			wantErr: ErrMCPTransportBaseURLEmpty,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestBDD_EXT008a_ConnectionStateLifecycle(t *testing.T) {
	// GIVEN the full set of connection states
	// THEN they cover disconnected → connecting → connected → error lifecycle
	required := []MCPConnectionState{
		MCPConnectionDisconnected,
		MCPConnectionConnecting,
		MCPConnectionConnected,
		MCPConnectionError,
	}
	for _, s := range required {
		assert.True(t, s.IsValid(), "state %q must be valid", s)
	}
}
