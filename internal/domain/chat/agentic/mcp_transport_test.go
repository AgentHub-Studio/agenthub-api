package agentic

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPTransportType_AllCount(t *testing.T) {
	assert.Equal(t, 2, len(AllMCPTransportTypes))
}

func TestMCPTransportType_IsValid(t *testing.T) {
	assert.True(t, MCPTransportStdio.IsValid())
	assert.True(t, MCPTransportStreamableHTTP.IsValid())
	assert.False(t, MCPTransportType("ws").IsValid())
	assert.False(t, MCPTransportType("").IsValid())
}

func TestMCPTransportType_StringValues(t *testing.T) {
	assert.Equal(t, "stdio", string(MCPTransportStdio))
	assert.Equal(t, "streamable_http", string(MCPTransportStreamableHTTP))
}

func TestMCPContentNegotiation_AllCount(t *testing.T) {
	assert.Equal(t, 2, len(AllMCPContentNegotiations))
}

func TestMCPContentNegotiation_IsValid(t *testing.T) {
	assert.True(t, MCPContentJSON.IsValid())
	assert.True(t, MCPContentSSE.IsValid())
	assert.False(t, MCPContentNegotiation("text/plain").IsValid())
	assert.False(t, MCPContentNegotiation("").IsValid())
}

func TestMCPConnectionState_AllCount(t *testing.T) {
	assert.Equal(t, 4, len(AllMCPConnectionStates))
}

func TestMCPConnectionState_IsValid(t *testing.T) {
	assert.True(t, MCPConnectionDisconnected.IsValid())
	assert.True(t, MCPConnectionConnecting.IsValid())
	assert.True(t, MCPConnectionConnected.IsValid())
	assert.True(t, MCPConnectionError.IsValid())
	assert.False(t, MCPConnectionState("unknown").IsValid())
}

func TestCapabilitiesFor_Stdio(t *testing.T) {
	caps := CapabilitiesFor(MCPTransportStdio)
	assert.False(t, caps.SupportsSSE)
	assert.False(t, caps.SupportsStreaming)
	assert.True(t, caps.RequiresSubprocess)
	assert.Equal(t, 1, caps.MaxConcurrentRequests)
}

func TestCapabilitiesFor_StreamableHTTP(t *testing.T) {
	caps := CapabilitiesFor(MCPTransportStreamableHTTP)
	assert.True(t, caps.SupportsSSE)
	assert.True(t, caps.SupportsStreaming)
	assert.False(t, caps.RequiresSubprocess)
	assert.Equal(t, 0, caps.MaxConcurrentRequests)
}

func TestMCPTransportConfig_Validate_StdioHappy(t *testing.T) {
	cfg := DefaultStdioConfig("npx", "-y", "@modelcontextprotocol/server-filesystem", "/tmp")
	require.NoError(t, cfg.Validate())
}

func TestMCPTransportConfig_Validate_HTTPHappy(t *testing.T) {
	cfg := DefaultStreamableHTTPConfig("https://mcp.example.com")
	require.NoError(t, cfg.Validate())
}

func TestMCPTransportConfig_Validate_InvalidType(t *testing.T) {
	cfg := &MCPTransportConfig{
		TransportType:  "ws",
		ConnectTimeout: 5 * time.Second,
	}
	assert.ErrorIs(t, cfg.Validate(), ErrMCPTransportTypeInvalid)
}

func TestMCPTransportConfig_Validate_StdioEmptyCommand(t *testing.T) {
	cfg := DefaultStdioConfig("")
	assert.ErrorIs(t, cfg.Validate(), ErrMCPTransportCommandEmpty)
}

func TestMCPTransportConfig_Validate_HTTPEmptyURL(t *testing.T) {
	cfg := DefaultStreamableHTTPConfig("")
	assert.ErrorIs(t, cfg.Validate(), ErrMCPTransportBaseURLEmpty)
}

func TestMCPTransportConfig_Validate_HTTPBadScheme(t *testing.T) {
	cfg := DefaultStreamableHTTPConfig("ftp://bad.example.com")
	assert.ErrorIs(t, cfg.Validate(), ErrMCPTransportBaseURLScheme)
}

func TestMCPTransportConfig_Validate_ZeroTimeout(t *testing.T) {
	cfg := &MCPTransportConfig{
		TransportType:  MCPTransportStdio,
		Command:        "npx",
		ConnectTimeout: 0,
	}
	assert.ErrorIs(t, cfg.Validate(), ErrMCPTransportTimeoutZero)
}

func TestDefaultStdioConfig_Fields(t *testing.T) {
	cfg := DefaultStdioConfig("uvx", "mcp-server-sqlite", "--db-path", "/tmp/test.db")
	assert.Equal(t, MCPTransportStdio, cfg.TransportType)
	assert.Equal(t, "uvx", cfg.Command)
	assert.Equal(t, []string{"mcp-server-sqlite", "--db-path", "/tmp/test.db"}, cfg.Args)
	assert.True(t, cfg.AutoStart)
	assert.Greater(t, cfg.ConnectTimeout, time.Duration(0))
}

func TestDefaultStreamableHTTPConfig_Fields(t *testing.T) {
	cfg := DefaultStreamableHTTPConfig("https://mcp.example.com")
	assert.Equal(t, MCPTransportStreamableHTTP, cfg.TransportType)
	assert.Equal(t, "https://mcp.example.com", cfg.BaseURL)
	assert.Equal(t, MCPContentSSE, cfg.DefaultNegotiation)
	assert.False(t, cfg.AutoStart)
}
