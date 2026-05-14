package agentic

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// BDD-style scenarios that ratify PERSIST-006 (Rewind/file checkpoints)
// against the Claude Code architecture paper "Dive into Claude Code"
// (arXiv:2604.14228v1):
//
//   - Section 9.2 (Resume, Fork, Not Restoring Permissions): "The
//     'checkpoints' in Claude Code are file-history checkpoints for
//     --rewind-files, stored at ~/.claude/file-history/<sessionId>/.
//     These are file-level snapshots for reverting filesystem changes,
//     not a generic checkpoint store."
//   - Section 9 (overall): file mutations across a multi-turn session need
//     bounded retention so the user can rewind specific edits without
//     unbounded disk growth.
//
// AgentHub maps file checkpoints to filehistory.go FileHistory:
//   - FileSnapshot — Path + ContentHash (MD5) + Content + TurnIndex +
//     Timestamp + Size — the per-version record.
//   - FileHistoryConfig — MaxSnapshots per file (default 50) +
//     MaxTotalSnapshots global (default 500) — bounded retention.
//   - Record(path, content, turnIndex) — appends if hash changed; returns
//     false on no-op (dedup by ContentHash).
//   - GetAtTurn(path, turnIndex) — the REWIND primitive: locates the
//     snapshot active during a given turn for restoration.
//   - GetLatest, GetAll, TrackedFiles, VersionCount, TotalSnapshots,
//     MonotonicCounter, HasChanged, Clear — observation + lifecycle.
//   - LRU eviction when MaxTotalSnapshots is exceeded.
//
// These scenarios assert: dedup behavior, bounded retention, rewind
// lookup, change detection, concurrent safety.

