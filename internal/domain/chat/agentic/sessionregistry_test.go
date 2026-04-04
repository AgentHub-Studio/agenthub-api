package agentic_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewSessionRegistry ---

func TestNewSessionRegistry(t *testing.T) {
	dir := t.TempDir()
	r := agentic.NewSessionRegistry(dir, nil)
	assert.NotNil(t, r)
	assert.False(t, r.IsRegistered())
}

// --- Register ---

func TestSessionRegistry_Register(t *testing.T) {
	dir := t.TempDir()
	r := agentic.NewSessionRegistry(dir, nil)

	err := r.Register(agentic.SessionInfo{
		SessionID: "s1",
		Cwd:       "/tmp",
		Kind:      agentic.SessionKindInteractive,
	})
	require.NoError(t, err)
	assert.True(t, r.IsRegistered())

	// PID file should exist
	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 1)
	assert.Contains(t, entries[0].Name(), ".json")
}

// --- Update ---

func TestSessionRegistry_Update(t *testing.T) {
	dir := t.TempDir()
	r := agentic.NewSessionRegistry(dir, nil)

	err := r.Register(agentic.SessionInfo{
		SessionID: "s1",
		Kind:      agentic.SessionKindInteractive,
	})
	require.NoError(t, err)

	err = r.Update(map[string]interface{}{
		"status": "busy",
		"name":   "my-session",
	})
	require.NoError(t, err)

	sessions := r.List()
	require.Len(t, sessions, 1)
	assert.Equal(t, agentic.SessionStatusBusy, sessions[0].Status)
	assert.Equal(t, "my-session", sessions[0].Name)
}

// --- Unregister ---

func TestSessionRegistry_Unregister(t *testing.T) {
	dir := t.TempDir()
	r := agentic.NewSessionRegistry(dir, nil)

	err := r.Register(agentic.SessionInfo{SessionID: "s1"})
	require.NoError(t, err)

	err = r.Unregister()
	require.NoError(t, err)
	assert.False(t, r.IsRegistered())

	entries, _ := os.ReadDir(dir)
	assert.Len(t, entries, 0)
}

// --- Count ---

func TestSessionRegistry_Count(t *testing.T) {
	dir := t.TempDir()
	r := agentic.NewSessionRegistry(dir, nil)

	err := r.Register(agentic.SessionInfo{SessionID: "s1"})
	require.NoError(t, err)

	assert.Equal(t, 1, r.Count())
}

func TestSessionRegistry_Count_CleansStale(t *testing.T) {
	dir := t.TempDir()
	// All PIDs are "dead" except current process
	r := agentic.NewSessionRegistry(dir, func(pid int) bool {
		return pid == os.Getpid()
	})

	err := r.Register(agentic.SessionInfo{SessionID: "s1"})
	require.NoError(t, err)

	// Write a stale PID file for a "dead" process
	staleFile := filepath.Join(dir, "99999.json")
	_ = os.WriteFile(staleFile, []byte(`{"pid":99999,"sessionId":"stale"}`), 0o600)

	count := r.Count()
	assert.Equal(t, 1, count, "should only count live sessions")

	// Stale file should be removed
	_, err = os.Stat(staleFile)
	assert.True(t, os.IsNotExist(err))
}

func TestSessionRegistry_Count_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	r := agentic.NewSessionRegistry(dir, nil)
	assert.Equal(t, 0, r.Count())
}

func TestSessionRegistry_Count_NonexistentDir(t *testing.T) {
	r := agentic.NewSessionRegistry("/nonexistent/path", nil)
	assert.Equal(t, 0, r.Count())
}

// --- List ---

func TestSessionRegistry_List(t *testing.T) {
	dir := t.TempDir()
	r := agentic.NewSessionRegistry(dir, nil)

	err := r.Register(agentic.SessionInfo{
		SessionID: "s1",
		Kind:      agentic.SessionKindBackground,
		Cwd:       "/home/test",
	})
	require.NoError(t, err)

	sessions := r.List()
	require.Len(t, sessions, 1)
	assert.Equal(t, "s1", sessions[0].SessionID)
	assert.Equal(t, agentic.SessionKindBackground, sessions[0].Kind)
}

func TestSessionRegistry_List_IgnoresNonPIDFiles(t *testing.T) {
	dir := t.TempDir()
	r := agentic.NewSessionRegistry(dir, nil)

	err := r.Register(agentic.SessionInfo{SessionID: "s1"})
	require.NoError(t, err)

	// Write non-PID files that should be ignored
	_ = os.WriteFile(filepath.Join(dir, "notes.md"), []byte("hi"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "2026-03-14_backup.json"), []byte("{}"), 0o600)

	sessions := r.List()
	assert.Len(t, sessions, 1)
}

func TestSessionRegistry_List_SkipsInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	r := agentic.NewSessionRegistry(dir, func(pid int) bool { return true })

	// Write valid PID filename but invalid JSON
	_ = os.WriteFile(filepath.Join(dir, "12345.json"), []byte("not json"), 0o600)

	sessions := r.List()
	assert.Empty(t, sessions)
}

// --- Dir ---

func TestSessionRegistry_Dir(t *testing.T) {
	dir := t.TempDir()
	r := agentic.NewSessionRegistry(dir, nil)
	assert.Equal(t, dir, r.Dir())
}

// --- SessionKind values ---

func TestSessionKind_Values(t *testing.T) {
	assert.Equal(t, agentic.SessionKind("interactive"), agentic.SessionKindInteractive)
	assert.Equal(t, agentic.SessionKind("bg"), agentic.SessionKindBackground)
	assert.Equal(t, agentic.SessionKind("daemon"), agentic.SessionKindDaemon)
	assert.Equal(t, agentic.SessionKind("daemon-worker"), agentic.SessionKindWorker)
}

// --- SessionStatus values ---

func TestSessionStatus_Values(t *testing.T) {
	assert.Equal(t, agentic.SessionStatus("busy"), agentic.SessionStatusBusy)
	assert.Equal(t, agentic.SessionStatus("idle"), agentic.SessionStatusIdle)
	assert.Equal(t, agentic.SessionStatus("waiting"), agentic.SessionStatusWaiting)
}
