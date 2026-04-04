package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestValidateBoundedIntStr_Empty(t *testing.T) {
	r := agentic.ValidateBoundedIntStr("TEST_VAR", "", 42, 100)
	assert.Equal(t, 42, r.Effective)
	assert.Equal(t, agentic.EnvVarValid, r.Status)
	assert.Empty(t, r.Message)
}

func TestValidateBoundedIntStr_ValidValue(t *testing.T) {
	r := agentic.ValidateBoundedIntStr("TEST_VAR", "50", 10, 100)
	assert.Equal(t, 50, r.Effective)
	assert.Equal(t, agentic.EnvVarValid, r.Status)
	assert.Empty(t, r.Message)
}

func TestValidateBoundedIntStr_ExactUpperLimit(t *testing.T) {
	r := agentic.ValidateBoundedIntStr("TEST_VAR", "100", 10, 100)
	assert.Equal(t, 100, r.Effective)
	assert.Equal(t, agentic.EnvVarValid, r.Status)
}

func TestValidateBoundedIntStr_Capped(t *testing.T) {
	r := agentic.ValidateBoundedIntStr("MAX_TOKENS", "200", 10, 100)
	assert.Equal(t, 100, r.Effective)
	assert.Equal(t, agentic.EnvVarCapped, r.Status)
	assert.Contains(t, r.Message, "capped from 200 to 100")
}

func TestValidateBoundedIntStr_NotANumber(t *testing.T) {
	r := agentic.ValidateBoundedIntStr("BATCH_SIZE", "abc", 10, 100)
	assert.Equal(t, 10, r.Effective)
	assert.Equal(t, agentic.EnvVarInvalid, r.Status)
	assert.Contains(t, r.Message, "invalid value")
}

func TestValidateBoundedIntStr_Zero(t *testing.T) {
	r := agentic.ValidateBoundedIntStr("TIMEOUT", "0", 5000, 60000)
	assert.Equal(t, 5000, r.Effective)
	assert.Equal(t, agentic.EnvVarInvalid, r.Status)
}

func TestValidateBoundedIntStr_Negative(t *testing.T) {
	r := agentic.ValidateBoundedIntStr("PORT", "-1", 8080, 65535)
	assert.Equal(t, 8080, r.Effective)
	assert.Equal(t, agentic.EnvVarInvalid, r.Status)
}

func TestValidateBoundedIntStr_One(t *testing.T) {
	r := agentic.ValidateBoundedIntStr("WORKERS", "1", 4, 32)
	assert.Equal(t, 1, r.Effective)
	assert.Equal(t, agentic.EnvVarValid, r.Status)
}

func TestValidateBoundedIntStr_Float(t *testing.T) {
	r := agentic.ValidateBoundedIntStr("RATE", "3.14", 10, 100)
	assert.Equal(t, 10, r.Effective)
	assert.Equal(t, agentic.EnvVarInvalid, r.Status)
}

func TestValidateBoundedIntStr_Whitespace(t *testing.T) {
	r := agentic.ValidateBoundedIntStr("VAR", " 50 ", 10, 100)
	// strconv.Atoi doesn't trim — this should be invalid
	assert.Equal(t, agentic.EnvVarInvalid, r.Status)
}

func TestEnvVarStatus_Values(t *testing.T) {
	assert.Equal(t, agentic.EnvVarStatus("valid"), agentic.EnvVarValid)
	assert.Equal(t, agentic.EnvVarStatus("capped"), agentic.EnvVarCapped)
	assert.Equal(t, agentic.EnvVarStatus("invalid"), agentic.EnvVarInvalid)
}
