package agentic

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	consolidationLockDirMode  = 0o700
	consolidationLockFileMode = 0o600
)

// Process-coordination lock for consolidation tasks.
//
// Inspired by Claude Code's consolidationLock.ts — implements lock-file-based
// inter-process coordination for memory/session consolidation. Uses file mtime
// as last-consolidation timestamp, PID-based stale holder detection, and
// atomic acquire→verify sequences.

// ConsolidationLockConfig configures the lock behavior.
type ConsolidationLockConfig struct {
	// LockFilePath is the path to the lock file.
	LockFilePath string
	// StaleTimeout is how long a lock holder can keep the lock
	// before it's considered stale and forcibly reclaimed.
	StaleTimeout time.Duration
}

// DefaultConsolidationLockConfig returns sensible defaults.
func DefaultConsolidationLockConfig(dir string) ConsolidationLockConfig {
	return ConsolidationLockConfig{
		LockFilePath: filepath.Join(dir, ".consolidation.lock"),
		StaleTimeout: 60 * time.Minute,
	}
}

// ConsolidationLock manages a file-based inter-process lock.
type ConsolidationLock struct {
	mu     sync.Mutex
	config ConsolidationLockConfig
}

// NewConsolidationLock creates a new lock instance.
func NewConsolidationLock(config ConsolidationLockConfig) *ConsolidationLock {
	if config.StaleTimeout <= 0 {
		config.StaleTimeout = 60 * time.Minute
	}
	return &ConsolidationLock{config: config}
}

// LockAcquisition represents a successfully acquired lock.
type LockAcquisition struct {
	// PriorMtime is the mtime before acquisition, used for rollback.
	PriorMtime time.Time
	// HolderPID is the PID that acquired the lock.
	HolderPID int
}

// TryAcquire attempts to acquire the consolidation lock.
// Returns a LockAcquisition on success, or nil if the lock is held by
// another active process.
func (l *ConsolidationLock) TryAcquire() (*LockAcquisition, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	pid := os.Getpid()

	// Read existing lock
	info, err := os.Stat(l.config.LockFilePath)
	if err == nil {
		// Lock file exists — check if holder is still alive
		content, readErr := os.ReadFile(l.config.LockFilePath)
		if readErr == nil {
			holderPID, parseErr := strconv.Atoi(strings.TrimSpace(string(content)))
			if parseErr == nil && holderPID != pid {
				// Check if holder process is alive
				if isProcessAlive(holderPID) {
					// Check if lock is stale (held too long)
					if time.Since(info.ModTime()) < l.config.StaleTimeout {
						return nil, nil // lock held by active process
					}
					// Stale — forcibly reclaim below
				}
				// Holder is dead or stale — reclaim
			}
		}
	}

	// Record prior mtime for rollback
	var priorMtime time.Time
	if info != nil {
		priorMtime = info.ModTime()
	}

	// Write our PID to the lock file
	dir := filepath.Dir(l.config.LockFilePath)
	if err := os.MkdirAll(dir, consolidationLockDirMode); err != nil {
		return nil, fmt.Errorf("create lock dir: %w", err)
	}

	if err := os.WriteFile(l.config.LockFilePath, []byte(strconv.Itoa(pid)), consolidationLockFileMode); err != nil {
		return nil, fmt.Errorf("write lock file: %w", err)
	}

	// Verify we still hold it (race detection: re-read)
	content, err := os.ReadFile(l.config.LockFilePath)
	if err != nil {
		return nil, fmt.Errorf("verify lock: %w", err)
	}

	writtenPID, _ := strconv.Atoi(strings.TrimSpace(string(content)))
	if writtenPID != pid {
		return nil, nil // lost race
	}

	return &LockAcquisition{
		PriorMtime: priorMtime,
		HolderPID:  pid,
	}, nil
}

// Release removes the lock if we still hold it.
func (l *ConsolidationLock) Release() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	pid := os.Getpid()

	content, err := os.ReadFile(l.config.LockFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // already released
		}
		return fmt.Errorf("read lock for release: %w", err)
	}

	holderPID, _ := strconv.Atoi(strings.TrimSpace(string(content)))
	if holderPID != pid {
		return nil // not our lock
	}

	return os.Remove(l.config.LockFilePath)
}

// Rollback restores the lock file mtime to the prior value,
// used when a consolidation operation fails and we want the
// next run to pick up where we left off.
func (l *ConsolidationLock) Rollback(priorMtime time.Time) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if priorMtime.IsZero() {
		// No prior mtime — just remove
		return os.Remove(l.config.LockFilePath)
	}

	return os.Chtimes(l.config.LockFilePath, priorMtime, priorMtime)
}

// ReadLastConsolidatedAt returns the mtime of the lock file,
// which represents when consolidation last completed.
func (l *ConsolidationLock) ReadLastConsolidatedAt() (time.Time, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	info, err := os.Stat(l.config.LockFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, nil // never consolidated
		}
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// ListFilesTouchedSince returns files in dir modified after the given time.
func ListFilesTouchedSince(dir string, since time.Time) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var result []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if info.ModTime().After(since) {
			result = append(result, filepath.Join(dir, entry.Name()))
		}
	}
	return result, nil
}

// isProcessAlive checks if a process with the given PID exists.
// On Unix, signal 0 checks process existence without actually sending a signal.
func isProcessAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	// nil means process exists and we have permission.
	// EPERM means process exists but we lack permission (still alive).
	return err == nil || errors.Is(err, syscall.EPERM)
}
