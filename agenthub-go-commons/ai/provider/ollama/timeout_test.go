package ollama

import (
	"testing"
	"time"
)

func TestResolveTimeout_DefaultWhenUnset(t *testing.T) {
	t.Setenv("OLLAMA_TIMEOUT_SECONDS", "")
	if got := resolveTimeout(); got != defaultOllamaTimeout {
		t.Errorf("unset env → got %v, want %v", got, defaultOllamaTimeout)
	}
}

func TestResolveTimeout_ValidOverride(t *testing.T) {
	t.Setenv("OLLAMA_TIMEOUT_SECONDS", "45")
	if got := resolveTimeout(); got != 45*time.Second {
		t.Errorf("env=45 → got %v, want 45s", got)
	}
}

func TestResolveTimeout_InvalidFallsBack(t *testing.T) {
	for _, v := range []string{"abc", "-5", "0", " 30 ", "30s"} {
		t.Setenv("OLLAMA_TIMEOUT_SECONDS", v)
		if got := resolveTimeout(); got != defaultOllamaTimeout {
			t.Errorf("invalid %q → got %v, want default", v, got)
		}
	}
}

func TestDefaultOllamaTimeoutIsGenerous(t *testing.T) {
	// Lock in the new default so regressions bump CPU inference back to
	// 120s are caught immediately.
	if defaultOllamaTimeout < 5*time.Minute {
		t.Errorf("default timeout %v is too short for local CPU inference; must be ≥5min",
			defaultOllamaTimeout)
	}
}
