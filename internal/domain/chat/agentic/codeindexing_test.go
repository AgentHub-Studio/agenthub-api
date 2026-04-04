package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- DetectCodeIndexingFromCommand ---

func TestDetectCodeIndexingFromCommand_Sourcegraph(t *testing.T) {
	assert.Equal(t, "sourcegraph", agentic.DetectCodeIndexingFromCommand(`src search "pattern"`))
}

func TestDetectCodeIndexingFromCommand_Cody(t *testing.T) {
	assert.Equal(t, "cody", agentic.DetectCodeIndexingFromCommand(`cody chat --message "help"`))
}

func TestDetectCodeIndexingFromCommand_NotIndexing(t *testing.T) {
	assert.Equal(t, "", agentic.DetectCodeIndexingFromCommand("ls -la"))
}

func TestDetectCodeIndexingFromCommand_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.DetectCodeIndexingFromCommand(""))
}

func TestDetectCodeIndexingFromCommand_NpxPrefix(t *testing.T) {
	assert.Equal(t, "sourcegraph", agentic.DetectCodeIndexingFromCommand("npx src search test"))
}

func TestDetectCodeIndexingFromCommand_BunxPrefix(t *testing.T) {
	assert.Equal(t, "cody", agentic.DetectCodeIndexingFromCommand("bunx cody chat"))
}

func TestDetectCodeIndexingFromCommand_NpxUnknown(t *testing.T) {
	assert.Equal(t, "", agentic.DetectCodeIndexingFromCommand("npx eslint ."))
}

func TestDetectCodeIndexingFromCommand_AmazonQ(t *testing.T) {
	assert.Equal(t, "amazon-q", agentic.DetectCodeIndexingFromCommand("q explain"))
}

func TestDetectCodeIndexingFromCommand_Aider(t *testing.T) {
	assert.Equal(t, "aider", agentic.DetectCodeIndexingFromCommand("aider --model gpt-4"))
}

func TestDetectCodeIndexingFromCommand_CaseInsensitive(t *testing.T) {
	assert.Equal(t, "sourcegraph", agentic.DetectCodeIndexingFromCommand("SRC search"))
}

// --- DetectCodeIndexingFromMCPTool ---

func TestDetectCodeIndexingFromMCPTool_Sourcegraph(t *testing.T) {
	assert.Equal(t, "sourcegraph", agentic.DetectCodeIndexingFromMCPTool("mcp__sourcegraph__search"))
}

func TestDetectCodeIndexingFromMCPTool_Cody(t *testing.T) {
	assert.Equal(t, "cody", agentic.DetectCodeIndexingFromMCPTool("mcp__cody__chat"))
}

func TestDetectCodeIndexingFromMCPTool_NotIndexing(t *testing.T) {
	assert.Equal(t, "", agentic.DetectCodeIndexingFromMCPTool("mcp__filesystem__read"))
}

func TestDetectCodeIndexingFromMCPTool_NotMCP(t *testing.T) {
	assert.Equal(t, "", agentic.DetectCodeIndexingFromMCPTool("regular_tool"))
}

func TestDetectCodeIndexingFromMCPTool_ShortParts(t *testing.T) {
	assert.Equal(t, "", agentic.DetectCodeIndexingFromMCPTool("mcp__"))
}

// --- DetectCodeIndexingFromMCPServerName ---

func TestDetectCodeIndexingFromMCPServerName_Sourcegraph(t *testing.T) {
	assert.Equal(t, "sourcegraph", agentic.DetectCodeIndexingFromMCPServerName("sourcegraph"))
}

func TestDetectCodeIndexingFromMCPServerName_CaseInsensitive(t *testing.T) {
	assert.Equal(t, "sourcegraph", agentic.DetectCodeIndexingFromMCPServerName("Sourcegraph"))
}

func TestDetectCodeIndexingFromMCPServerName_GithubCopilot(t *testing.T) {
	assert.Equal(t, "github-copilot", agentic.DetectCodeIndexingFromMCPServerName("github-copilot"))
}

func TestDetectCodeIndexingFromMCPServerName_GithubCopilotUnderscore(t *testing.T) {
	assert.Equal(t, "github-copilot", agentic.DetectCodeIndexingFromMCPServerName("github_copilot"))
}

func TestDetectCodeIndexingFromMCPServerName_AugmentCode(t *testing.T) {
	assert.Equal(t, "augment", agentic.DetectCodeIndexingFromMCPServerName("augment-code"))
}

func TestDetectCodeIndexingFromMCPServerName_ClaudeContext(t *testing.T) {
	assert.Equal(t, "claude-context", agentic.DetectCodeIndexingFromMCPServerName("claude-context"))
}

func TestDetectCodeIndexingFromMCPServerName_CodeIndex(t *testing.T) {
	assert.Equal(t, "code-index-mcp", agentic.DetectCodeIndexingFromMCPServerName("code-index"))
}

func TestDetectCodeIndexingFromMCPServerName_Unknown(t *testing.T) {
	assert.Equal(t, "", agentic.DetectCodeIndexingFromMCPServerName("filesystem"))
}

func TestDetectCodeIndexingFromMCPServerName_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.DetectCodeIndexingFromMCPServerName(""))
}
