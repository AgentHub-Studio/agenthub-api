package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func mockLookup(vars map[string]string) func(string) string {
	return func(key string) string {
		return vars[key]
	}
}

func TestExpandEnvVars_Simple(t *testing.T) {
	lookup := mockLookup(map[string]string{"HOME": "/home/user"})
	result := agentic.ExpandEnvVarsWithLookup("path: ${HOME}/data", lookup)
	assert.Equal(t, "path: /home/user/data", result.Expanded)
	assert.Empty(t, result.MissingVars)
}

func TestExpandEnvVars_Multiple(t *testing.T) {
	lookup := mockLookup(map[string]string{"HOST": "localhost", "PORT": "8080"})
	result := agentic.ExpandEnvVarsWithLookup("${HOST}:${PORT}", lookup)
	assert.Equal(t, "localhost:8080", result.Expanded)
	assert.Empty(t, result.MissingVars)
}

func TestExpandEnvVars_WithDefault(t *testing.T) {
	lookup := mockLookup(map[string]string{})
	result := agentic.ExpandEnvVarsWithLookup("port: ${PORT:-8080}", lookup)
	assert.Equal(t, "port: 8080", result.Expanded)
	assert.Empty(t, result.MissingVars)
}

func TestExpandEnvVars_DefaultNotUsedWhenSet(t *testing.T) {
	lookup := mockLookup(map[string]string{"PORT": "9090"})
	result := agentic.ExpandEnvVarsWithLookup("port: ${PORT:-8080}", lookup)
	assert.Equal(t, "port: 9090", result.Expanded)
	assert.Empty(t, result.MissingVars)
}

func TestExpandEnvVars_Missing(t *testing.T) {
	lookup := mockLookup(map[string]string{})
	result := agentic.ExpandEnvVarsWithLookup("url: ${API_URL}/v1", lookup)
	assert.Equal(t, "url: ${API_URL}/v1", result.Expanded) // preserved for debugging
	assert.Equal(t, []string{"API_URL"}, result.MissingVars)
}

func TestExpandEnvVars_MultipleMissing(t *testing.T) {
	lookup := mockLookup(map[string]string{})
	result := agentic.ExpandEnvVarsWithLookup("${A}:${B}", lookup)
	assert.Equal(t, []string{"A", "B"}, result.MissingVars)
}

func TestExpandEnvVars_NoPlaceholders(t *testing.T) {
	lookup := mockLookup(map[string]string{})
	result := agentic.ExpandEnvVarsWithLookup("plain text", lookup)
	assert.Equal(t, "plain text", result.Expanded)
	assert.Empty(t, result.MissingVars)
}

func TestExpandEnvVars_Empty(t *testing.T) {
	lookup := mockLookup(map[string]string{})
	result := agentic.ExpandEnvVarsWithLookup("", lookup)
	assert.Equal(t, "", result.Expanded)
	assert.Empty(t, result.MissingVars)
}

func TestExpandEnvVars_DefaultWithColon(t *testing.T) {
	lookup := mockLookup(map[string]string{})
	result := agentic.ExpandEnvVarsWithLookup("${URL:-http://localhost:8080}", lookup)
	assert.Equal(t, "http://localhost:8080", result.Expanded)
	assert.Empty(t, result.MissingVars)
}

func TestExpandEnvVars_MixedFoundAndMissing(t *testing.T) {
	lookup := mockLookup(map[string]string{"HOST": "db.local"})
	result := agentic.ExpandEnvVarsWithLookup("${HOST}:${PORT}", lookup)
	assert.Equal(t, "db.local:${PORT}", result.Expanded)
	assert.Equal(t, []string{"PORT"}, result.MissingVars)
}

func TestExpandEnvVars_EmptyDefault(t *testing.T) {
	lookup := mockLookup(map[string]string{})
	result := agentic.ExpandEnvVarsWithLookup("val=${X:-}", lookup)
	assert.Equal(t, "val=", result.Expanded)
	assert.Empty(t, result.MissingVars)
}