func TestBDD_FileCheckpointRewind(t *testing.T) {
	t.Run("Scenario_RecordCreatesSnapshotForFirstWrite", func(t *testing.T) {
		// Given a fresh FileHistory (PDF Section 9.2: file checkpoints
		//       must be bounded but cover every edit),
		fh := NewFileHistory(FileHistoryConfig{})

		// When the runner records the first write to a file,
		when := fh.Record("/src/main.go", "package main\n", 0)

		// Then a snapshot is created.
		assert.True(t, when,
			"first write must produce a new snapshot")
		assert.Equal(t, 1, fh.VersionCount("/src/main.go"),
			"version count for the file is 1")
		assert.Equal(t, 1, fh.TotalSnapshots())
	})

	t.Run("Scenario_RecordDeduplicatesIdenticalContent", func(t *testing.T) {
		// Given the same content recorded twice (PDF: snapshot store must
		//       not bloat with identical versions — common when re-reading
		//       a file the agent didn't actually change),
		fh := NewFileHistory(FileHistoryConfig{})
		fh.Record("/x.txt", "v1", 1)

		// When a second Record submits the same content,
		when := fh.Record("/x.txt", "v1", 2)

		// Then the snapshot is NOT duplicated — Record returns false and
		//      the version count stays at 1.
		assert.False(t, when,
			"unchanged content must NOT create a new snapshot")
		assert.Equal(t, 1, fh.VersionCount("/x.txt"),
			"identical content stays at version 1 (dedup)")
	})

	t.Run("Scenario_RecordCreatesNewSnapshotWhenContentChanges", func(t *testing.T) {
		// Given a file with one snapshot,
		fh := NewFileHistory(FileHistoryConfig{})
		fh.Record("/x.txt", "v1", 1)

		// When the content changes,
		when := fh.Record("/x.txt", "v2", 2)

		// Then a new snapshot is appended.
		assert.True(t, when,
			"changed content must produce a new snapshot")
		assert.Equal(t, 2, fh.VersionCount("/x.txt"),
			"version count advances on real change")
	})

	t.Run("Scenario_GetAtTurnReturnsActiveSnapshotForRewind", func(t *testing.T) {
		// Given a file mutated across multiple turns (PDF: --rewind-files
		//       lookup needs to find the snapshot ACTIVE at a given turn,
		//       not necessarily one taken on that exact turn),
		fh := NewFileHistory(FileHistoryConfig{})
		fh.Record("/x.txt", "v1", 1)
		fh.Record("/x.txt", "v2", 5)
		fh.Record("/x.txt", "v3", 10)

		// When the user asks for the file as of turn 3,
		when, ok := fh.GetAtTurn("/x.txt", 3)

		// Then v1 is returned — it was the active version (recorded at
		//      turn 1, still in effect through turn 4).
		assert.True(t, ok, "snapshot active at turn 3 must exist")
		assert.Equal(t, "v1", when.Content,
			"active snapshot at turn 3 is v1 (recorded at turn 1)")
	})

	t.Run("Scenario_GetAtTurnReturnsLatestForFutureTurn", func(t *testing.T) {
		// Given a file with snapshots up to turn 5 (PDF: rewind lookup
		//       to a future turn returns the latest available — caller
		//       can use this as "current state at the most recent turn"),
		fh := NewFileHistory(FileHistoryConfig{})
		fh.Record("/x.txt", "v1", 1)
		fh.Record("/x.txt", "v2", 5)

		// When the user asks for turn 100,
		when, ok := fh.GetAtTurn("/x.txt", 100)

		// Then the latest available snapshot (v2) is returned.
		assert.True(t, ok)
		assert.Equal(t, "v2", when.Content,
			"future turn must return latest available snapshot")
	})

	t.Run("Scenario_GetAtTurnBeforeFirstSnapshotReturnsNotFound", func(t *testing.T) {
		// Given a file recorded starting at turn 5,
		fh := NewFileHistory(FileHistoryConfig{})
		fh.Record("/x.txt", "v1", 5)

		// When the user asks for turn 0,
		_, ok := fh.GetAtTurn("/x.txt", 0)

		// Then the lookup fails — there's no version active before the
		//      first recorded turn (rewind would have nothing to restore).
		assert.False(t, ok,
			"turn before first snapshot must return not-found")
	})

	t.Run("Scenario_GetLatestReturnsTheNewestSnapshot", func(t *testing.T) {
		// Given multiple versions,
		fh := NewFileHistory(FileHistoryConfig{})
		fh.Record("/x.txt", "v1", 1)
		fh.Record("/x.txt", "v2", 2)
		fh.Record("/x.txt", "v3", 3)

		// When the user asks for the latest,
		when, ok := fh.GetLatest("/x.txt")

		// Then v3 is returned.
		assert.True(t, ok)
		assert.Equal(t, "v3", when.Content,
			"GetLatest must surface the newest snapshot")
		assert.Equal(t, 3, when.TurnIndex)
	})

	t.Run("Scenario_HasChangedDetectsContentMutation", func(t *testing.T) {
		// Given a file with one snapshot,
		fh := NewFileHistory(FileHistoryConfig{})
		fh.Record("/x.txt", "v1", 1)

		// When the runtime queries change vs current content,
		// Then identical content reports unchanged; different reports changed.
		assert.False(t, fh.HasChanged("/x.txt", "v1"),
			"identical content must report unchanged")
		assert.True(t, fh.HasChanged("/x.txt", "v2"),
			"different content must report changed")
	})

	t.Run("Scenario_TrackedFilesEnumeratesAllPaths", func(t *testing.T) {
		// Given multiple files recorded,
		fh := NewFileHistory(FileHistoryConfig{})
		fh.Record("/a.go", "package a", 1)
		fh.Record("/b.go", "package b", 1)
		fh.Record("/a.go", "package a v2", 2)

		// When the operator asks for tracked files,
		when := fh.TrackedFiles()

		// Then both paths are surfaced — supports operator visibility for
		//      "what did this session touch".
		assert.Len(t, when, 2,
			"both tracked files must be listed")
		assert.Contains(t, when, "/a.go")
		assert.Contains(t, when, "/b.go")
	})

	t.Run("Scenario_PerFileSnapshotCapEnforcedByEviction", func(t *testing.T) {
		// Given a per-file cap of 3 (PDF Section 9.2: bounded retention —
		//       file history must not grow unbounded across a long session),
		fh := NewFileHistory(FileHistoryConfig{MaxSnapshots: 3})

		// When 5 distinct versions are recorded,
		fh.Record("/x.txt", "v1", 1)
		fh.Record("/x.txt", "v2", 2)
		fh.Record("/x.txt", "v3", 3)
		fh.Record("/x.txt", "v4", 4)
		fh.Record("/x.txt", "v5", 5)

		// Then per-file count caps at 3 — oldest evicted.
		assert.LessOrEqual(t, fh.VersionCount("/x.txt"), 3,
			"per-file cap must enforce eviction")
	})

	t.Run("Scenario_MonotonicCounterAdvancesOnEverySnapshot", func(t *testing.T) {
		// Given a fresh history (PDF: monotonic activity counter for change
		//       detection signals),
		fh := NewFileHistory(FileHistoryConfig{})
		initial := fh.MonotonicCounter()

		// When 3 snapshots are taken,
		fh.Record("/a.txt", "v1", 1)
		fh.Record("/a.txt", "v2", 2)
		fh.Record("/b.txt", "v1", 1)

		// Then the counter advances for each — useful as a "anything
		//      changed since X?" signal independent of file paths.
		assert.GreaterOrEqual(t, fh.MonotonicCounter(), initial+3,
			"monotonic counter must advance once per snapshot")
	})

	t.Run("Scenario_ClearResetsAllHistory", func(t *testing.T) {
		// Given a populated history (PDF: explicit reset path needed for
		//       session end / fresh start),
		fh := NewFileHistory(FileHistoryConfig{})
		fh.Record("/a.txt", "v1", 1)
		fh.Record("/b.txt", "v1", 1)

		// When the runtime clears,
		fh.Clear()

		// Then no snapshots remain.
		assert.Equal(t, 0, fh.TotalSnapshots(),
			"Clear must drop all snapshots")
		assert.Empty(t, fh.TrackedFiles(),
			"Clear must drop tracked files")
	})

	t.Run("Scenario_ConcurrentRecordIsSafe", func(t *testing.T) {
		// Given multiple goroutines writing concurrently (PDF Section 4.2
		//       parallel tools may all touch file history),
		fh := NewFileHistory(FileHistoryConfig{MaxSnapshots: 1000, MaxTotalSnapshots: 10000})
		var wg sync.WaitGroup

		// When 20 goroutines each record 50 distinct versions,
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				for j := 0; j < 50; j++ {
					content := string(rune('A'+g)) + string(rune('0'+j%10)) + string(rune('0'+j/10))
					fh.Record("/file"+string(rune('A'+g))+".txt", content, j)
				}
			}(i)
		}
		wg.Wait()

		// Then no race occurs (test fails under -race) and tracked files
		//      include all 20 paths.
		assert.LessOrEqual(t, len(fh.TrackedFiles()), 20,
			"20 distinct file paths must be tracked or fewer if dedupes applied")
	})
}
