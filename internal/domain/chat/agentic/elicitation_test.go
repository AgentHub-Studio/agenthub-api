package agentic_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewElicitationHandler ---

func TestNewElicitationHandler(t *testing.T) {
	h := agentic.NewElicitationHandler()
	assert.NotNil(t, h)
	assert.Equal(t, 0, h.PendingCount())
}

// --- Submit + Respond ---

func TestElicitation_SubmitAndRespond(t *testing.T) {
	h := agentic.NewElicitationHandler()

	var result agentic.ElicitationResult
	done := make(chan struct{})
	go func() {
		result = h.Submit(context.Background(), "server1", "req-1", agentic.ElicitationParams{
			Mode:    agentic.ElicitationModeForm,
			Message: "Confirm?",
		})
		close(done)
	}()

	// Wait for request to be queued
	time.Sleep(20 * time.Millisecond)
	assert.Equal(t, 1, h.PendingCount())

	ok := h.Respond("req-1", agentic.ElicitationResult{Action: agentic.ElicitationAccept})
	assert.True(t, ok)

	<-done
	assert.Equal(t, agentic.ElicitationAccept, result.Action)
}

// --- Submit cancelled by context ---

func TestElicitation_Submit_ContextCancel(t *testing.T) {
	h := agentic.NewElicitationHandler()
	ctx, cancel := context.WithCancel(context.Background())

	var result agentic.ElicitationResult
	done := make(chan struct{})
	go func() {
		result = h.Submit(ctx, "s", "req-c", agentic.ElicitationParams{
			Mode:    agentic.ElicitationModeForm,
			Message: "Do it?",
		})
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	<-done
	assert.Equal(t, agentic.ElicitationCancel, result.Action)
	assert.Equal(t, 0, h.PendingCount())
}

// --- Respond unknown request ---

func TestElicitation_Respond_Unknown(t *testing.T) {
	h := agentic.NewElicitationHandler()
	ok := h.Respond("nonexistent", agentic.ElicitationResult{Action: agentic.ElicitationAccept})
	assert.False(t, ok)
}

// --- PreHook resolves without queuing ---

func TestElicitation_PreHook(t *testing.T) {
	h := agentic.NewElicitationHandler()
	h.AddPreHook(func(server string, params agentic.ElicitationParams) *agentic.ElicitationResult {
		if params.Message == "auto-approve" {
			return &agentic.ElicitationResult{Action: agentic.ElicitationAccept}
		}
		return nil
	})

	result := h.Submit(context.Background(), "s", "req-h", agentic.ElicitationParams{
		Mode:    agentic.ElicitationModeForm,
		Message: "auto-approve",
	})
	assert.Equal(t, agentic.ElicitationAccept, result.Action)
	assert.Equal(t, 0, h.PendingCount(), "hook-resolved should not queue")
}

// --- PostHook modifies response ---

func TestElicitation_PostHook(t *testing.T) {
	h := agentic.NewElicitationHandler()
	h.AddPostHook(func(server string, result agentic.ElicitationResult) agentic.ElicitationResult {
		result.Content = map[string]interface{}{"modified": true}
		return result
	})

	done := make(chan agentic.ElicitationResult, 1)
	go func() {
		done <- h.Submit(context.Background(), "s", "req-p", agentic.ElicitationParams{
			Mode:    agentic.ElicitationModeForm,
			Message: "test",
		})
	}()

	time.Sleep(20 * time.Millisecond)
	h.Respond("req-p", agentic.ElicitationResult{Action: agentic.ElicitationAccept})

	result := <-done
	assert.Equal(t, agentic.ElicitationAccept, result.Action)
	assert.Equal(t, true, result.Content["modified"])
}

// --- OnEnqueue callback ---

func TestElicitation_OnEnqueue(t *testing.T) {
	h := agentic.NewElicitationHandler()

	var notified bool
	var mu sync.Mutex
	h.OnEnqueue(func(req *agentic.ElicitationRequest) {
		mu.Lock()
		notified = true
		mu.Unlock()
	})

	go func() {
		h.Submit(context.Background(), "s", "req-n", agentic.ElicitationParams{
			Mode:    agentic.ElicitationModeForm,
			Message: "hi",
		})
	}()

	time.Sleep(20 * time.Millisecond)
	mu.Lock()
	assert.True(t, notified)
	mu.Unlock()

	// Clean up
	h.Respond("req-n", agentic.ElicitationResult{Action: agentic.ElicitationCancel})
}

// --- MarkCompleted ---

func TestElicitation_MarkCompleted(t *testing.T) {
	h := agentic.NewElicitationHandler()

	go func() {
		h.Submit(context.Background(), "s", "req-m", agentic.ElicitationParams{
			Mode:          agentic.ElicitationModeURL,
			Message:       "Open URL",
			ElicitationID: "elic-123",
		})
	}()

	time.Sleep(20 * time.Millisecond)
	ok := h.MarkCompleted("elic-123")
	assert.True(t, ok)

	pending := h.Pending()
	require.Len(t, pending, 1)
	assert.True(t, pending[0].Completed)

	// Clean up
	h.Respond("req-m", agentic.ElicitationResult{Action: agentic.ElicitationAccept})
}

func TestElicitation_MarkCompleted_NotFound(t *testing.T) {
	h := agentic.NewElicitationHandler()
	ok := h.MarkCompleted("unknown")
	assert.False(t, ok)
}

// --- Pending ---

func TestElicitation_Pending(t *testing.T) {
	h := agentic.NewElicitationHandler()

	go func() {
		h.Submit(context.Background(), "s", "r1", agentic.ElicitationParams{Mode: agentic.ElicitationModeForm, Message: "a"})
	}()
	go func() {
		h.Submit(context.Background(), "s", "r2", agentic.ElicitationParams{Mode: agentic.ElicitationModeForm, Message: "b"})
	}()

	time.Sleep(30 * time.Millisecond)
	assert.Equal(t, 2, h.PendingCount())

	h.Respond("r1", agentic.ElicitationResult{Action: agentic.ElicitationAccept})
	time.Sleep(10 * time.Millisecond)
	assert.Equal(t, 1, h.PendingCount())

	// Clean up
	h.Respond("r2", agentic.ElicitationResult{Action: agentic.ElicitationCancel})
}

// --- Clear ---

func TestElicitation_Clear(t *testing.T) {
	h := agentic.NewElicitationHandler()

	go func() {
		h.Submit(context.Background(), "s", "r1", agentic.ElicitationParams{Mode: agentic.ElicitationModeForm, Message: "a"})
	}()
	go func() {
		h.Submit(context.Background(), "s", "r2", agentic.ElicitationParams{Mode: agentic.ElicitationModeForm, Message: "b"})
	}()

	time.Sleep(20 * time.Millisecond)
	h.Respond("r1", agentic.ElicitationResult{Action: agentic.ElicitationAccept})
	time.Sleep(10 * time.Millisecond)

	h.Clear()
	// r1 responded → removed, r2 still pending
	assert.Equal(t, 1, h.PendingCount())

	// Clean up
	h.Respond("r2", agentic.ElicitationResult{Action: agentic.ElicitationCancel})
}

// --- Modes ---

func TestElicitationMode_Values(t *testing.T) {
	assert.Equal(t, agentic.ElicitationMode("form"), agentic.ElicitationModeForm)
	assert.Equal(t, agentic.ElicitationMode("url"), agentic.ElicitationModeURL)
}

// --- Actions ---

func TestElicitationAction_Values(t *testing.T) {
	assert.Equal(t, agentic.ElicitationAction("accept"), agentic.ElicitationAccept)
	assert.Equal(t, agentic.ElicitationAction("decline"), agentic.ElicitationDecline)
	assert.Equal(t, agentic.ElicitationAction("cancel"), agentic.ElicitationCancel)
}
