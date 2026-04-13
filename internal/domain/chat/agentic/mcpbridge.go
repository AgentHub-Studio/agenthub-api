package agentic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const mcpToolPrefix = "mcp__"

// MCPToolInfo describes a single tool exposed by an MCP server.
type MCPToolInfo struct {
	ServerName  string          `json:"serverName"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// MCPClientService is the interface for communicating with the MCP client runtime.
type MCPClientService interface {
	// ListTools returns all tools available across all active MCP servers for the tenant.
	ListTools(ctx context.Context, tenantID string) ([]MCPToolInfo, error)
	// CallTool invokes a specific tool on a specific MCP server.
	CallTool(ctx context.Context, tenantID, serverName, toolName string, input json.RawMessage) (json.RawMessage, error)
}

// MCPToolBridge adapts MCP tools into the agentic loop as LLMTool definitions
// and routes tool_call executions to the MCP client runtime.
type MCPToolBridge struct {
	mcpClient      MCPClientService
	tenantID       string
	allowedServers map[string]bool // nil = no filter (all servers); non-nil = filter applies; P-C253-1
	filterEnabled  bool            // true when WithAllowedServerNames was called with a non-nil slice
}

// NewMCPToolBridge creates a bridge for the given tenant.
func NewMCPToolBridge(mcpClient MCPClientService, tenantID string) *MCPToolBridge {
	return &MCPToolBridge{
		mcpClient: mcpClient,
		tenantID:  tenantID,
	}
}

// WithAllowedServerNames restricts MCP tools to the named servers only.
// P-C253-1: agent-level MCP binding — called when the agent has explicit MCP server bindings.
//
// Nil resets the filter (no restriction — all MCP tools exposed).
// An empty non-nil slice blocks all MCP tools (agent has bindings but all are disabled).
// A non-empty slice exposes only tools from the named servers.
func (b *MCPToolBridge) WithAllowedServerNames(names []string) {
	if names == nil {
		b.allowedServers = nil
		b.filterEnabled = false
		return
	}
	b.filterEnabled = true
	b.allowedServers = make(map[string]bool, len(names))
	for _, n := range names {
		b.allowedServers[n] = true
	}
}

// ListTools fetches all MCP tools and converts them into LLMTool format.
// Tool names follow the Claude Code convention: mcp__{serverName}__{toolName}.
// When allowedServers is set, only tools from those servers are returned.
//
// BUG-MCP-PARTIAL-DROP fix: when mcpClient.ListTools returns (partial-tools, error),
// the partial tools come from servers that DID succeed. We must not discard them —
// return the partial set along with the error so the caller can include what worked
// while still surfacing the warning about servers that failed.
func (b *MCPToolBridge) ListTools(ctx context.Context) ([]LLMTool, error) {
	infos, err := b.mcpClient.ListTools(ctx, b.tenantID)
	if err != nil && len(infos) == 0 {
		// All servers failed — nothing to return.
		return nil, fmt.Errorf("mcpbridge: list tools: %w", err)
	}
	var listErr error
	if err != nil {
		// Partial failure: some servers failed. Wrap the error so the caller can log it,
		// but continue to convert the tools we did get.
		listErr = fmt.Errorf("mcpbridge: list tools: %w", err)
	}

	tools := make([]LLMTool, 0, len(infos))
	for _, info := range infos {
		// P-C253-1: skip tools from servers not in the agent's binding list.
		// filterEnabled is true when the agent has explicit MCP bindings (even if all are disabled).
		if b.filterEnabled && !b.allowedServers[info.ServerName] {
			continue
		}
		name := FormatMCPToolName(info.ServerName, info.Name)
		desc := info.Description
		if desc == "" {
			desc = fmt.Sprintf("MCP tool %s from server %s", info.Name, info.ServerName)
		}

		schema := info.InputSchema
		if len(schema) == 0 {
			schema = json.RawMessage(`{"type":"object","properties":{}}`)
		}

		tools = append(tools, LLMTool{
			Name:        name,
			Description: desc,
			InputSchema: schema,
		})
	}
	return tools, listErr
}

// Execute routes an MCP tool call to the correct server and tool.
func (b *MCPToolBridge) Execute(ctx context.Context, toolName string, input json.RawMessage) (*ToolExecResult, error) {
	serverName, mcpToolName, ok := ParseMCPToolName(toolName)
	if !ok {
		return nil, fmt.Errorf("mcpbridge: invalid MCP tool name %q", toolName)
	}

	start := time.Now()
	output, err := b.mcpClient.CallTool(ctx, b.tenantID, serverName, mcpToolName, input)
	elapsed := time.Since(start).Milliseconds()

	if err != nil {
		errMsg := err.Error()
		return &ToolExecResult{
			Error:     &errMsg,
			LatencyMs: elapsed,
		}, nil
	}

	return &ToolExecResult{
		Output:    output,
		LatencyMs: elapsed,
	}, nil
}

// IsMCPToolCall returns true if the tool name follows the mcp__ convention.
func IsMCPToolCall(toolName string) bool {
	return strings.HasPrefix(toolName, mcpToolPrefix)
}

// FormatMCPToolName builds the canonical tool name: mcp__{serverName}__{toolName}.
func FormatMCPToolName(serverName, toolName string) string {
	return mcpToolPrefix + serverName + "__" + toolName
}

// ParseMCPToolName extracts the server and tool names from an mcp__ prefixed name.
// Returns ("", "", false) if the name is not a valid MCP tool name.
func ParseMCPToolName(name string) (serverName, toolName string, ok bool) {
	if !strings.HasPrefix(name, mcpToolPrefix) {
		return "", "", false
	}
	rest := name[len(mcpToolPrefix):]
	idx := strings.Index(rest, "__")
	if idx <= 0 || idx >= len(rest)-2 {
		return "", "", false
	}
	return rest[:idx], rest[idx+2:], true
}

// --- HTTP-based MCPClientService implementation ---

// HTTPMCPClient implements MCPClientService by calling the agenthub-mcp-client-runtime via HTTP.
type HTTPMCPClient struct {
	baseURL string
	client  *http.Client
}

// NewHTTPMCPClient creates an HTTP-based MCP client.
// If baseURL is empty it defaults to http://agenthub-mcp-client-runtime:8083.
func NewHTTPMCPClient(baseURL string) *HTTPMCPClient {
	if baseURL == "" {
		baseURL = "http://agenthub-mcp-client-runtime:8083"
	}
	return &HTTPMCPClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// listToolsResponse is the response shape from the MCP client runtime.
// Warnings contains per-server failures (BUG-MCP2 fix): when a server can't be
// started or listed, the runtime now includes it here instead of silently skipping.
type listToolsResponse struct {
	Tools    []MCPToolInfo `json:"tools"`
	Warnings []string      `json:"warnings,omitempty"`
}

// ListTools fetches available tools from all active MCP servers.
// Returns (tools, nil) when all servers succeeded.
// Returns (partial-tools, error) when some servers failed — the error
// aggregates all per-server warning messages so the caller can surface them.
func (c *HTTPMCPClient) ListTools(ctx context.Context, tenantID string) ([]MCPToolInfo, error) {
	url := fmt.Sprintf("%s/api/tools?tenantId=%s", c.baseURL, tenantID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: new request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: list tools: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mcpclient: list tools returned %d: %s", resp.StatusCode, string(body))
	}

	var result listToolsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("mcpclient: unmarshal tools: %w", err)
	}

	// BUG-MCP2: propagate per-server warnings as a combined error so the caller
	// (ToolSchemaBuilder) can emit them as run-level warning events.
	// We still return the tools we did get (partial success).
	if len(result.Warnings) > 0 {
		return result.Tools, fmt.Errorf("%s", strings.Join(result.Warnings, "; "))
	}

	return result.Tools, nil
}

// callToolRequest is the body sent to invoke an MCP tool.
type callToolRequest struct {
	ServerName string          `json:"serverName"`
	ToolName   string          `json:"toolName"`
	Input      json.RawMessage `json:"input"`
}

// callToolResponse is the response from the MCP client runtime.
// The runtime mirrors the MCP spec: when the tool itself signals a failure,
// it sets isError=true and puts a description in output rather than returning
// a non-200 HTTP status or a top-level error string.
type callToolResponse struct {
	Output  json.RawMessage `json:"output"`
	Error   *string         `json:"error,omitempty"`
	IsError bool            `json:"isError"`
}

// CallTool invokes a tool on a specific MCP server.
func (c *HTTPMCPClient) CallTool(ctx context.Context, tenantID, serverName, toolName string, input json.RawMessage) (json.RawMessage, error) {
	reqBody := callToolRequest{
		ServerName: serverName,
		ToolName:   toolName,
		Input:      input,
	}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/tools/call?tenantId=%s", c.baseURL, tenantID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("mcpclient: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: call tool: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("mcpclient: read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("mcpclient: call tool returned %d: %s", resp.StatusCode, string(body))
	}

	var result callToolResponse
	if err := json.Unmarshal(body, &result); err != nil {
		// If we can't parse, return raw body.
		return body, nil
	}

	if result.Error != nil {
		return nil, fmt.Errorf("mcpclient: tool error: %s", *result.Error)
	}

	// BUG-MCP1: the MCP spec uses isError=true (not a non-2xx HTTP status) to
	// signal that the tool call itself failed. Without this check, the error is
	// silently delivered as success output and hadToolFailures stays false.
	if result.IsError {
		errMsg := string(result.Output)
		if errMsg == "" || errMsg == "null" {
			errMsg = "tool returned an error (no details provided)"
		}
		return nil, fmt.Errorf("mcpclient: mcp tool failure: %s", errMsg)
	}

	return result.Output, nil
}

// --- CachedMCPClient ---

// mcpToolCacheEntry holds a cached ListTools result with an expiry timestamp.
type mcpToolCacheEntry struct {
	tools     []MCPToolInfo
	expiresAt time.Time
}

// CachedMCPClient wraps an MCPClientService and caches ListTools results per tenant.
// CallTool is always forwarded without caching (side effects must not be cached).
// P-C274-1: 60s TTL prevents hammering the mcp-client-runtime on every LLM turn.
type CachedMCPClient struct {
	inner MCPClientService
	ttl   time.Duration
	mu    sync.Mutex
	cache map[string]mcpToolCacheEntry // key: tenantID
}

// NewCachedMCPClient wraps inner with a tool-listing cache of the given TTL.
// If ttl is zero, 60 seconds is used.
func NewCachedMCPClient(inner MCPClientService, ttl time.Duration) *CachedMCPClient {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &CachedMCPClient{
		inner: inner,
		ttl:   ttl,
		cache: make(map[string]mcpToolCacheEntry),
	}
}

// ListTools returns cached results if still fresh, otherwise fetches from the wrapped client.
func (c *CachedMCPClient) ListTools(ctx context.Context, tenantID string) ([]MCPToolInfo, error) {
	c.mu.Lock()
	entry, ok := c.cache[tenantID]
	c.mu.Unlock()

	if ok && time.Now().Before(entry.expiresAt) {
		return entry.tools, nil
	}

	tools, err := c.inner.ListTools(ctx, tenantID)
	if err != nil {
		// BUG-MCP-PARTIAL-DROP: on partial failure (some servers returned tools, some failed),
		// pass partial tools through to the caller without caching — the error state should
		// not be persisted so the next call retries fresh.
		return tools, err
	}

	c.mu.Lock()
	c.cache[tenantID] = mcpToolCacheEntry{
		tools:     tools,
		expiresAt: time.Now().Add(c.ttl),
	}
	c.mu.Unlock()

	return tools, nil
}

// Invalidate removes the cached entry for the given tenant, forcing a fresh fetch next time.
func (c *CachedMCPClient) Invalidate(tenantID string) {
	c.mu.Lock()
	delete(c.cache, tenantID)
	c.mu.Unlock()
}

// CallTool is forwarded directly to the underlying client (no caching).
func (c *CachedMCPClient) CallTool(ctx context.Context, tenantID, serverName, toolName string, input json.RawMessage) (json.RawMessage, error) {
	return c.inner.CallTool(ctx, tenantID, serverName, toolName, input)
}

// --- CircuitBreakerMCPClient ---

// CircuitBreakerMCPClient wraps an MCPClientService and tracks consecutive
// failures per MCP server. After maxFailures consecutive errors from a server,
// that server is disabled and its tools are excluded from subsequent ListTools
// responses. The failure count resets on a successful CallTool.
// P-C275-1: auto-disable of MCP servers after 3 consecutive failures.
type CircuitBreakerMCPClient struct {
	inner       MCPClientService
	maxFailures int
	mu          sync.Mutex
	failures    map[string]int  // key: serverName → consecutive failure count
	disabled    map[string]bool // key: serverName → whether server is tripped
}

// NewCircuitBreakerMCPClient creates a new circuit breaker wrapper.
// maxFailures is the number of consecutive tool-call errors before a server is disabled.
// If maxFailures <= 0, the default of 3 is used.
func NewCircuitBreakerMCPClient(inner MCPClientService, maxFailures int) *CircuitBreakerMCPClient {
	if maxFailures <= 0 {
		maxFailures = 3
	}
	return &CircuitBreakerMCPClient{
		inner:       inner,
		maxFailures: maxFailures,
		failures:    make(map[string]int),
		disabled:    make(map[string]bool),
	}
}

// ListTools returns only tools from servers that have not been tripped.
func (c *CircuitBreakerMCPClient) ListTools(ctx context.Context, tenantID string) ([]MCPToolInfo, error) {
	all, err := c.inner.ListTools(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]MCPToolInfo, 0, len(all))
	for _, tool := range all {
		if !c.disabled[tool.ServerName] {
			out = append(out, tool)
		}
	}
	return out, nil
}

// CallTool forwards the call and tracks consecutive failures per server.
// On success the failure counter for the server is reset.
// Once maxFailures is reached, the server is disabled.
func (c *CircuitBreakerMCPClient) CallTool(ctx context.Context, tenantID, serverName, toolName string, input json.RawMessage) (json.RawMessage, error) {
	result, err := c.inner.CallTool(ctx, tenantID, serverName, toolName, input)

	c.mu.Lock()
	defer c.mu.Unlock()

	if err != nil {
		c.failures[serverName]++
		if c.failures[serverName] >= c.maxFailures {
			c.disabled[serverName] = true
		}
	} else {
		// Reset on success.
		c.failures[serverName] = 0
	}

	return result, err
}

// IsDisabled reports whether the given server has been auto-disabled.
func (c *CircuitBreakerMCPClient) IsDisabled(serverName string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.disabled[serverName]
}

// Reset clears the circuit breaker state for all servers (for testing or admin reset).
func (c *CircuitBreakerMCPClient) Reset(serverName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.failures, serverName)
	delete(c.disabled, serverName)
}
