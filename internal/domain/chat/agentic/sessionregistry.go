package agentic

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PID-based session registry for tracking concurrent processes.
//
// Inspired by Claude Code's concurrentSessions — tracks active sessions
// through PID files in a shared directory. Supports registration,
// lifecycle tracking, activity state updates, and automatic cleanup
// of stale entries from crashed processes.

// SessionKind identifies how a session was started.
type SessionKind string

const (
	SessionKindInteractive SessionKind = "interactive"
	SessionKindBackground  SessionKind = "bg"
	SessionKindDaemon      SessionKind = "daemon"
	SessionKindWorker      SessionKind = "daemon-worker"
)

// SessionStatus is the current activity state.
type SessionStatus string

const (
	SessionStatusBusy    SessionStatus = "busy"
	SessionStatusIdle    SessionStatus = "idle"
	SessionStatusWaiting SessionStatus = "waiting"
)

// SessionInfo is the data stored in a session's PID file.
type SessionInfo struct {
	PID        int           `json:"pid"`
	SessionID  string        `json:"sessionId"`
	Cwd        string        `json:"cwd"`
	StartedAt  int64         `json:"startedAt"`
	Kind       SessionKind   `json:"kind"`
	Name       string        `json:"name,omitempty"`
	Status     SessionStatus `json:"status,omitempty"`
	WaitingFor string        `json:"waitingFor,omitempty"`
	UpdatedAt  int64         `json:"updatedAt,omitempty"`
}

// SessionRegistry manages PID-based session tracking in a directory.
type SessionRegistry struct {
	mu         sync.Mutex
	dir        string
	pid        int
	registered bool
	isRunning  func(pid int) bool // customizable process liveness check
}

// NewSessionRegistry creates a registry using the given directory.
// isRunning checks if a process is alive (pass nil for default os-based check).
func NewSessionRegistry(dir string, isRunning func(int) bool) *SessionRegistry {
	if isRunning == nil {
		isRunning = defaultIsProcessRunning
	}
	return &SessionRegistry{
		dir:       dir,
		pid:       os.Getpid(),
		isRunning: isRunning,
	}
}

// Register writes a PID file for the current process.
func (r *SessionRegistry) Register(info SessionInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := os.MkdirAll(r.dir, 0o700); err != nil {
		return err
	}

	info.PID = r.pid
	if info.StartedAt == 0 {
		info.StartedAt = time.Now().UnixMilli()
	}

	data, err := json.Marshal(info)
	if err != nil {
		return err
	}

	path := r.pidFilePath(r.pid)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}

	r.registered = true
	return nil
}

// Update patches the current session's PID file with new fields.
func (r *SessionRegistry) Update(patch map[string]interface{}) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	path := r.pidFilePath(r.pid)
	data, err := r.readPIDFile(r.pid)
	if err != nil {
		return err
	}

	var existing map[string]interface{}
	if err := json.Unmarshal(data, &existing); err != nil {
		return err
	}

	for k, v := range patch {
		existing[k] = v
	}

	updated, err := json.Marshal(existing)
	if err != nil {
		return err
	}

	return os.WriteFile(path, updated, 0o600)
}

// Unregister removes the current session's PID file.
func (r *SessionRegistry) Unregister() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.registered = false
	return os.Remove(r.pidFilePath(r.pid))
}

// Count returns the number of live sessions, cleaning up stale entries.
func (r *SessionRegistry) Count() int {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		name := entry.Name()
		if !isValidPIDFile(name) {
			continue
		}
		pid := extractPID(name)
		if pid <= 0 {
			continue
		}

		if pid == r.pid {
			count++
			continue
		}

		if r.isRunning(pid) {
			count++
		} else {
			// Stale — clean up
			_ = os.Remove(filepath.Join(r.dir, name))
		}
	}
	return count
}

// List returns info for all live sessions.
func (r *SessionRegistry) List() []SessionInfo {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return nil
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		name := entry.Name()
		if !isValidPIDFile(name) {
			continue
		}
		pid := extractPID(name)
		if pid <= 0 {
			continue
		}

		if pid != r.pid && !r.isRunning(pid) {
			_ = os.Remove(filepath.Join(r.dir, name))
			continue
		}

		data, err := r.readPIDFile(pid)
		if err != nil {
			continue
		}

		var info SessionInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}
		sessions = append(sessions, info)
	}
	return sessions
}

// IsRegistered returns whether this process has a registered session.
func (r *SessionRegistry) IsRegistered() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.registered
}

// Dir returns the sessions directory path.
func (r *SessionRegistry) Dir() string {
	return r.dir
}

func (r *SessionRegistry) pidFilePath(pid int) string {
	return filepath.Join(r.dir, strconv.Itoa(pid)+".json")
}

func (r *SessionRegistry) readPIDFile(pid int) ([]byte, error) {
	return readFileFromRoot(r.dir, strconv.Itoa(pid)+".json")
}

func isValidPIDFile(name string) bool {
	if !strings.HasSuffix(name, ".json") {
		return false
	}
	prefix := strings.TrimSuffix(name, ".json")
	_, err := strconv.Atoi(prefix)
	return err == nil
}

func extractPID(name string) int {
	prefix := strings.TrimSuffix(name, ".json")
	pid, _ := strconv.Atoi(prefix)
	return pid
}

func defaultIsProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds. Signal 0 checks existence.
	err = proc.Signal(os.Signal(nil))
	return err == nil
}
