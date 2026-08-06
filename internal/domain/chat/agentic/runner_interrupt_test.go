package agentic

import (
	"context"
	"testing"
)

func TestContextForToolResultPersistenceDetachesCancelledRun(t *testing.T) {
	type contextKey string
	const key contextKey = "tenant"
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), key, "tenant-a"))
	cancel()

	persistCtx, cleanup := contextForToolResultPersistence(parent)
	defer cleanup()

	if got := persistCtx.Value(key); got != "tenant-a" {
		t.Fatalf("persist context lost parent value: got %v", got)
	}
	select {
	case <-persistCtx.Done():
		t.Fatal("persist context must remain usable after run cancellation")
	default:
	}
}
