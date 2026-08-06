package main

import (
	"strings"
	"testing"
)

func TestChatRunPublishedLogMessageOmitsUntrustedSessionID(t *testing.T) {
	message := chatRunPublishedLogMessage()

	if strings.Contains(message, "attacker-session") {
		t.Fatalf("published log message must not include untrusted session id: %q", message)
	}
	if strings.ContainsAny(message, "\r\n") {
		t.Fatalf("published log message must be single-line: %q", message)
	}
}
