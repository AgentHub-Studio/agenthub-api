package agentic

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// --- FrontendActionHandler ---------------------------------------------------

func TestFrontendActionHandler_SubmitAndRespond(t *testing.T) {
	h := NewFrontendActionHandler()
	sessionID := uuid.New()

	done := make(chan FrontendActionResult, 1)
	go func() {
		done <- h.Submit(context.Background(), sessionID, "call-1", "navigate_to",
			json.RawMessage(`{"route":"/x"}`))
	}()

	// Wait briefly for the request to land in the queue.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(h.Pending()) > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(h.Pending()) != 1 {
		t.Fatalf("expected 1 pending request, got %d", len(h.Pending()))
	}

	ok := h.Respond("call-1", FrontendActionResult{
		Status: "ok",
		Result: json.RawMessage(`{"navigated":true}`),
	})
	if !ok {
		t.Fatal("Respond returned false; expected the pending request to resolve")
	}

	select {
	case got := <-done:
		if got.Status != "ok" {
			t.Errorf("status = %q, want %q", got.Status, "ok")
		}
		if got.ID != "call-1" {
			t.Errorf("ID = %q, want %q (Respond must echo the callID)", got.ID, "call-1")
		}
	case <-time.After(time.Second):
		t.Fatal("Submit did not return within 1s")
	}
}

func TestFrontendActionHandler_Respond_UnknownCallID(t *testing.T) {
	h := NewFrontendActionHandler()
	if h.Respond("nonexistent", FrontendActionResult{Status: "ok"}) {
		t.Fatal("expected Respond on unknown callID to return false")
	}
}

func TestFrontendActionHandler_ContextCancel(t *testing.T) {
	h := NewFrontendActionHandler()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan FrontendActionResult, 1)
	go func() {
		done <- h.Submit(ctx, uuid.New(), "call-1", "x", nil)
	}()

	// Cancel before any Respond — Submit must unblock with status=error.
	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case got := <-done:
		if got.Status != "error" || got.Error != "cancelled" {
			t.Errorf("status=%q error=%q, want error/cancelled", got.Status, got.Error)
		}
	case <-time.After(time.Second):
		t.Fatal("Submit did not return after context cancellation")
	}

	// The request must have been evicted from the queue.
	if len(h.Pending()) != 0 {
		t.Errorf("Pending after cancel = %d, want 0", len(h.Pending()))
	}
}

func TestFrontendActionHandler_OnEnqueue(t *testing.T) {
	h := NewFrontendActionHandler()
	var seen []*FrontendActionRequest
	var mu sync.Mutex
	h.OnEnqueue(func(req *FrontendActionRequest) {
		mu.Lock()
		defer mu.Unlock()
		seen = append(seen, req)
	})

	go h.Submit(context.Background(), uuid.New(), "c1", "n", nil)
	go h.Submit(context.Background(), uuid.New(), "c2", "n", nil)

	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(seen)
		mu.Unlock()
		if count == 2 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 2 {
		t.Fatalf("OnEnqueue fired %d times, want 2", len(seen))
	}
}

// --- ClientStateStore --------------------------------------------------------

func TestClientStateStore_ApplyAndGetActions(t *testing.T) {
	s := NewClientStateStore(0)
	sessionID := uuid.New()

	s.Apply(sessionID, ClientStatePatch{
		FrontendActions: []FrontendAction{
			{Name: "a", Description: "first"},
			{Name: "b", Description: "second"},
		},
	})

	got := s.GetActions(sessionID)
	if len(got) != 2 {
		t.Fatalf("GetActions = %d, want 2", len(got))
	}
	if got[0].Name != "a" || got[1].Name != "b" {
		t.Errorf("unexpected order: %+v", got)
	}
}

func TestClientStateStore_ApplyOverwritesActions(t *testing.T) {
	s := NewClientStateStore(0)
	sessionID := uuid.New()

	s.Apply(sessionID, ClientStatePatch{
		FrontendActions: []FrontendAction{{Name: "a"}, {Name: "b"}},
	})
	s.Apply(sessionID, ClientStatePatch{
		FrontendActions: []FrontendAction{{Name: "c"}},
	})

	got := s.GetActions(sessionID)
	if len(got) != 1 || got[0].Name != "c" {
		t.Errorf("overwrite failed; got %+v", got)
	}
}

