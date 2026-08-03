package agentic_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Config ---

func TestDefaultSessionIngressConfig(t *testing.T) {
	cfg := agentic.DefaultSessionIngressConfig()
	assert.Equal(t, 10, cfg.MaxRetries)
	assert.Equal(t, 500, cfg.BaseDelayMs)
	assert.Equal(t, 8000, cfg.MaxDelayMs)
	assert.Equal(t, 100, cfg.FlushIntervalMs)
}

func TestRemoteSessionIngressConfig(t *testing.T) {
	cfg := agentic.RemoteSessionIngressConfig()
	assert.Equal(t, 10, cfg.FlushIntervalMs)
}

// --- TranscriptEntry ---

func TestNewTranscriptEntry(t *testing.T) {
	msg, _ := json.Marshal(map[string]string{"content": "hello"})
	parent := "parent-uuid"
	entry := agentic.NewTranscriptEntry("user", msg, &parent)
	assert.NotEmpty(t, entry.UUID)
	assert.Equal(t, "user", entry.Type)
	assert.Equal(t, &parent, entry.ParentUUID)
	assert.Greater(t, entry.Timestamp, int64(0))
}

func TestNewTranscriptEntry_NilParent(t *testing.T) {
	entry := agentic.NewTranscriptEntry("assistant", nil, nil)
	assert.Nil(t, entry.ParentUUID)
}

// --- SessionIngress ---

func TestSessionIngress_AppendLog_Success(t *testing.T) {
	var received []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = readBody(r)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.SessionIngressConfig{
		MaxRetries: 3, BaseDelayMs: 10, MaxDelayMs: 50,
	})

	entry := agentic.NewTranscriptEntry("user", json.RawMessage(`{"content":"hi"}`), nil)
	ok, err := si.AppendLog(context.Background(), "sess-1", entry, server.URL, nil)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.NotEmpty(t, received)
	assert.Equal(t, entry.UUID, si.GetLastUUID("sess-1"))
}

func TestSessionIngress_AppendLog_SendsLastUUID(t *testing.T) {
	var lastUUIDHeader string
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		lastUUIDHeader = r.Header.Get("Last-Uuid")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.SessionIngressConfig{
		MaxRetries: 3, BaseDelayMs: 10, MaxDelayMs: 50,
	})

	entry1 := agentic.NewTranscriptEntry("user", json.RawMessage(`{"content":"msg1"}`), nil)
	ok, err := si.AppendLog(context.Background(), "sess-1", entry1, server.URL, nil)
	require.True(t, ok)
	require.NoError(t, err)

	entry2 := agentic.NewTranscriptEntry("assistant", json.RawMessage(`{"content":"msg2"}`), &entry1.UUID)
	ok, err = si.AppendLog(context.Background(), "sess-1", entry2, server.URL, nil)
	require.True(t, ok)
	require.NoError(t, err)

	assert.Equal(t, 2, callCount)
	assert.Equal(t, entry1.UUID, lastUUIDHeader, "second request should send first entry's UUID")
}

func TestSessionIngress_AppendLog_AuthFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		if _, err := fmt.Fprint(w, "invalid token"); err != nil {
			return
		}
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.SessionIngressConfig{
		MaxRetries: 3, BaseDelayMs: 10, MaxDelayMs: 50,
	})

	entry := agentic.NewTranscriptEntry("user", nil, nil)
	ok, err := si.AppendLog(context.Background(), "sess-1", entry, server.URL, nil)

	assert.Error(t, err, "should return error on 401")
	assert.False(t, ok)
}

func TestSessionIngress_AppendLog_RetryOn500(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.SessionIngressConfig{
		MaxRetries: 5, BaseDelayMs: 10, MaxDelayMs: 50,
	})

	entry := agentic.NewTranscriptEntry("user", nil, nil)
	ok, err := si.AppendLog(context.Background(), "sess-1", entry, server.URL, nil)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, int32(3), atomic.LoadInt32(&attempts))
}

func TestSessionIngress_AppendLog_RetriesExhausted(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.SessionIngressConfig{
		MaxRetries: 3, BaseDelayMs: 10, MaxDelayMs: 50,
	})

	entry := agentic.NewTranscriptEntry("user", nil, nil)
	ok, err := si.AppendLog(context.Background(), "sess-1", entry, server.URL, nil)

	assert.NoError(t, err, "exhausted retries is not an error")
	assert.False(t, ok)
}

// --- 409 Conflict Resolution ---

func TestSessionIngress_409_Idempotent(t *testing.T) {
	entry := agentic.NewTranscriptEntry("user", nil, nil)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate: entry already persisted, server returns 409 with matching UUID.
		w.Header().Set("X-Last-Uuid", entry.UUID)
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.SessionIngressConfig{
		MaxRetries: 3, BaseDelayMs: 10, MaxDelayMs: 50,
	})

	ok, err := si.AppendLog(context.Background(), "sess-1", entry, server.URL, nil)
	require.NoError(t, err)
	assert.True(t, ok, "should succeed because server already has our entry")
	assert.Equal(t, entry.UUID, si.GetLastUUID("sess-1"))
}

func TestSessionIngress_409_AdoptServerUUID(t *testing.T) {
	serverUUID := "server-advanced-uuid"
	callCount := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount == 1 {
			// First attempt: conflict with different UUID.
			w.Header().Set("X-Last-Uuid", serverUUID)
			w.WriteHeader(http.StatusConflict)
			return
		}
		// Second attempt: should have adopted server UUID.
		assert.Equal(t, serverUUID, r.Header.Get("Last-Uuid"))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.SessionIngressConfig{
		MaxRetries: 5, BaseDelayMs: 10, MaxDelayMs: 50,
	})

	entry := agentic.NewTranscriptEntry("user", nil, nil)
	ok, err := si.AppendLog(context.Background(), "sess-1", entry, server.URL, nil)

	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, 2, callCount)
}

