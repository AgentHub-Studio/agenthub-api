package agentic_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- AgenticError ---

func TestAgenticError_Message(t *testing.T) {
	err := agentic.NewAgenticError(344, "tool summary failed", nil)
	assert.Equal(t, "[344] tool summary failed", err.Error())
}

func TestAgenticError_WithCause(t *testing.T) {
	cause := errors.New("timeout")
	err := agentic.NewAgenticError(345, "stream fallback failed", cause)
	assert.Contains(t, err.Error(), "timeout")
	assert.Equal(t, cause, errors.Unwrap(err))
}

func TestAgenticError_SafeMessage(t *testing.T) {
	err := agentic.NewTelemetrySafeError(346, "compact failed on /secret/path.go", "compact failed", nil)
	assert.Equal(t, "compact failed", err.SafeMessage())
}

func TestAgenticError_SafeMessage_Fallback(t *testing.T) {
	err := agentic.NewAgenticError(347, "context overflow", nil)
	assert.Equal(t, "context overflow", err.SafeMessage())
}

func TestAgenticError_ErrorIDs(t *testing.T) {
	assert.Equal(t, 344, agentic.ErrIDToolUseSummaryFailed)
	assert.Equal(t, 345, agentic.ErrIDStreamFallbackFailed)
	assert.Equal(t, 346, agentic.ErrIDCompactFailed)
	assert.Equal(t, 347, agentic.ErrIDContextOverflow)
	assert.Equal(t, 354, agentic.ErrIDPermissionDenied)
}

// --- AbortError ---

func TestAbortError_Message(t *testing.T) {
	err := &agentic.AbortError{Message: "user cancelled"}
	assert.Equal(t, "user cancelled", err.Error())
}

func TestAbortError_DefaultMessage(t *testing.T) {
	err := &agentic.AbortError{}
	assert.Equal(t, "operation aborted", err.Error())
}

func TestIsAbortError_True(t *testing.T) {
	err := &agentic.AbortError{Message: "cancelled"}
	assert.True(t, agentic.IsAbortError(err))
}

func TestIsAbortError_Wrapped(t *testing.T) {
	err := fmt.Errorf("outer: %w", &agentic.AbortError{})
	assert.True(t, agentic.IsAbortError(err))
}

func TestIsAbortError_ContextCanceled(t *testing.T) {
	err := errors.New("context canceled")
	assert.True(t, agentic.IsAbortError(err))
}

func TestIsAbortError_Nil(t *testing.T) {
	assert.False(t, agentic.IsAbortError(nil))
}

func TestIsAbortError_Regular(t *testing.T) {
	assert.False(t, agentic.IsAbortError(errors.New("something else")))
}

// --- ShellError ---

func TestShellError_WithStderr(t *testing.T) {
	err := &agentic.ShellError{Stdout: "out", Stderr: "permission denied", ExitCode: 1}
	assert.Contains(t, err.Error(), "permission denied")
	assert.Contains(t, err.Error(), "exit 1")
}

func TestShellError_WithoutStderr(t *testing.T) {
	err := &agentic.ShellError{ExitCode: 127}
	assert.Contains(t, err.Error(), "exit 127")
}

func TestShellError_Interrupted(t *testing.T) {
	err := &agentic.ShellError{ExitCode: 130, Interrupted: true}
	assert.True(t, err.Interrupted)
}

// --- ConfigParseError ---

func TestConfigParseError(t *testing.T) {
	cause := errors.New("unexpected EOF")
	err := &agentic.ConfigParseError{
		FilePath:      "/etc/agent.yaml",
		DefaultConfig: map[string]string{},
		Cause:         cause,
	}
	assert.Contains(t, err.Error(), "/etc/agent.yaml")
	assert.Equal(t, cause, errors.Unwrap(err))
}

// --- ClassifyAPIError ---

func TestClassifyAPIError_Auth(t *testing.T) {
	result := agentic.ClassifyAPIError(401, nil)
	assert.Equal(t, agentic.APIErrorAuth, result.Kind)
	assert.Equal(t, 401, result.Status)
	assert.False(t, result.IsRetryable())

	result403 := agentic.ClassifyAPIError(403, nil)
	assert.Equal(t, agentic.APIErrorAuth, result403.Kind)
}

func TestClassifyAPIError_Timeout(t *testing.T) {
	result := agentic.ClassifyAPIError(408, nil)
	assert.Equal(t, agentic.APIErrorTimeout, result.Kind)
	assert.True(t, result.IsRetryable())
}

func TestClassifyAPIError_TimeoutFromError(t *testing.T) {
	result := agentic.ClassifyAPIError(0, errors.New("request timeout exceeded"))
	assert.Equal(t, agentic.APIErrorTimeout, result.Kind)
}

func TestClassifyAPIError_Network(t *testing.T) {
	result := agentic.ClassifyAPIError(0, errors.New("connection refused"))
	assert.Equal(t, agentic.APIErrorNetwork, result.Kind)
	assert.True(t, result.IsRetryable())
}

func TestClassifyAPIError_ServerError(t *testing.T) {
	result := agentic.ClassifyAPIError(500, nil)
	assert.Equal(t, agentic.APIErrorHTTP, result.Kind)
	assert.True(t, result.IsRetryable())
}

func TestClassifyAPIError_RateLimit(t *testing.T) {
	result := agentic.ClassifyAPIError(429, nil)
	assert.Equal(t, agentic.APIErrorHTTP, result.Kind)
	assert.True(t, result.IsRetryable())
}

func TestClassifyAPIError_ClientError(t *testing.T) {
	result := agentic.ClassifyAPIError(400, nil)
	assert.Equal(t, agentic.APIErrorHTTP, result.Kind)
	assert.False(t, result.IsRetryable())
}

func TestClassifyAPIError_Other(t *testing.T) {
	result := agentic.ClassifyAPIError(200, nil)
	assert.Equal(t, agentic.APIErrorOther, result.Kind)
	assert.False(t, result.IsRetryable())
}

// --- Utility functions ---

func TestToError_Nil(t *testing.T) {
	assert.Nil(t, agentic.ToError(nil))
}

func TestToError_Error(t *testing.T) {
	err := errors.New("test")
	assert.Equal(t, err, agentic.ToError(err))
}

func TestToError_String(t *testing.T) {
	err := agentic.ToError("something went wrong")
	require.NotNil(t, err)
	assert.Contains(t, err.Error(), "something went wrong")
}

func TestErrorMessage_Nil(t *testing.T) {
	assert.Equal(t, "", agentic.ErrorMessage(nil))
}

func TestErrorMessage_Error(t *testing.T) {
	assert.Equal(t, "test", agentic.ErrorMessage(errors.New("test")))
}

func TestErrorMessage_String(t *testing.T) {
	assert.Equal(t, "hello", agentic.ErrorMessage("hello"))
}

func TestShortErrorStack(t *testing.T) {
	stack := agentic.ShortErrorStack(3)
	assert.NotEmpty(t, stack)
	assert.Contains(t, stack, "TestShortErrorStack")
}

func TestShortErrorStack_DefaultFrames(t *testing.T) {
	stack := agentic.ShortErrorStack(0)
	assert.NotEmpty(t, stack)
}

func TestHasExactMessage_True(t *testing.T) {
	assert.True(t, agentic.HasExactMessage(errors.New("exact"), "exact"))
}

func TestHasExactMessage_False(t *testing.T) {
	assert.False(t, agentic.HasExactMessage(errors.New("something"), "other"))
}

func TestHasExactMessage_Nil(t *testing.T) {
	assert.False(t, agentic.HasExactMessage(nil, "test"))
}
