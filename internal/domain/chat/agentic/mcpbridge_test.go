package agentic_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- mock MCPClientService ---

type mockMCPClient struct {
	tools    []agentic.MCPToolInfo
	listErr  error
	callResp json.RawMessage
	callErr  error

	// Captures for assertions.
	lastCallServer string
	lastCallTool   string
	lastCallInput  json.RawMessage
}

func (m *mockMCPClient) ListTools(_ context.Context, _ string) ([]agentic.MCPToolInfo, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.tools, nil
}

func (m *mockMCPClient) CallTool(_ context.Context, _, serverName, toolName string, input json.RawMessage) (json.RawMessage, error) {
	m.lastCallServer = serverName
	m.lastCallTool = toolName
	m.lastCallInput = input
	if m.callErr != nil {
		return nil, m.callErr
	}
	return m.callResp, nil
}

// --- Name format tests ---

func TestFormatMCPToolName(t *testing.T) {
	name := agentic.FormatMCPToolName("github", "list_repos")
	assert.Equal(t, "mcp__github__list_repos", name)
}

func TestParseMCPToolName_Valid(t *testing.T) {
	server, tool, ok := agentic.ParseMCPToolName("mcp__github__list_repos")
	require.True(t, ok)
	assert.Equal(t, "github", server)
	assert.Equal(t, "list_repos", tool)
}

func TestParseMCPToolName_ValidWithDashes(t *testing.T) {
	server, tool, ok := agentic.ParseMCPToolName("mcp__my-server__create_issue")
	require.True(t, ok)
	assert.Equal(t, "my-server", server)
	assert.Equal(t, "create_issue", tool)
}

func TestParseMCPToolName_NoPrefix(t *testing.T) {
	_, _, ok := agentic.ParseMCPToolName("document_search")
	assert.False(t, ok)
}

func TestParseMCPToolName_MissingToolPart(t *testing.T) {
	_, _, ok := agentic.ParseMCPToolName("mcp__github")
	assert.False(t, ok)
}

func TestParseMCPToolName_EmptyServer(t *testing.T) {
	_, _, ok := agentic.ParseMCPToolName("mcp____tool")
	assert.False(t, ok)
}

func TestParseMCPToolName_EmptyTool(t *testing.T) {
	_, _, ok := agentic.ParseMCPToolName("mcp__server__")
	assert.False(t, ok)
}

func TestIsMCPToolCall(t *testing.T) {
	assert.True(t, agentic.IsMCPToolCall("mcp__github__list_repos"))
	assert.False(t, agentic.IsMCPToolCall("document_search"))
	assert.False(t, agentic.IsMCPToolCall(""))
}

func TestFormatAndParse_Roundtrip(t *testing.T) {
	name := agentic.FormatMCPToolName("slack", "send_message")
	server, tool, ok := agentic.ParseMCPToolName(name)
	require.True(t, ok)
	assert.Equal(t, "slack", server)
	assert.Equal(t, "send_message", tool)
}

// --- MCPToolBridge.ListTools tests ---

func TestMCPToolBridge_ListTools_Success(t *testing.T) {
	client := &mockMCPClient{
		tools: []agentic.MCPToolInfo{
			{
				ServerName:  "github",
				Name:        "list_repos",
				Description: "List repositories",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"org":{"type":"string"}}}`),
			},
			{
				ServerName:  "github",
				Name:        "create_issue",
				Description: "Create an issue",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"title":{"type":"string"}}}`),
			},
		},
	}

	bridge := agentic.NewMCPToolBridge(client, "my-tenant")
	tools, err := bridge.ListTools(context.Background())
	require.NoError(t, err)
	assert.Len(t, tools, 2)
	assert.Equal(t, "mcp__github__list_repos", tools[0].Name)
	assert.Equal(t, "List repositories", tools[0].Description)
	assert.Equal(t, "mcp__github__create_issue", tools[1].Name)
}

func TestMCPToolBridge_ListTools_EmptyDescription(t *testing.T) {
	client := &mockMCPClient{
		tools: []agentic.MCPToolInfo{
			{ServerName: "fs", Name: "read_file", Description: ""},
		},
	}

	bridge := agentic.NewMCPToolBridge(client, "t")
	tools, err := bridge.ListTools(context.Background())
	require.NoError(t, err)
	assert.Contains(t, tools[0].Description, "MCP tool read_file from server fs")
}

