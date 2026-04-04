package agentic

import (
	"fmt"
	"sync"
	"time"
)

// Rate limit tracking and usage management.
//
// Inspired by Claude Code's rate limit and usage tracking — monitors
// API utilization across multiple time windows (e.g., 5-hour, 7-day)
// with model-specific limits and fallback support.

// RateLimitWindow identifies a tracking time window.
type RateLimitWindow string

const (
	WindowFiveHour    RateLimitWindow = "five_hour"
	WindowSevenDay    RateLimitWindow = "seven_day"
	WindowSevenDayAPI RateLimitWindow = "seven_day_api"
)

// RateLimit holds the utilization state for a single window.
type RateLimit struct {
	// Utilization is the fraction used (0.0 to 1.0+). Nil means unknown.
	Utilization *float64 `json:"utilization,omitempty"`
	// ResetsAt is when the window resets. Nil means unknown.
	ResetsAt *time.Time `json:"resetsAt,omitempty"`
}

// ExtraUsage tracks overage/credit usage.
type ExtraUsage struct {
	Enabled     bool     `json:"enabled"`
	MonthlyLimit *float64 `json:"monthlyLimit,omitempty"`
	UsedCredits  *float64 `json:"usedCredits,omitempty"`
	Utilization  *float64 `json:"utilization,omitempty"`
}

// Utilization aggregates rate limits across all windows.
type Utilization struct {
	Windows    map[RateLimitWindow]RateLimit `json:"windows"`
	ExtraUsage *ExtraUsage                   `json:"extraUsage,omitempty"`
	FetchedAt  time.Time                     `json:"fetchedAt"`
}

// RateLimitState represents the current rate limit status.
type RateLimitState int

const (
	// RateLimitOK means usage is within limits.
	RateLimitOK RateLimitState = iota
	// RateLimitWarning means usage is approaching limits.
	RateLimitWarning
	// RateLimitExceeded means the limit has been hit.
	RateLimitExceeded
)

// String returns the state label.
func (s RateLimitState) String() string {
	switch s {
	case RateLimitOK:
		return "ok"
	case RateLimitWarning:
		return "warning"
	case RateLimitExceeded:
		return "exceeded"
	default:
		return "unknown"
	}
}

// RateLimitWarningThreshold is the utilization level that triggers a warning.
const RateLimitWarningThreshold = 0.8

// RateLimitExceededThreshold is the utilization level considered exceeded.
const RateLimitExceededThreshold = 1.0

// RateLimitManager tracks rate limits and provides status checks.
type RateLimitManager struct {
	mu          sync.RWMutex
	utilization *Utilization
	history     []Utilization
	maxHistory  int
}

// NewRateLimitManager creates a rate limit manager.
func NewRateLimitManager() *RateLimitManager {
	return &RateLimitManager{
		maxHistory: 100,
	}
}

// Update records new utilization data.
func (m *RateLimitManager) Update(u Utilization) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if u.FetchedAt.IsZero() {
		u.FetchedAt = time.Now()
	}

	m.utilization = &u
	m.history = append(m.history, u)
	if len(m.history) > m.maxHistory {
		m.history = m.history[1:]
	}
}

// UpdateWindow updates a single rate limit window.
func (m *RateLimitManager) UpdateWindow(window RateLimitWindow, rl RateLimit) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.utilization == nil {
		m.utilization = &Utilization{
			Windows:   make(map[RateLimitWindow]RateLimit),
			FetchedAt: time.Now(),
		}
	}
	if m.utilization.Windows == nil {
		m.utilization.Windows = make(map[RateLimitWindow]RateLimit)
	}
	m.utilization.Windows[window] = rl
}

// GetState returns the worst-case rate limit state across all windows.
func (m *RateLimitManager) GetState() RateLimitState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.utilization == nil {
		return RateLimitOK
	}

	worst := RateLimitOK
	for _, rl := range m.utilization.Windows {
		s := windowState(rl)
		if s > worst {
			worst = s
		}
	}
	return worst
}

// GetWindowState returns the state for a specific window.
func (m *RateLimitManager) GetWindowState(window RateLimitWindow) RateLimitState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.utilization == nil {
		return RateLimitOK
	}
	rl, ok := m.utilization.Windows[window]
	if !ok {
		return RateLimitOK
	}
	return windowState(rl)
}

// GetUtilization returns the current utilization snapshot.
func (m *RateLimitManager) GetUtilization() *Utilization {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.utilization == nil {
		return nil
	}
	copy := *m.utilization
	return &copy
}

// IsExceeded returns true if any window has hit its limit.
func (m *RateLimitManager) IsExceeded() bool {
	return m.GetState() == RateLimitExceeded
}

// TimeUntilReset returns the shortest time until any exceeded window resets.
func (m *RateLimitManager) TimeUntilReset() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.utilization == nil {
		return 0
	}

	var shortest time.Duration
	found := false
	now := time.Now()

	for _, rl := range m.utilization.Windows {
		if rl.Utilization != nil && *rl.Utilization >= RateLimitExceededThreshold && rl.ResetsAt != nil {
			d := rl.ResetsAt.Sub(now)
			if d > 0 && (!found || d < shortest) {
				shortest = d
				found = true
			}
		}
	}
	return shortest
}

// Summary returns a human-readable rate limit summary.
func (m *RateLimitManager) Summary() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.utilization == nil {
		return "No rate limit data available."
	}

	state := RateLimitOK
	for _, rl := range m.utilization.Windows {
		s := windowState(rl)
		if s > state {
			state = s
		}
	}

	switch state {
	case RateLimitExceeded:
		return fmt.Sprintf("Rate limit exceeded. Resets in %s.", m.timeUntilResetLocked())
	case RateLimitWarning:
		return "Approaching rate limit. Consider reducing request frequency."
	default:
		return "Rate limits OK."
	}
}

// HistoryLen returns the number of recorded utilization snapshots.
func (m *RateLimitManager) HistoryLen() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.history)
}

// Reset clears all utilization data.
func (m *RateLimitManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.utilization = nil
	m.history = nil
}

func (m *RateLimitManager) timeUntilResetLocked() string {
	if m.utilization == nil {
		return "unknown"
	}
	now := time.Now()
	for _, rl := range m.utilization.Windows {
		if rl.Utilization != nil && *rl.Utilization >= RateLimitExceededThreshold && rl.ResetsAt != nil {
			d := rl.ResetsAt.Sub(now)
			if d > 0 {
				return d.Round(time.Second).String()
			}
		}
	}
	return "unknown"
}

func windowState(rl RateLimit) RateLimitState {
	if rl.Utilization == nil {
		return RateLimitOK
	}
	if *rl.Utilization >= RateLimitExceededThreshold {
		return RateLimitExceeded
	}
	if *rl.Utilization >= RateLimitWarningThreshold {
		return RateLimitWarning
	}
	return RateLimitOK
}