func TestClientStateStore_ApplyNilActions_PreservesExisting(t *testing.T) {
	s := NewClientStateStore(0)
	sessionID := uuid.New()

	s.Apply(sessionID, ClientStatePatch{
		FrontendActions: []FrontendAction{{Name: "a"}},
	})
	// Patch with only readables — actions must remain.
	s.Apply(sessionID, ClientStatePatch{
		Readables: []Readable{{ID: "r1", Description: "d", Value: json.RawMessage(`1`)}},
	})

	if len(s.GetActions(sessionID)) != 1 {
		t.Errorf("actions wiped by readables-only patch")
	}
	if len(s.GetReadables(sessionID)) != 1 {
		t.Errorf("readables not stored")
	}
}

func TestClientStateStore_GetReadables(t *testing.T) {
	s := NewClientStateStore(0)
	sessionID := uuid.New()

	s.Apply(sessionID, ClientStatePatch{
		Readables: []Readable{
			{ID: "route", Description: "current route", Value: json.RawMessage(`"/x"`)},
		},
	})

	got := s.GetReadables(sessionID)
	if len(got) != 1 || got[0].ID != "route" {
		t.Fatalf("GetReadables = %+v, want [route]", got)
	}
}

func TestClientStateStore_ActionResultsDispatch(t *testing.T) {
	s := NewClientStateStore(0)
	sessionID := uuid.New()

	h := NewFrontendActionHandler()
	s.AttachHandler(sessionID, h)

	done := make(chan FrontendActionResult, 1)
	go func() {
		done <- h.Submit(context.Background(), sessionID, "call-X", "navigate_to", nil)
	}()

	// Wait for Submit to enqueue.
	for i := 0; i < 100 && len(h.Pending()) == 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}

	s.Apply(sessionID, ClientStatePatch{
		ActionResults: []FrontendActionResult{
			{ID: "call-X", Status: "ok", Result: json.RawMessage(`{"ok":true}`)},
		},
	})

	select {
	case got := <-done:
		if got.Status != "ok" {
			t.Errorf("status = %q, want ok", got.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("action result was not dispatched to handler")
	}
}

func TestClientStateStore_ActionResultsWithoutHandler_AreNoOp(t *testing.T) {
	s := NewClientStateStore(0)
	sessionID := uuid.New()

	// No handler attached — Apply must not panic.
	s.Apply(sessionID, ClientStatePatch{
		ActionResults: []FrontendActionResult{
			{ID: "x", Status: "ok"},
		},
	})
}

func TestClientStateStore_Submit_WithoutHandler_ReturnsError(t *testing.T) {
	s := NewClientStateStore(0)
	got := s.Submit(context.Background(), uuid.New(), "c", "n", nil)
	if got.Status != "error" || got.Error == "" {
		t.Errorf("expected error result; got %+v", got)
	}
}

func TestClientStateStore_Submit_ProxiesToHandler(t *testing.T) {
	s := NewClientStateStore(0)
	sessionID := uuid.New()
	h := NewFrontendActionHandler()
	s.AttachHandler(sessionID, h)

	done := make(chan FrontendActionResult, 1)
	go func() {
		done <- s.Submit(context.Background(), sessionID, "c-1", "n", nil)
	}()

	for i := 0; i < 100 && len(h.Pending()) == 0; i++ {
		time.Sleep(5 * time.Millisecond)
	}
	h.Respond("c-1", FrontendActionResult{Status: "ok"})

	select {
	case got := <-done:
		if got.Status != "ok" {
			t.Errorf("status = %q, want ok", got.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("Submit via store did not return")
	}
}

func TestClientStateStore_DetachHandler_StopsResultDispatch(t *testing.T) {
	s := NewClientStateStore(0)
	sessionID := uuid.New()
	h := NewFrontendActionHandler()
	s.AttachHandler(sessionID, h)
	s.DetachHandler(sessionID)

	// The handler is no longer attached — Apply should not dispatch results.
	s.Apply(sessionID, ClientStatePatch{
		ActionResults: []FrontendActionResult{{ID: "x", Status: "ok"}},
	})
	if len(h.Pending()) != 0 {
		t.Errorf("handler should be detached; saw pending=%d", len(h.Pending()))
	}
}

func TestClientStateStore_Cleanup_EvictsStale(t *testing.T) {
	s := NewClientStateStore(10 * time.Millisecond)
	sessionID := uuid.New()

	s.Apply(sessionID, ClientStatePatch{
		FrontendActions: []FrontendAction{{Name: "a"}},
	})
	// Ensure entry exists.
	if len(s.GetActions(sessionID)) == 0 {
		t.Fatal("setup: actions not stored")
	}

	time.Sleep(20 * time.Millisecond)
	s.Cleanup()

	if len(s.GetActions(sessionID)) != 0 {
		t.Errorf("Cleanup did not evict stale session")
	}
}

func TestClientStateStore_Cleanup_DoesNotEvictWithHandler(t *testing.T) {
	s := NewClientStateStore(10 * time.Millisecond)
	sessionID := uuid.New()

	s.Apply(sessionID, ClientStatePatch{
		FrontendActions: []FrontendAction{{Name: "a"}},
	})
	s.AttachHandler(sessionID, NewFrontendActionHandler())

	time.Sleep(20 * time.Millisecond)
	s.Cleanup()

	if len(s.GetActions(sessionID)) == 0 {
		t.Errorf("Cleanup evicted a session with an active handler")
	}
}

func TestClientStateStore_Cleanup_ZeroTTL_IsNoOp(t *testing.T) {
	s := NewClientStateStore(0)
	sessionID := uuid.New()
	s.Apply(sessionID, ClientStatePatch{FrontendActions: []FrontendAction{{Name: "a"}}})
	time.Sleep(5 * time.Millisecond)
	s.Cleanup()
	if len(s.GetActions(sessionID)) != 1 {
		t.Errorf("zero-TTL Cleanup evicted entry it should not have")
	}
}

func TestClientStateStore_UnknownSession_ReturnsNil(t *testing.T) {
	s := NewClientStateStore(0)
	if got := s.GetActions(uuid.New()); got != nil {
		t.Errorf("GetActions on unknown session = %+v, want nil", got)
	}
	if got := s.GetReadables(uuid.New()); got != nil {
		t.Errorf("GetReadables on unknown session = %+v, want nil", got)
	}
}

// --- FormatAppStateBlock -----------------------------------------------------

func TestFormatAppStateBlock_Empty_ReturnsEmptyString(t *testing.T) {
	if got := FormatAppStateBlock(nil); got != "" {
		t.Errorf("nil readables → %q, want \"\"", got)
	}
	if got := FormatAppStateBlock([]Readable{}); got != "" {
		t.Errorf("empty readables → %q, want \"\"", got)
	}
}

func TestFormatAppStateBlock_RendersSingleReadable(t *testing.T) {
	readables := []Readable{
		{
			ID:          "route",
			Description: "current route",
			Value:       json.RawMessage(`"/standalone/abc"`),
		},
	}
	got := FormatAppStateBlock(readables)
	wantContains := []string{
		"<app_state>",
		`<readable id="route" description="current route">`,
		`"/standalone/abc"`,
		"</readable>",
		"</app_state>",
	}
	for _, w := range wantContains {
		if !strings.Contains(got, w) {
			t.Errorf("output missing %q\n----- got -----\n%s", w, got)
		}
	}
}

func TestFormatAppStateBlock_RendersMultipleWithParent(t *testing.T) {
	readables := []Readable{
		{ID: "auth", Description: "auth bundle", Value: json.RawMessage(`{}`)},
		{
			ID:          "user",
			Description: "logged-in user",
			ParentID:    "auth",
			Value:       json.RawMessage(`{"name":"Cezar"}`),
		},
	}
	got := FormatAppStateBlock(readables)
	if !strings.Contains(got, `id="auth"`) {
		t.Errorf("missing first readable")
	}
	if !strings.Contains(got, `parent="auth"`) {
		t.Errorf("missing parent attribute on child")
	}
	if !strings.Contains(got, `{"name":"Cezar"}`) {
		t.Errorf("missing JSON-encoded value")
	}
}

func TestFormatAppStateBlock_EscapesXMLMetaCharsInAttributes(t *testing.T) {
	readables := []Readable{
		{
			ID:          `it's "weird" & <bad>`,
			Description: `also "weird"`,
			Value:       json.RawMessage(`true`),
		},
	}
	got := FormatAppStateBlock(readables)
	bad := []string{`id="it's`, `id="it"s`, `description="also "weird""`}
	for _, b := range bad {
		if strings.Contains(got, b) {
			t.Errorf("unescaped chars leaked into attribute: %q", b)
		}
	}
	wantContains := []string{
		`id="it&apos;s &quot;weird&quot; &amp; &lt;bad&gt;"`,
		`description="also &quot;weird&quot;"`,
	}
	for _, w := range wantContains {
		if !strings.Contains(got, w) {
			t.Errorf("expected escaped: %q\n----- got -----\n%s", w, got)
		}
	}
}

func TestFormatAppStateBlock_EmptyValue_RendersNull(t *testing.T) {
	readables := []Readable{
		{ID: "x", Description: "y"},
	}
	got := FormatAppStateBlock(readables)
	if !strings.Contains(got, "null") {
		t.Errorf("expected null fallback for empty value\n----- got -----\n%s", got)
	}
}