// --- Context cancellation ---

func TestSessionIngress_ContextCancelled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError) // force retry
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.SessionIngressConfig{
		MaxRetries: 10, BaseDelayMs: 1000, MaxDelayMs: 5000,
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	entry := agentic.NewTranscriptEntry("user", nil, nil)
	ok, err := si.AppendLog(ctx, "sess-1", entry, server.URL, nil)

	assert.Error(t, err)
	assert.False(t, ok)
}

// --- Sequential ordering ---

func TestSessionIngress_SequentialOrdering(t *testing.T) {
	var mu sync.Mutex
	var receivedOrder []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var entry agentic.TranscriptEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		receivedOrder = append(receivedOrder, entry.UUID)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.SessionIngressConfig{
		MaxRetries: 3, BaseDelayMs: 10, MaxDelayMs: 50,
	})

	// Send 5 entries concurrently for the same session.
	entries := make([]agentic.TranscriptEntry, 5)
	for i := 0; i < 5; i++ {
		entries[i] = agentic.NewTranscriptEntry("user", json.RawMessage(fmt.Sprintf(`{"seq":%d}`, i)), nil)
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(entries))
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ok, err := si.AppendLog(context.Background(), "sess-ordered", entries[idx], server.URL, nil)
			if !ok && err == nil {
				err = fmt.Errorf("append log did not persist entry")
			}
			errCh <- err
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, receivedOrder, 5, "all 5 entries should be persisted")
}

// --- Multi-session isolation ---

func TestSessionIngress_MultiSessionIsolation(t *testing.T) {
	var mu sync.Mutex
	sessionHits := make(map[string]int)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var entry agentic.TranscriptEntry
		if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		sessionHits[entry.AgentID]++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.SessionIngressConfig{
		MaxRetries: 3, BaseDelayMs: 10, MaxDelayMs: 50,
	})

	var wg sync.WaitGroup
	errCh := make(chan error, 6)
	for _, sessID := range []string{"sess-A", "sess-B"} {
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(sid string) {
				defer wg.Done()
				entry := agentic.NewTranscriptEntry("user", nil, nil)
				entry.AgentID = sid
				ok, err := si.AppendLog(context.Background(), sid, entry, server.URL, nil)
				if !ok && err == nil {
					err = fmt.Errorf("append log did not persist entry")
				}
				errCh <- err
			}(sessID)
		}
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 3, sessionHits["sess-A"])
	assert.Equal(t, 3, sessionHits["sess-B"])
}

// --- HydrateSession ---

func TestSessionIngress_HydrateSession(t *testing.T) {
	entries := []agentic.TranscriptEntry{
		{UUID: "uuid-1", Type: "user", Timestamp: 1000},
		{UUID: "uuid-2", Type: "assistant", Timestamp: 2000},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode(entries); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.DefaultSessionIngressConfig())
	result, err := si.HydrateSession(context.Background(), "sess-1", server.URL, nil)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "uuid-2", si.GetLastUUID("sess-1"))
}

func TestSessionIngress_HydrateSession_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewEncoder(w).Encode([]agentic.TranscriptEntry{}); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.DefaultSessionIngressConfig())
	result, err := si.HydrateSession(context.Background(), "sess-1", server.URL, nil)

	require.NoError(t, err)
	assert.Empty(t, result)
	assert.Empty(t, si.GetLastUUID("sess-1"))
}

func TestSessionIngress_HydrateSession_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		if _, err := fmt.Fprint(w, "internal error"); err != nil {
			return
		}
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.DefaultSessionIngressConfig())
	_, err := si.HydrateSession(context.Background(), "sess-1", server.URL, nil)
	assert.Error(t, err)
}

// --- CleanupSession ---

func TestSessionIngress_CleanupSession(t *testing.T) {
	si := agentic.NewSessionIngress(agentic.DefaultSessionIngressConfig())

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	entry := agentic.NewTranscriptEntry("user", nil, nil)
	ok, err := si.AppendLog(context.Background(), "sess-cleanup", entry, server.URL, nil)
	require.True(t, ok)
	require.NoError(t, err)
	assert.Equal(t, 1, si.ActiveSessions())

	si.CleanupSession("sess-cleanup")
	assert.Equal(t, 0, si.ActiveSessions())
	assert.Empty(t, si.GetLastUUID("sess-cleanup"))
}

// --- Custom headers ---

func TestSessionIngress_CustomHeaders(t *testing.T) {
	var receivedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	si := agentic.NewSessionIngress(agentic.SessionIngressConfig{
		MaxRetries: 1, BaseDelayMs: 10, MaxDelayMs: 50,
	})

	entry := agentic.NewTranscriptEntry("user", nil, nil)
	headers := map[string]string{"Authorization": "Bearer test-token"}
	ok, err := si.AppendLog(context.Background(), "sess-1", entry, server.URL, headers)
	require.True(t, ok)
	require.NoError(t, err)

	assert.Equal(t, "Bearer test-token", receivedAuth)
}

// helper
func readBody(r *http.Request) ([]byte, error) {
	defer func() { _ = r.Body.Close() }()
	return io.ReadAll(r.Body)
}
