package mcp

import (
	"strings"
	"testing"
)

func TestRuntimeToolsFetchLogMessageOmitsRuntimeURL(t *testing.T) {
	message := runtimeToolsFetchLogMessage()

	if strings.Contains(message, "https://runtime.example") {
		t.Fatalf("runtime tools log message must not include runtime URL: %q", message)
	}
	if strings.ContainsAny(message, "\r\n") {
		t.Fatalf("runtime tools log message must be single-line: %q", message)
	}
}
