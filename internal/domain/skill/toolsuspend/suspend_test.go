package toolsuspend_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill/toolsuspend"
)

func TestSuspendResumeRoundTrip(t *testing.T) {
	b := toolsuspend.NewResumeBuffer()
	ch := b.Suspend(toolsuspend.SuspendRequest{
		ExecutionID: "e1",
		ToolName:    "approve",
		Prompt:      "approve?",
		CreatedAt:   time.Now(),
	})

	if pending := b.Pending(); len(pending) != 1 {
		t.Fatalf("want 1 pending, got %d", len(pending))
	}

	go func() {
		_ = b.Resume(toolsuspend.ResumeData{ExecutionID: "e1", Data: map[string]any{"approved": true}})
	}()

	select {
	case d := <-ch:
		if d.Data["approved"] != true {
			t.Fatalf("bad payload: %+v", d)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for resume")
	}

	if pending := b.Pending(); len(pending) != 0 {
		t.Fatalf("pending not cleared: %d", len(pending))
	}
}

func TestResumeMissingExecution(t *testing.T) {
	b := toolsuspend.NewResumeBuffer()
	err := b.Resume(toolsuspend.ResumeData{ExecutionID: "nope"})
	if !errors.Is(err, toolsuspend.ErrNotSuspended) {
		t.Fatalf("want ErrNotSuspended, got %v", err)
	}
}
