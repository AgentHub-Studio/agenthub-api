package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-go-commons/config"
)

// testCfg is a minimal config struct used to exercise Load[T] without requiring
// all Common fields to be set.
type testCfg struct {
	Host string `env:"TEST_HOST,required"`
	Port int    `env:"TEST_PORT" envDefault:"8080"`
}

// optionalCfg has no required fields, used for defaults testing.
type optionalCfg struct {
	Port     int    `env:"OPT_PORT"     envDefault:"9090"`
	LogLevel string `env:"OPT_LOG_LEVEL" envDefault:"warn"`
}

func TestLoad_Success(t *testing.T) {
	t.Setenv("TEST_HOST", "localhost")
	t.Setenv("TEST_PORT", "3000")

	cfg, err := config.Load[testCfg]()

	require.NoError(t, err)
	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 3000, cfg.Port)
}

func TestLoad_MissingRequired(t *testing.T) {
	// TEST_HOST is required but not set; TEST_PORT has a default.
	// Do NOT call t.Setenv("TEST_HOST", ...) so it remains unset.
	_, err := config.Load[testCfg]()

	require.Error(t, err)
	assert.Contains(t, err.Error(), "config: parse env")
}

func TestLoad_Defaults(t *testing.T) {
	// No env vars set — both fields should fall back to envDefault values.
	cfg, err := config.Load[optionalCfg]()

	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.Port)
	assert.Equal(t, "warn", cfg.LogLevel)
}

func TestLoad_CustomStruct(t *testing.T) {
	type customCfg struct {
		ServiceName string `env:"SVC_NAME" envDefault:"my-service"`
		MaxRetries  int    `env:"SVC_MAX_RETRIES" envDefault:"3"`
	}

	t.Setenv("SVC_NAME", "custom-service")
	// SVC_MAX_RETRIES not set → should use default

	cfg, err := config.Load[customCfg]()

	require.NoError(t, err)
	assert.Equal(t, "custom-service", cfg.ServiceName)
	assert.Equal(t, 3, cfg.MaxRetries)
}
