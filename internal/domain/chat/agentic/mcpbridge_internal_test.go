package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/mcpruntime"
)

func TestNewHTTPMCPClient_EmptyURLUsesRuntimeServiceContract(t *testing.T) {
	client := NewHTTPMCPClient("")

	assert.Equal(t, mcpruntime.DefaultHTTPURL, client.baseURL)
}
