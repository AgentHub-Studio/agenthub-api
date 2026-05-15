package agentic

import (
	"errors"
	"strings"
	"time"
)

// MCPTransportType identifies the wire protocol used to reach an MCP server.
type MCPTransportType string

const (
	// MCPTransportStdio communicates via stdin/stdout of a subprocess.
	MCPTransportStdio MCPTransportType = "stdio"
	// MCPTransportStreamableHTTP communicates via the MCP 2025-03-26 Streamable HTTP
	// transport: a single POST /mcp endpoint with Accept-header content negotiation.
	MCPTransportStreamableHTTP MCPTransportType = "streamable_http"
)

// AllMCPTransportTypes lists every valid transport type.
var AllMCPTransportTypes = []MCPTransportType{
	MCPTransportStdio,
	MCPTransportStreamableHTTP,
}

// IsValid returns true for recognised transport values.
func (t MCPTransportType) IsValid() bool {
	for _, v := range AllMCPTransportTypes {
		if t == v {
			return true
		}
	}
	return false
}

// MCPContentNegotiation describes the Accept header a Streamable HTTP client sends.
type MCPContentNegotiation string

const (
	// MCPContentJSON requests a synchronous JSON response body.
	MCPContentJSON MCPContentNegotiation = "application/json"
	// MCPContentSSE requests a Server-Sent Events stream.
	MCPContentSSE MCPContentNegotiation = "text/event-stream"
)

// AllMCPContentNegotiations lists every valid content-negotiation value.
var AllMCPContentNegotiations = []MCPContentNegotiation{
	MCPContentJSON,
	MCPContentSSE,
}

// IsValid returns true for recognised negotiation values.
func (c MCPContentNegotiation) IsValid() bool {
	for _, v := range AllMCPContentNegotiations {
		if c == v {
			return true
		}
	}
	return false
}

// MCPConnectionState tracks the lifecycle of a transport connection.
type MCPConnectionState string

const (
	MCPConnectionDisconnected MCPConnectionState = "disconnected"
	MCPConnectionConnecting   MCPConnectionState = "connecting"
	MCPConnectionConnected    MCPConnectionState = "connected"
	MCPConnectionError        MCPConnectionState = "error"
)

// AllMCPConnectionStates lists every valid connection state.
var AllMCPConnectionStates = []MCPConnectionState{
	MCPConnectionDisconnected,
	MCPConnectionConnecting,
	MCPConnectionConnected,
	MCPConnectionError,
}

// IsValid returns true for recognised state values.
func (s MCPConnectionState) IsValid() bool {
	for _, v := range AllMCPConnectionStates {
		if s == v {
			return true
		}
	}
	return false
}

// MCPTransportCapabilities describes what an MCP transport supports.
type MCPTransportCapabilities struct {
	// SupportsSSE is true when the transport can deliver Server-Sent Events
	// (only streamable_http with text/event-stream).
	SupportsSSE bool
	// SupportsStreaming indicates that partial/incremental results can be delivered.
	SupportsStreaming bool
	// RequiresSubprocess is true for stdio transports that launch a child process.
	RequiresSubprocess bool
	// MaxConcurrentRequests is the suggested concurrency cap (0 = unlimited).
	MaxConcurrentRequests int
}

// CapabilitiesFor returns the built-in capability profile for a transport type.
func CapabilitiesFor(t MCPTransportType) MCPTransportCapabilities {
	switch t {
	case MCPTransportStreamableHTTP:
		return MCPTransportCapabilities{
			SupportsSSE:           true,
			SupportsStreaming:      true,
			RequiresSubprocess:    false,
			MaxConcurrentRequests: 0,
		}
	default: // stdio
		return MCPTransportCapabilities{
			SupportsSSE:           false,
			SupportsStreaming:      false,
			RequiresSubprocess:    true,
			MaxConcurrentRequests: 1,
		}
	}
}

// Sentinel errors for MCPTransportConfig validation.
var (
	ErrMCPTransportTypeInvalid   = errors.New("mcp: transport type invalid")
	ErrMCPTransportCommandEmpty  = errors.New("mcp: stdio command must not be empty")
	ErrMCPTransportBaseURLEmpty  = errors.New("mcp: streamable_http base_url must not be empty")
	ErrMCPTransportBaseURLScheme = errors.New("mcp: streamable_http base_url must use http or https scheme")
	ErrMCPTransportTimeoutZero   = errors.New("mcp: timeout must be positive")
)

// MCPTransportConfig holds the validated configuration needed to establish
// an MCP connection over one of the supported transports.
type MCPTransportConfig struct {
	TransportType MCPTransportType
	// --- stdio fields ---
	Command string   // executable path
	Args    []string // command-line arguments
	Env     map[string]string
	// --- streamable_http fields ---
	BaseURL            string
	DefaultNegotiation MCPContentNegotiation // which Accept header to prefer
	// --- common ---
	ConnectTimeout  time.Duration
	RequestTimeout  time.Duration
	AutoStart       bool
}

// Validate returns an error when the config is inconsistent or incomplete.
func (c *MCPTransportConfig) Validate() error {
	if !c.TransportType.IsValid() {
		return ErrMCPTransportTypeInvalid
	}
	if c.ConnectTimeout <= 0 {
		return ErrMCPTransportTimeoutZero
	}
	switch c.TransportType {
	case MCPTransportStdio:
		if strings.TrimSpace(c.Command) == "" {
			return ErrMCPTransportCommandEmpty
		}
	case MCPTransportStreamableHTTP:
		if strings.TrimSpace(c.BaseURL) == "" {
			return ErrMCPTransportBaseURLEmpty
		}
		lower := strings.ToLower(c.BaseURL)
		if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
			return ErrMCPTransportBaseURLScheme
		}
	}
	return nil
}

// DefaultStdioConfig returns a sensible MCPTransportConfig for stdio transports.
func DefaultStdioConfig(command string, args ...string) *MCPTransportConfig {
	return &MCPTransportConfig{
		TransportType:  MCPTransportStdio,
		Command:        command,
		Args:           args,
		ConnectTimeout: 30 * time.Second,
		RequestTimeout: 60 * time.Second,
		AutoStart:      true,
	}
}

// DefaultStreamableHTTPConfig returns a sensible MCPTransportConfig for HTTP transports.
func DefaultStreamableHTTPConfig(baseURL string) *MCPTransportConfig {
	return &MCPTransportConfig{
		TransportType:      MCPTransportStreamableHTTP,
		BaseURL:            baseURL,
		DefaultNegotiation: MCPContentSSE,
		ConnectTimeout:     10 * time.Second,
		RequestTimeout:     120 * time.Second,
		AutoStart:          false,
	}
}
