package ai_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

func TestAPIError_ErrorsIs(t *testing.T) {
	ae := &ai.APIError{StatusCode: 429, Message: "rate limit", Err: ai.ErrRateLimited}
	assert.True(t, errors.Is(ae, ai.ErrRateLimited))
	assert.False(t, errors.Is(ae, ai.ErrInvalidAPIKey))
}

func TestAPIError_ErrorMessage(t *testing.T) {
	ae := &ai.APIError{StatusCode: 401, Message: "unauthorized", Err: ai.ErrInvalidAPIKey, RetryAfter: -1}
	assert.Contains(t, ae.Error(), "401")
	assert.Contains(t, ae.Error(), "unauthorized")
}

func TestAPIError_Unwrap(t *testing.T) {
	ae := &ai.APIError{StatusCode: 500, Message: "server error", Err: ai.ErrProviderUnavailable}
	require.Equal(t, ai.ErrProviderUnavailable, errors.Unwrap(ae))
}

func TestChatOptions_Merge_OverwritesNonZero(t *testing.T) {
	base := ai.ChatOptions{Model: "gpt-4o", MaxTokens: 512, Temperature: 0.7}
	override := ai.ChatOptions{MaxTokens: 1024, SystemMsg: "You are helpful."}

	merged := base.Merge(override)
	assert.Equal(t, "gpt-4o", merged.Model)               // kept from base
	assert.Equal(t, 1024, merged.MaxTokens)               // overridden
	assert.Equal(t, 0.7, merged.Temperature)              // kept from base
	assert.Equal(t, "You are helpful.", merged.SystemMsg) // added from override
}

func TestChatOptions_Merge_ZeroFieldsDoNotOverwrite(t *testing.T) {
	base := ai.ChatOptions{Model: "gpt-4o", MaxTokens: 512}
	override := ai.ChatOptions{} // all zero

	merged := base.Merge(override)
	assert.Equal(t, "gpt-4o", merged.Model)
	assert.Equal(t, 512, merged.MaxTokens)
}
