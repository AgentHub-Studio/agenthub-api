package agentic_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

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

func TestMCPToolBridge_WithAllowedServerNames_EmptyAllowsAll(t *testing.T) {
	client := &mockMCPClient{
		tools: []agentic.MCPToolInfo{
			{ServerName: "github", Name: "list_repos"},
			{ServerName: "slack", Name: "send_message"},
		},
	}

	bridge := agentic.NewMCPToolBridge(client, "t")
	bridge.WithAllowedServerNames(nil)

	tools, err := bridge.ListTools(context.Background())
	require.NoError(t, err)
	assert.Len(t, tools, 2)
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
