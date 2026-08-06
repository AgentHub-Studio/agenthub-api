package tool_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
)

func TestToolService_TestHTTPTool_ClassifiesUpstreamFailure(t *testing.T) {
	const upstreamSecret = "tool-upstream-contract-secret"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "upstream failed api_key="+upstreamSecret, http.StatusServiceUnavailable)
	}))
	defer server.Close()

	config, err := json.Marshal(map[string]any{
		"url":    server.URL,
		"method": http.MethodGet,
	})
	require.NoError(t, err)

	repo := newMockRepo()
	id := uuid.New()
	repo.data[id] = tool.Tool{
		ID:     id,
		Name:   "HTTP upstream failure",
		Type:   tool.ToolTypeHTTP,
		Config: config,
	}
	service := tool.NewService(repo).WithHTTPURLValidator(func(string) error { return nil })

	_, err = service.TestTool(context.Background(), id, nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrUpstream)
	assert.NotContains(t, err.Error(), upstreamSecret)
}