func TestMCPToolBridge_ListTools_EmptySchema(t *testing.T) {
	client := &mockMCPClient{
		tools: []agentic.MCPToolInfo{
			{ServerName: "fs", Name: "pwd", Description: "Get cwd"},
		},
	}

	bridge := agentic.NewMCPToolBridge(client, "t")
	tools, err := bridge.ListTools(context.Background())
	require.NoError(t, err)
	assert.JSONEq(t, `{"type":"object","properties":{}}`, string(tools[0].InputSchema))
}

func TestMCPToolBridge_ListTools_Error(t *testing.T) {
	client := &mockMCPClient{listErr: fmt.Errorf("connection refused")}
	bridge := agentic.NewMCPToolBridge(client, "t")
	_, err := bridge.ListTools(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestMCPToolBridge_ListTools_Empty(t *testing.T) {
	client := &mockMCPClient{tools: nil}
	bridge := agentic.NewMCPToolBridge(client, "t")
	tools, err := bridge.ListTools(context.Background())
	require.NoError(t, err)
	assert.Empty(t, tools)
}

// --- MCPToolBridge.Execute tests ---

func TestMCPToolBridge_Execute_Success(t *testing.T) {
	client := &mockMCPClient{
		callResp: json.RawMessage(`{"repos":["repo1","repo2"]}`),
	}

	bridge := agentic.NewMCPToolBridge(client, "my-tenant")
	input := json.RawMessage(`{"org":"acme"}`)
	result, err := bridge.Execute(context.Background(), "mcp__github__list_repos", input)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Nil(t, result.Error)
	assert.JSONEq(t, `{"repos":["repo1","repo2"]}`, string(result.Output))
	assert.Equal(t, "github", client.lastCallServer)
	assert.Equal(t, "list_repos", client.lastCallTool)
}

func TestMCPToolBridge_Execute_Error(t *testing.T) {
	client := &mockMCPClient{
		callErr: fmt.Errorf("server disconnected"),
	}

	bridge := agentic.NewMCPToolBridge(client, "t")
	result, err := bridge.Execute(context.Background(), "mcp__slack__send_msg", json.RawMessage(`{}`))
	require.NoError(t, err) // bridge wraps tool errors in result, not in err
	require.NotNil(t, result)
	require.NotNil(t, result.Error)
	assert.Contains(t, *result.Error, "server disconnected")
}

func TestMCPToolBridge_Execute_InvalidToolName(t *testing.T) {
	client := &mockMCPClient{}
	bridge := agentic.NewMCPToolBridge(client, "t")
	_, err := bridge.Execute(context.Background(), "document_search", json.RawMessage(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid MCP tool name")
}

func TestMCPToolBridge_Execute_LatencyTracked(t *testing.T) {
	client := &mockMCPClient{
		callResp: json.RawMessage(`"ok"`),
	}

	bridge := agentic.NewMCPToolBridge(client, "t")
	result, err := bridge.Execute(context.Background(), "mcp__fs__pwd", json.RawMessage(`{}`))
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.LatencyMs, int64(0))
}

// --- TR-01-TASK-34: MCPToolBridge server-level filtering (P-C253-1) ---

func TestMCPToolBridge_WithAllowedServerNames_FiltersUnbound(t *testing.T) {
	client := &mockMCPClient{
		tools: []agentic.MCPToolInfo{
			{ServerName: "github", Name: "list_repos"},
			{ServerName: "slack", Name: "send_message"},
			{ServerName: "fs", Name: "read_file"},
		},
	}

	bridge := agentic.NewMCPToolBridge(client, "t")
	bridge.WithAllowedServerNames([]string{"github", "fs"})

	tools, err := bridge.ListTools(context.Background())
	require.NoError(t, err)
	assert.Len(t, tools, 2)
	assert.Equal(t, "mcp__github__list_repos", tools[0].Name)
	assert.Equal(t, "mcp__fs__read_file", tools[1].Name)
}

func TestMCPToolBridge_WithAllowedServerNames_NilAllowsAll(t *testing.T) {
	client := &mockMCPClient{
		tools: []agentic.MCPToolInfo{
			{ServerName: "github", Name: "list_repos"},
			{ServerName: "slack", Name: "send_message"},
		},
	}

	bridge := agentic.NewMCPToolBridge(client, "t")
	bridge.WithAllowedServerNames(nil) // nil = no filter (agent has no bindings)

	tools, err := bridge.ListTools(context.Background())
	require.NoError(t, err)
	assert.Len(t, tools, 2)
}

// P-C253-1: when the agent has explicit MCP bindings but all are disabled,
// the snapshot is an empty (non-nil) slice → the bridge should expose zero tools.
func TestMCPToolBridge_WithAllowedServerNames_EmptySliceBlocksAll(t *testing.T) {
	client := &mockMCPClient{
		tools: []agentic.MCPToolInfo{
			{ServerName: "github", Name: "list_repos"},
			{ServerName: "slack", Name: "send_message"},
		},
	}

	bridge := agentic.NewMCPToolBridge(client, "t")
	bridge.WithAllowedServerNames([]string{}) // empty non-nil = has bindings but all disabled

	tools, err := bridge.ListTools(context.Background())
	require.NoError(t, err)
	assert.Empty(t, tools, "empty non-nil allowed list should block all MCP tools")
}

func TestMCPToolBridge_WithAllowedServerNames_AllFiltered(t *testing.T) {
	client := &mockMCPClient{
		tools: []agentic.MCPToolInfo{
			{ServerName: "github", Name: "list_repos"},
		},
	}

	bridge := agentic.NewMCPToolBridge(client, "t")
	bridge.WithAllowedServerNames([]string{"slack"})

	tools, err := bridge.ListTools(context.Background())
	require.NoError(t, err)
	assert.Empty(t, tools)
}

// --- TR-01-TASK-35: CachedMCPClient 60s TTL (P-C274-1) ---

type countingMCPClient struct {
	mockMCPClient
	listCalls int
}

func (m *countingMCPClient) ListTools(ctx context.Context, tenantID string) ([]agentic.MCPToolInfo, error) {
	m.listCalls++
	return m.mockMCPClient.ListTools(ctx, tenantID)
}

func TestCachedMCPClient_CachesListTools(t *testing.T) {
	inner := &countingMCPClient{
		mockMCPClient: mockMCPClient{
			tools: []agentic.MCPToolInfo{{ServerName: "github", Name: "list_repos"}},
		},
	}
	cached := agentic.NewCachedMCPClient(inner, 5*time.Second)

	_, err := cached.ListTools(context.Background(), "tenant-a")
	require.NoError(t, err)
	_, err = cached.ListTools(context.Background(), "tenant-a")
	require.NoError(t, err)

	assert.Equal(t, 1, inner.listCalls, "second call should hit cache")
}

func TestCachedMCPClient_SeparateCachePerTenant(t *testing.T) {
	inner := &countingMCPClient{
		mockMCPClient: mockMCPClient{
			tools: []agentic.MCPToolInfo{{ServerName: "fs", Name: "read"}},
		},
	}
	cached := agentic.NewCachedMCPClient(inner, 5*time.Second)

	_, _ = cached.ListTools(context.Background(), "tenant-a")
	_, _ = cached.ListTools(context.Background(), "tenant-b")

	assert.Equal(t, 2, inner.listCalls, "each tenant should be cached independently")
}

func TestCachedMCPClient_Invalidate_ForcesRefresh(t *testing.T) {
	inner := &countingMCPClient{
		mockMCPClient: mockMCPClient{
			tools: []agentic.MCPToolInfo{{ServerName: "fs", Name: "read"}},
		},
	}
	cached := agentic.NewCachedMCPClient(inner, 5*time.Second)

	_, _ = cached.ListTools(context.Background(), "tenant-a")
	cached.Invalidate("tenant-a")
	_, _ = cached.ListTools(context.Background(), "tenant-a")

	assert.Equal(t, 2, inner.listCalls, "invalidate should force fresh fetch")
}

func TestCachedMCPClient_CallToolNotCached(t *testing.T) {
	inner := &countingMCPClient{
		mockMCPClient: mockMCPClient{
			callResp: []byte(`"ok"`),
		},
	}
	cached := agentic.NewCachedMCPClient(inner, 5*time.Second)

	// CallTool should forward directly to the inner client (no caching).
	_, _ = cached.CallTool(context.Background(), "tenant", "server", "tool", nil)
	_, _ = cached.CallTool(context.Background(), "tenant", "server", "tool2", nil)

	// Both calls reached the inner client (last call was tool2).
	assert.Equal(t, "tool2", inner.lastCallTool, "CallTool should forward directly")
}

// --- TR-01-TASK-36: CircuitBreakerMCPClient auto-disable (P-C275-1) ---

func TestCircuitBreaker_DisablesAfterMaxFailures(t *testing.T) {
	inner := &mockMCPClient{
		tools:   []agentic.MCPToolInfo{{ServerName: "flaky", Name: "do_thing"}},
		callErr: fmt.Errorf("connection refused"),
	}
	cb := agentic.NewCircuitBreakerMCPClient(inner, 3)

	for i := 0; i < 3; i++ {
		_, _ = cb.CallTool(context.Background(), "tenant", "flaky", "do_thing", nil)
	}

	assert.True(t, cb.IsDisabled("flaky"), "server should be disabled after 3 failures")
}

func TestCircuitBreaker_NotDisabledBefore3Failures(t *testing.T) {
	inner := &mockMCPClient{callErr: fmt.Errorf("error")}
	cb := agentic.NewCircuitBreakerMCPClient(inner, 3)

	_, _ = cb.CallTool(context.Background(), "tenant", "flaky", "tool", nil)
	_, _ = cb.CallTool(context.Background(), "tenant", "flaky", "tool", nil)

	assert.False(t, cb.IsDisabled("flaky"), "2 failures should not trip the breaker")
}

func TestCircuitBreaker_SuccessResetsCounter(t *testing.T) {
	callCount := 0
	inner := &mockMCPClient{}
	cb := agentic.NewCircuitBreakerMCPClient(inner, 3)

	// 2 failures, then success, then 2 more failures — should not trip.
	inner.callErr = fmt.Errorf("err")
	_, _ = cb.CallTool(context.Background(), "tenant", "server", "tool", nil)
	_, _ = cb.CallTool(context.Background(), "tenant", "server", "tool", nil)
	inner.callErr = nil
	inner.callResp = []byte(`"ok"`)
	_, _ = cb.CallTool(context.Background(), "tenant", "server", "tool", nil) // reset
	_ = callCount
	inner.callErr = fmt.Errorf("err")
	_, _ = cb.CallTool(context.Background(), "tenant", "server", "tool", nil)
	_, _ = cb.CallTool(context.Background(), "tenant", "server", "tool", nil)

	assert.False(t, cb.IsDisabled("server"), "counter should have been reset by the successful call")
}

func TestCircuitBreaker_DisabledServerFilteredFromListTools(t *testing.T) {
	inner := &mockMCPClient{
		tools: []agentic.MCPToolInfo{
			{ServerName: "good", Name: "tool_a"},
			{ServerName: "flaky", Name: "tool_b"},
		},
		callErr: fmt.Errorf("err"),
	}
	cb := agentic.NewCircuitBreakerMCPClient(inner, 3)

	for i := 0; i < 3; i++ {
		_, _ = cb.CallTool(context.Background(), "tenant", "flaky", "tool_b", nil)
	}

	tools, err := cb.ListTools(context.Background(), "tenant")
	require.NoError(t, err)
	assert.Len(t, tools, 1, "disabled server's tools should be excluded")
	assert.Equal(t, "good", tools[0].ServerName)
}

func TestCircuitBreaker_Reset_ReEnablesServer(t *testing.T) {
	inner := &mockMCPClient{callErr: fmt.Errorf("err")}
	cb := agentic.NewCircuitBreakerMCPClient(inner, 3)

	for i := 0; i < 3; i++ {
		_, _ = cb.CallTool(context.Background(), "tenant", "flaky", "tool", nil)
	}
	require.True(t, cb.IsDisabled("flaky"))

	cb.Reset("flaky")
	assert.False(t, cb.IsDisabled("flaky"), "Reset should re-enable the server")
}
