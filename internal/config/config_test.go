package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/config"
)

func TestLoad_AuditRetentionIntervalDefault(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agenthub?sslmode=disable")
	t.Setenv("KEYCLOAK_BASE_URL", "http://localhost:28080")
	t.Setenv("AUDIT_RETENTION_INTERVAL_SECS", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 86400, cfg.AuditRetentionIntervalSecs)
}

func TestLoad_LLMCallTimeoutDefaultMatchesRT01(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agenthub?sslmode=disable")
	t.Setenv("KEYCLOAK_BASE_URL", "http://localhost:28080")
	t.Setenv("LLM_CALL_TIMEOUT_SECS", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 120, cfg.LLMCallTimeoutSecs)
}

func TestLoad_LLMCallTimeoutFromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agenthub?sslmode=disable")
	t.Setenv("KEYCLOAK_BASE_URL", "http://localhost:28080")
	t.Setenv("LLM_CALL_TIMEOUT_SECS", "42")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 42, cfg.LLMCallTimeoutSecs)
}

func TestLoad_AuditRetentionIntervalFromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agenthub?sslmode=disable")
	t.Setenv("KEYCLOAK_BASE_URL", "http://localhost:28080")
	t.Setenv("AUDIT_RETENTION_INTERVAL_SECS", "2")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, 2, cfg.AuditRetentionIntervalSecs)
}

func TestLoad_BackendBaseURLDefaultUsesAPIPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agenthub?sslmode=disable")
	t.Setenv("KEYCLOAK_BASE_URL", "http://localhost:28080")
	t.Setenv("PORT", "18081")
	t.Setenv("BACKEND_BASE_URL", "")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "http://localhost:18081", cfg.BackendBaseURL)
}

func TestLoad_BackendBaseURLFromEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/agenthub?sslmode=disable")
	t.Setenv("KEYCLOAK_BASE_URL", "http://localhost:28080")
	t.Setenv("BACKEND_BASE_URL", "http://agenthub-api:8081")

	cfg, err := config.Load()

	require.NoError(t, err)
	assert.Equal(t, "http://agenthub-api:8081", cfg.BackendBaseURL)
}
