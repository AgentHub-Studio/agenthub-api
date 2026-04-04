package agentic

import (
	"crypto/md5"
	"encoding/hex"
	"sync"
	"time"
)

// Versioned file snapshot tracking.
//
// Inspired by Claude Code's fileHistory.ts — maintains immutable snapshots
// of file contents across session turns. Uses hash-based deduplication
// to avoid storing identical versions. Supports LRU-like eviction
// when the snapshot cap is reached.

// FileSnapshot represents a single version of a file's content.
type FileSnapshot struct {
	// Path is the file path.
	Path string `json:"path"`
	// ContentHash is the MD5 hash of the content.
	ContentHash string `json:"contentHash"`
	// Content is the full file content (may be empty if evicted).
	Content string `json:"content,omitempty"`
	// TurnIndex is the turn during which this snapshot was taken.
	TurnIndex int `json:"turnIndex"`
	// Timestamp is when the snapshot was created.
	Timestamp time.Time `json:"timestamp"`
	// Size is the byte size of the content.
	Size int `json:"size"`
}

// FileHistoryConfig configures the file history tracker.
type FileHistoryConfig struct {
	// MaxSnapshots is the maximum number of snapshots to retain per file.
	// Default: 50.
	MaxSnapshots int
	// MaxTotalSnapshots is the global cap across all files.
	// Default: 500.
	MaxTotalSnapshots int
}

// FileHistory tracks versioned snapshots of files across turns.
type FileHistory struct {
	mu     sync.RWMutex
	config FileHistoryConfig

	// snapshots is keyed by file path, ordered oldest-first.
	snapshots map[string][]FileSnapshot

	// hashIndex tracks which content hashes we've already stored
	// to avoid duplicate content storage.
	hashIndex map[string]bool

	// totalCount tracks the total number of snapshots across all files.
	totalCount int

	// monotonicCounter increments on every snapshot for activity signals.
	monotonicCounter int
}

// NewFileHistory creates a file history tracker.
func NewFileHistory(config FileHistoryConfig) *FileHistory {
	if config.MaxSnapshots <= 0 {
		config.MaxSnapshots = 50
	}
	if config.MaxTotalSnapshots <= 0 {
		config.MaxTotalSnapshots = 500
	}
	return &FileHistory{
		config:    config,
		snapshots: make(map[string][]FileSnapshot),
		hashIndex: make(map[string]bool),
	}
}

// Record stores a snapshot of a file's content at the given turn.
// Returns true if a new snapshot was created, false if content was unchanged.
func (fh *FileHistory) Record(path, content string, turnIndex int) bool {
	hash := hashMD5(content)

	fh.mu.Lock()
	defer fh.mu.Unlock()

	// Check if the latest snapshot for this file has the same hash.
	existing := fh.snapshots[path]
	if len(existing) > 0 {
		latest := existing[len(existing)-1]
		if latest.ContentHash == hash {
			return false // unchanged
		}
	}

	snap := FileSnapshot{
		Path:        path,
		ContentHash: hash,
		Content:     content,
		TurnIndex:   turnIndex,
		Timestamp:   time.Now(),
		Size:        len(content),
	}

	fh.snapshots[path] = append(fh.snapshots[path], snap)
	fh.hashIndex[hash] = true
	fh.totalCount++
	fh.monotonicCounter++

	// Evict if per-file cap exceeded.
	if len(fh.snapshots[path]) > fh.config.MaxSnapshots {
		fh.snapshots[path] = fh.snapshots[path][1:]
		fh.totalCount--
	}

	// Evict globally if total cap exceeded (remove oldest across all files).
	for fh.totalCount > fh.config.MaxTotalSnapshots {
		fh.evictOldest()
	}

	return true
}

// GetLatest returns the most recent snapshot for a file.
func (fh *FileHistory) GetLatest(path string) (FileSnapshot, bool) {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	snaps := fh.snapshots[path]
	if len(snaps) == 0 {
		return FileSnapshot{}, false
	}
	return snaps[len(snaps)-1], true
}

// GetAtTurn returns the snapshot closest to (but not after) the given turn.
func (fh *FileHistory) GetAtTurn(path string, turnIndex int) (FileSnapshot, bool) {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	snaps := fh.snapshots[path]
	var best FileSnapshot
	found := false
	for _, s := range snaps {
		if s.TurnIndex <= turnIndex {
			best = s
			found = true
		}
	}
	return best, found
}

// GetAll returns all snapshots for a file, oldest first.
func (fh *FileHistory) GetAll(path string) []FileSnapshot {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	snaps := fh.snapshots[path]
	result := make([]FileSnapshot, len(snaps))
	copy(result, snaps)
	return result
}

// TrackedFiles returns all file paths that have snapshots.
func (fh *FileHistory) TrackedFiles() []string {
	fh.mu.RLock()
	defer fh.mu.RUnlock()

	paths := make([]string, 0, len(fh.snapshots))
	for p := range fh.snapshots {
		paths = append(paths, p)
	}
	return paths
}

// VersionCount returns the number of snapshots for a file.
func (fh *FileHistory) VersionCount(path string) int {
	fh.mu.RLock()
	defer fh.mu.RUnlock()
	return len(fh.snapshots[path])
}

// TotalSnapshots returns the total number of snapshots across all files.
func (fh *FileHistory) TotalSnapshots() int {
	fh.mu.RLock()
	defer fh.mu.RUnlock()
	return fh.totalCount
}

// MonotonicCounter returns the ever-increasing counter value.
// Useful for detecting any history activity even after eviction.
func (fh *FileHistory) MonotonicCounter() int {
	fh.mu.RLock()
	defer fh.mu.RUnlock()
	return fh.monotonicCounter
}

// HasChanged returns true if the file's current content differs from the latest snapshot.
func (fh *FileHistory) HasChanged(path, currentContent string) bool {
	hash := hashMD5(currentContent)

	fh.mu.RLock()
	defer fh.mu.RUnlock()

	snaps := fh.snapshots[path]
	if len(snaps) == 0 {
		return true // no snapshot = changed
	}
	return snaps[len(snaps)-1].ContentHash != hash
}

// Clear removes all snapshots.
func (fh *FileHistory) Clear() {
	fh.mu.Lock()
	defer fh.mu.Unlock()

	fh.snapshots = make(map[string][]FileSnapshot)
	fh.hashIndex = make(map[string]bool)
	fh.totalCount = 0
}

// evictOldest removes the single oldest snapshot across all files.
// Must be called with mu held.
func (fh *FileHistory) evictOldest() {
	var oldestPath string
	var oldestTime time.Time

	for path, snaps := range fh.snapshots {
		if len(snaps) > 0 {
			if oldestPath == "" || snaps[0].Timestamp.Before(oldestTime) {
				oldestPath = path
				oldestTime = snaps[0].Timestamp
			}
		}
	}

	if oldestPath != "" {
		fh.snapshots[oldestPath] = fh.snapshots[oldestPath][1:]
		fh.totalCount--
		if len(fh.snapshots[oldestPath]) == 0 {
			delete(fh.snapshots, oldestPath)
		}
	}
}

// hashMD5 computes the MD5 hex digest of content.
func hashMD5(content string) string {
	h := md5.Sum([]byte(content))
	return hex.EncodeToString(h[:])
}
