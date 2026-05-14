package chat

import (
	"strings"
	"testing"
)

// --- friendlyStartupError tests ---

func TestFriendlyStartupError_DraftAgent(t *testing.T) {
	msg := friendlyStartupError("agent is not published: agent abc is in DRAFT status")
	if !strings.Contains(msg, "publicado") {
		t.Errorf("expected 'publicado' in message, got: %s", msg)
	}
}

func TestFriendlyStartupError_NotPublished(t *testing.T) {
	msg := friendlyStartupError("agent is not published")
	if !strings.Contains(msg, "publicado") {
		t.Errorf("expected 'publicado' in message, got: %s", msg)
	}
}

func TestFriendlyStartupError_Archived(t *testing.T) {
	msg := friendlyStartupError("agent is archived and no longer accepts new sessions")
	if !strings.Contains(msg, "arquivado") {
		t.Errorf("expected 'arquivado' in message, got: %s", msg)
	}
}

func TestFriendlyStartupError_UnsupportedProvider(t *testing.T) {
	msg := friendlyStartupError("unsupported provider: foobar")
	if !strings.Contains(msg, "provedor") {
		t.Errorf("expected 'provedor' in message, got: %s", msg)
	}
}

func TestFriendlyStartupError_APIKey(t *testing.T) {
	msg := friendlyStartupError("no AI provider configured: API key missing")
	if !strings.Contains(msg, "API") || !strings.Contains(msg, "chave") {
		t.Errorf("expected API key message, got: %s", msg)
	}
}

func TestFriendlyStartupError_BuildModel(t *testing.T) {
	msg := friendlyStartupError("build model failed: invalid config")
	if !strings.Contains(msg, "modelo") {
		t.Errorf("expected 'modelo' in message, got: %s", msg)
	}
}

func TestFriendlyStartupError_Generic(t *testing.T) {
	msg := friendlyStartupError("some unknown internal error")
	if strings.Contains(msg, "publicado") || strings.Contains(msg, "arquivado") {
		t.Errorf("generic error should not mention publish/archive status, got: %s", msg)
	}
	if !strings.Contains(msg, "Não foi possível iniciar") {
		t.Errorf("expected generic startup error message, got: %s", msg)
	}
}

func TestFriendlyStartupError_NoAgentAvailable(t *testing.T) {
	// Both the SSE-path wrapped error and the async-path literal must resolve to
	// the same friendly, actionable message instead of falling through to the
	// generic "Detalhes técnicos" branch.
	for _, raw := range []string{
		"no published agent available for routing",
		"chat service: no published agent available for routing",
		"no agent configured for this session and no default published agent found",
	} {
		msg := friendlyStartupError(raw)
		if !strings.Contains(msg, "agente publicado") {
			t.Errorf("expected 'agente publicado' in message for %q, got: %s", raw, msg)
		}
		if strings.Contains(msg, "Detalhes técnicos") {
			t.Errorf("no-agent error should be friendly, not generic, for %q, got: %s", raw, msg)
		}
	}
}
