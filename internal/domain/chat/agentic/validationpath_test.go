package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- FormatValidationPath ---

func TestFormatValidationPath_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.FormatValidationPath(nil))
}

func TestFormatValidationPath_SingleKey(t *testing.T) {
	assert.Equal(t, "name", agentic.FormatValidationPath([]string{"name"}))
}

func TestFormatValidationPath_Nested(t *testing.T) {
	assert.Equal(t, "user.address.city",
		agentic.FormatValidationPath([]string{"user", "address", "city"}))
}

func TestFormatValidationPath_WithIndex(t *testing.T) {
	assert.Equal(t, "todos[0].activeForm",
		agentic.FormatValidationPath([]string{"todos", "0", "activeForm"}))
}

func TestFormatValidationPath_MultipleIndices(t *testing.T) {
	assert.Equal(t, "items[2].tags[0]",
		agentic.FormatValidationPath([]string{"items", "2", "tags", "0"}))
}

func TestFormatValidationPath_OnlyIndex(t *testing.T) {
	assert.Equal(t, "[0]",
		agentic.FormatValidationPath([]string{"0"}))
}

// --- FormatValidationError ---

func TestFormatValidationError_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.FormatValidationError("myTool", nil))
}

func TestFormatValidationError_MissingParam(t *testing.T) {
	issues := []agentic.ValidationIssue{
		{Type: agentic.ValidationMissing, Path: "name"},
	}
	result := agentic.FormatValidationError("myTool", issues)
	assert.Contains(t, result, "myTool failed")
	assert.Contains(t, result, "issue:")
	assert.Contains(t, result, "required parameter `name` is missing")
}

func TestFormatValidationError_UnexpectedParam(t *testing.T) {
	issues := []agentic.ValidationIssue{
		{Type: agentic.ValidationUnexpected, Path: "extra"},
	}
	result := agentic.FormatValidationError("search", issues)
	assert.Contains(t, result, "unexpected parameter `extra`")
}

func TestFormatValidationError_TypeMismatch(t *testing.T) {
	issues := []agentic.ValidationIssue{
		{Type: agentic.ValidationTypeMismatch, Path: "count", Expected: "number", Received: "string"},
	}
	result := agentic.FormatValidationError("update", issues)
	assert.Contains(t, result, "expected as `number`")
	assert.Contains(t, result, "provided as `string`")
}

func TestFormatValidationError_MultipleIssues(t *testing.T) {
	issues := []agentic.ValidationIssue{
		{Type: agentic.ValidationMissing, Path: "name"},
		{Type: agentic.ValidationUnexpected, Path: "foo"},
		{Type: agentic.ValidationTypeMismatch, Path: "count", Expected: "number", Received: "string"},
	}
	result := agentic.FormatValidationError("myTool", issues)
	assert.Contains(t, result, "issues:")
	assert.Contains(t, result, "required parameter `name`")
	assert.Contains(t, result, "unexpected parameter `foo`")
	assert.Contains(t, result, "`count` type is expected")
}

func TestFormatValidationError_SingularIssue(t *testing.T) {
	issues := []agentic.ValidationIssue{
		{Type: agentic.ValidationMissing, Path: "x"},
	}
	result := agentic.FormatValidationError("t", issues)
	assert.Contains(t, result, "issue:")
	assert.NotContains(t, result, "issues:")
}
