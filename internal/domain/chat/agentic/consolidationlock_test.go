package agentic_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func tempLockConfig(t *testing.T) agentic.ConsolidationLockConfig {
	t.Helper()
	dir := t.TempDir()
	return agentic.ConsolidationLockConfig{
		LockFilePath: filepath.Join(dir, ".consolidation.lock"),
		StaleTimeout: 60 * time.Minute,
	}
}

// --- TryAcquire ---

func TestConsolidationLock_TryAcquire(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)

	acq, err := l.TryAcquire()
	require.NoError(t, err)
	require.NotNil(t, acq)
	assert.Equal(t, os.Getpid(), acq.HolderPID)
}

func TestConsolidationLock_TryAcquire_SameProcess(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)

	// Same PID can re-acquire (it's us)
	acq1, err := l.TryAcquire()
	require.NoError(t, err)
	require.NotNil(t, acq1)

	acq2, err := l.TryAcquire()
	require.NoError(t, err)
	require.NotNil(t, acq2)
}

func TestConsolidationLock_TryAcquire_DeadHolder(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)

	// Write a dead PID to the lock file
	err := os.WriteFile(cfg.LockFilePath, []byte("99999999"), 0o644)
	require.NoError(t, err)

	acq, err := l.TryAcquire()
	require.NoError(t, err)
	require.NotNil(t, acq, "should reclaim from dead process")
	assert.Equal(t, os.Getpid(), acq.HolderPID)
}

func TestConsolidationLock_TryAcquire_UsesPrivatePermissions(t *testing.T) {
	root := t.TempDir()
	lockDir := filepath.Join(root, "state")
	cfg := agentic.ConsolidationLockConfig{
		LockFilePath: filepath.Join(lockDir, ".consolidation.lock"),
		StaleTimeout: 60 * time.Minute,
	}
	l := agentic.NewConsolidationLock(cfg)

	_, err := l.TryAcquire()
	require.NoError(t, err)

	dirInfo, err := os.Stat(lockDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm())

	fileInfo, err := os.Stat(cfg.LockFilePath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm())
}

// --- Release ---

func TestConsolidationLock_Release(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)

	_, err := l.TryAcquire()
	require.NoError(t, err)

	err = l.Release()
	require.NoError(t, err)

	_, statErr := os.Stat(cfg.LockFilePath)
	assert.True(t, os.IsNotExist(statErr), "lock file should be removed")
}

func TestConsolidationLock_Release_NotOurs(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)

	// Write someone else's PID
	err := os.WriteFile(cfg.LockFilePath, []byte("1"), 0o644)
	require.NoError(t, err)

	err = l.Release()
	assert.NoError(t, err) // no error, just no-op

	// File should still exist
	_, statErr := os.Stat(cfg.LockFilePath)
	assert.NoError(t, statErr, "should not delete someone else's lock")
}

func TestConsolidationLock_Release_AlreadyReleased(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)
	err := l.Release()
	assert.NoError(t, err)
}

// --- Rollback ---

func TestConsolidationLock_Rollback(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)

	_, err := l.TryAcquire()
	require.NoError(t, err)

	priorTime := time.Now().Add(-2 * time.Hour)
	err = l.Rollback(priorTime)
	require.NoError(t, err)

	info, err := os.Stat(cfg.LockFilePath)
	require.NoError(t, err)
	assert.WithinDuration(t, priorTime, info.ModTime(), time.Second)
}

func TestConsolidationLock_Rollback_ZeroTime(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)

	_, err := l.TryAcquire()
	require.NoError(t, err)

	err = l.Rollback(time.Time{})
	require.NoError(t, err)

	_, statErr := os.Stat(cfg.LockFilePath)
	assert.True(t, os.IsNotExist(statErr))
}

// --- ReadLastConsolidatedAt ---

func TestConsolidationLock_ReadLastConsolidatedAt_NoFile(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)

	ts, err := l.ReadLastConsolidatedAt()
	require.NoError(t, err)
	assert.True(t, ts.IsZero())
}

func TestConsolidationLock_ReadLastConsolidatedAt(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)

	_, err := l.TryAcquire()
	require.NoError(t, err)

	ts, err := l.ReadLastConsolidatedAt()
	require.NoError(t, err)
	assert.False(t, ts.IsZero())
	assert.WithinDuration(t, time.Now(), ts, 2*time.Second)
}

// --- ListFilesTouchedSince ---

func TestListFilesTouchedSince(t *testing.T) {
	dir := t.TempDir()

	// Create files with different mtimes
	oldFile := filepath.Join(dir, "old.txt")
	newFile := filepath.Join(dir, "new.txt")

	err := os.WriteFile(oldFile, []byte("old"), 0o644)
	require.NoError(t, err)
	// Set old mtime
	oldTime := time.Now().Add(-2 * time.Hour)
	err = os.Chtimes(oldFile, oldTime, oldTime)
	require.NoError(t, err)

	err = os.WriteFile(newFile, []byte("new"), 0o644)
	require.NoError(t, err)

	// List files touched since 1 hour ago
	since := time.Now().Add(-1 * time.Hour)
	files, err := agentic.ListFilesTouchedSince(dir, since)
	require.NoError(t, err)

	assert.Len(t, files, 1)
	assert.Equal(t, newFile, files[0])
}

func TestListFilesTouchedSince_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	files, err := agentic.ListFilesTouchedSince(dir, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestListFilesTouchedSince_SkipsDirs(t *testing.T) {
	dir := t.TempDir()
	err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0o644)
	require.NoError(t, err)

	files, err := agentic.ListFilesTouchedSince(dir, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

// --- DefaultConsolidationLockConfig ---

func TestDefaultConsolidationLockConfig(t *testing.T) {
	cfg := agentic.DefaultConsolidationLockConfig("/tmp/test")
	assert.Equal(t, "/tmp/test/.consolidation.lock", cfg.LockFilePath)
	assert.Equal(t, 60*time.Minute, cfg.StaleTimeout)
}

// --- PriorMtime preserved ---

func TestConsolidationLock_PriorMtime(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)

	// First acquire — no prior file
	acq1, err := l.TryAcquire()
	require.NoError(t, err)
	assert.True(t, acq1.PriorMtime.IsZero())

	// Second acquire — prior file exists
	acq2, err := l.TryAcquire()
	require.NoError(t, err)
	assert.False(t, acq2.PriorMtime.IsZero())
}

// --- Verify PID written ---

func TestConsolidationLock_VerifyPIDWritten(t *testing.T) {
	cfg := tempLockConfig(t)
	l := agentic.NewConsolidationLock(cfg)

	_, err := l.TryAcquire()
	require.NoError(t, err)

	content, err := os.ReadFile(cfg.LockFilePath)
	require.NoError(t, err)

	pid, err := strconv.Atoi(string(content))
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), pid)
}
