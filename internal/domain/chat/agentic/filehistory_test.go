package agentic_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- NewFileHistory ---

func TestNewFileHistory(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	require.NotNil(t, fh)
	assert.Equal(t, 0, fh.TotalSnapshots())
}

// --- Record ---

func TestFileHistory_Record_NewFile(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	created := fh.Record("main.go", "package main", 1)
	assert.True(t, created)
	assert.Equal(t, 1, fh.TotalSnapshots())
}

func TestFileHistory_Record_UnchangedContent(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "package main", 1)
	created := fh.Record("main.go", "package main", 2)
	assert.False(t, created, "should not create snapshot for unchanged content")
	assert.Equal(t, 1, fh.TotalSnapshots())
}

func TestFileHistory_Record_ChangedContent(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "package main", 1)
	created := fh.Record("main.go", "package main\nfunc main(){}", 2)
	assert.True(t, created)
	assert.Equal(t, 2, fh.TotalSnapshots())
}

func TestFileHistory_Record_MultipleFiles(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("a.go", "package a", 1)
	fh.Record("b.go", "package b", 1)
	assert.Equal(t, 2, fh.TotalSnapshots())
	assert.Equal(t, 2, len(fh.TrackedFiles()))
}

// --- GetLatest ---

func TestFileHistory_GetLatest(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "v1", 1)
	fh.Record("main.go", "v2", 2)
	fh.Record("main.go", "v3", 3)

	snap, ok := fh.GetLatest("main.go")
	require.True(t, ok)
	assert.Equal(t, "v3", snap.Content)
	assert.Equal(t, 3, snap.TurnIndex)
}

func TestFileHistory_GetLatest_NotFound(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	_, ok := fh.GetLatest("nonexistent.go")
	assert.False(t, ok)
}

// --- GetAtTurn ---

func TestFileHistory_GetAtTurn(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "v1", 1)
	fh.Record("main.go", "v2", 5)
	fh.Record("main.go", "v3", 10)

	snap, ok := fh.GetAtTurn("main.go", 7)
	require.True(t, ok)
	assert.Equal(t, "v2", snap.Content, "should return version at or before turn 7")
}

func TestFileHistory_GetAtTurn_ExactMatch(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "v1", 5)

	snap, ok := fh.GetAtTurn("main.go", 5)
	require.True(t, ok)
	assert.Equal(t, "v1", snap.Content)
}

func TestFileHistory_GetAtTurn_BeforeAny(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "v1", 5)

	_, ok := fh.GetAtTurn("main.go", 3)
	assert.False(t, ok)
}

// --- GetAll ---

func TestFileHistory_GetAll(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "v1", 1)
	fh.Record("main.go", "v2", 2)
	fh.Record("main.go", "v3", 3)

	all := fh.GetAll("main.go")
	require.Len(t, all, 3)
	assert.Equal(t, "v1", all[0].Content)
	assert.Equal(t, "v3", all[2].Content)
}

func TestFileHistory_GetAll_Empty(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	all := fh.GetAll("nope.go")
	assert.Empty(t, all)
}

func TestFileHistory_GetAll_ReturnsCopy(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "v1", 1)
	all := fh.GetAll("main.go")
	all[0].Content = "modified"

	original := fh.GetAll("main.go")
	assert.Equal(t, "v1", original[0].Content)
}

// --- VersionCount ---

func TestFileHistory_VersionCount(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	assert.Equal(t, 0, fh.VersionCount("main.go"))

	fh.Record("main.go", "v1", 1)
	fh.Record("main.go", "v2", 2)
	assert.Equal(t, 2, fh.VersionCount("main.go"))
}

// --- HasChanged ---

func TestFileHistory_HasChanged_NoSnapshot(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	assert.True(t, fh.HasChanged("new.go", "content"))
}

func TestFileHistory_HasChanged_Same(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "content", 1)
	assert.False(t, fh.HasChanged("main.go", "content"))
}

func TestFileHistory_HasChanged_Different(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "v1", 1)
	assert.True(t, fh.HasChanged("main.go", "v2"))
}

// --- MonotonicCounter ---

func TestFileHistory_MonotonicCounter(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	assert.Equal(t, 0, fh.MonotonicCounter())

	fh.Record("a.go", "v1", 1)
	assert.Equal(t, 1, fh.MonotonicCounter())

	fh.Record("a.go", "v1", 2) // unchanged, no new snapshot
	assert.Equal(t, 1, fh.MonotonicCounter())

	fh.Record("a.go", "v2", 3)
	assert.Equal(t, 2, fh.MonotonicCounter())
}

// --- Per-file eviction ---

func TestFileHistory_PerFileEviction(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{MaxSnapshots: 3})

	for i := 0; i < 5; i++ {
		fh.Record("main.go", fmt.Sprintf("v%d", i), i)
	}

	assert.Equal(t, 3, fh.VersionCount("main.go"))

	// Should keep the latest 3.
	all := fh.GetAll("main.go")
	assert.Equal(t, "v2", all[0].Content)
	assert.Equal(t, "v4", all[2].Content)
}

// --- Global eviction ---

func TestFileHistory_GlobalEviction(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{
		MaxSnapshots:      100,
		MaxTotalSnapshots: 5,
	})

	for i := 0; i < 10; i++ {
		fh.Record(fmt.Sprintf("file%d.go", i), fmt.Sprintf("content%d", i), i)
	}

	assert.Equal(t, 5, fh.TotalSnapshots())
}

// --- Clear ---

func TestFileHistory_Clear(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("a.go", "v1", 1)
	fh.Record("b.go", "v1", 1)

	fh.Clear()
	assert.Equal(t, 0, fh.TotalSnapshots())
	assert.Empty(t, fh.TrackedFiles())
}

// --- Snapshot metadata ---

func TestFileSnapshot_Size(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "hello world", 1) // 11 bytes

	snap, _ := fh.GetLatest("main.go")
	assert.Equal(t, 11, snap.Size)
}

func TestFileSnapshot_Hash(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "content", 1)

	snap, _ := fh.GetLatest("main.go")
	assert.NotEmpty(t, snap.ContentHash)
	assert.Len(t, snap.ContentHash, 32, "MD5 hex should be 32 chars")
}

func TestFileSnapshot_Timestamp(t *testing.T) {
	fh := agentic.NewFileHistory(agentic.FileHistoryConfig{})
	fh.Record("main.go", "content", 1)

	snap, _ := fh.GetLatest("main.go")
	assert.False(t, snap.Timestamp.IsZero())
}
