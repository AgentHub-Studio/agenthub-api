package agentic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
)

func TestHashConfig_EmptyReturnsEmpty(t *testing.T) {
	assert.Equal(t, "", hashConfig(nil))
	assert.Equal(t, "", hashConfig(json.RawMessage{}))
}

func TestHashConfig_SameInputSameHash(t *testing.T) {
	cfg := json.RawMessage(`{"provider":"anthropic","model":"claude-3-5-sonnet-20241022"}`)
	h1 := hashConfig(cfg)
	h2 := hashConfig(cfg)
	require.NotEmpty(t, h1)
	assert.Equal(t, h1, h2)
}

func TestHashConfig_DifferentInputDifferentHash(t *testing.T) {
	cfg1 := json.RawMessage(`{"provider":"anthropic","model":"claude-3-5-sonnet-20241022"}`)
	cfg2 := json.RawMessage(`{"provider":"openai","model":"gpt-4o"}`)
	assert.NotEqual(t, hashConfig(cfg1), hashConfig(cfg2))
}

func TestHashConfig_KeyOrderNormalised(t *testing.T) {
	// Same semantics, different key order — must produce the same hash.
	cfg1 := json.RawMessage(`{"model":"claude-3-5-sonnet-20241022","provider":"anthropic"}`)
	cfg2 := json.RawMessage(`{"provider":"anthropic","model":"claude-3-5-sonnet-20241022"}`)
	assert.Equal(t, hashConfig(cfg1), hashConfig(cfg2))
}

func TestDetectConfigChange_NoStoredHash_ReturnsFalse(t *testing.T) {
	session := chat.ChatSession{ConfigHash: nil}
	cfg := json.RawMessage(`{"provider":"anthropic"}`)
	assert.False(t, detectConfigChange(session, cfg))
}

func TestDetectConfigChange_EmptyHash_ReturnsFalse(t *testing.T) {
	empty := ""
	session := chat.ChatSession{ConfigHash: &empty}
	cfg := json.RawMessage(`{"provider":"anthropic"}`)
	assert.False(t, detectConfigChange(session, cfg))
}

func TestDetectConfigChange_SameHash_ReturnsFalse(t *testing.T) {
	cfg := json.RawMessage(`{"provider":"anthropic","model":"claude-3-5-sonnet-20241022"}`)
	h := hashConfig(cfg)
	session := chat.ChatSession{ConfigHash: &h}
	assert.False(t, detectConfigChange(session, cfg))
}

func TestDetectConfigChange_DifferentHash_ReturnsTrue(t *testing.T) {
	oldCfg := json.RawMessage(`{"provider":"anthropic","model":"claude-3-sonnet"}`)
	newCfg := json.RawMessage(`{"provider":"anthropic","model":"claude-3-5-sonnet-20241022"}`)
	h := hashConfig(oldCfg)
	session := chat.ChatSession{ConfigHash: &h}
	assert.True(t, detectConfigChange(session, newCfg))
}
