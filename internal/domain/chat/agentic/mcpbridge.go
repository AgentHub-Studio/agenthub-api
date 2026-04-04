package agentic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	mcpClient MCPClientService
	tenantID  string
}

// NewMCPToolBridge creates a bridge for the given tenant.
func NewMCPToolBridge(mcpClient MCPClientService, tenantID string) *MCPToolBridge {
	return &MCPToolBridge{
		mcpClient: mcpClient,
		tenantID:  tenantID,
	}
}

// ListTools fetches all MCP tools and converts them into LLMTool format.
// Tool names follow the Claude Code convention: mcp__{serverName}__{toolName}.
func (b *MCPToolBridge) ListTools(ctx context.Context) ([]LLMTool, error) {
	infos, err := b.mcpClient.ListTools(ctx, b.tenantID)
	if err != nil {
		return nil, fmt.Errorf("mcpbridge: list tools: %w", err)
	}

	tools := make([]LLMTool, 0, len(infos))
	for _, info := range infos {
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
	return tools, nil
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
type listToolsResponse struct {
	Tools []MCPToolInfo `json:"tools"`
}

// ListTools fetches available tools from all active MCP servers.
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
	return result.Tools, nil
}

// callToolRequest is the body sent to invoke an MCP tool.
type callToolRequest struct {
	ServerName string          `json:"serverName"`
	ToolName   string          `json:"toolName"`
	Input      json.RawMessage `json:"input"`
}

// callToolResponse is the response from the MCP client runtime.
type callToolResponse struct {
	Output json.RawMessage `json:"output"`
	Error  *string         `json:"error,omitempty"`
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

	return result.Output, nil
}
