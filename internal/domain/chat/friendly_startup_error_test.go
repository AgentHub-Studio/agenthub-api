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
