package agentic

import (
	"sync"
	"time"
)

// Context-aware tip scheduling with LRU selection.
//
// Inspired by Claude Code's tipScheduler.ts — selects and displays
// contextual tips to users, avoiding repetition via last-shown tracking
// and session cooldown.

// TipContext identifies the situation where a tip may be relevant.
type TipContext string

const (
	TipContextSpinner   TipContext = "spinner"
	TipContextIdle      TipContext = "idle"
	TipContextError     TipContext = "error"
	TipContextFirstRun  TipContext = "first_run"
	TipContextToolUse   TipContext = "tool_use"
)

// Tip defines a user-facing tip.
type Tip struct {
	// ID uniquely identifies this tip.
	ID string `json:"id"`
	// Content is the tip text.
	Content string `json:"content"`
	// Contexts are the situations where this tip is relevant.
	Contexts []TipContext `json:"contexts"`
	// CooldownSessions is the minimum number of sessions before reshowing.
	CooldownSessions int `json:"cooldownSessions,omitempty"`
}

// TipRecord tracks when a tip was last shown.
type TipRecord struct {
	TipID          string    `json:"tipId"`
	LastShownAt    time.Time `json:"lastShownAt"`
	ShowCount      int       `json:"showCount"`
	SessionsShown  int       `json:"sessionsShown"`
}

// TipScheduler selects tips based on context and display history.
type TipScheduler struct {
	mu            sync.Mutex
	tips          []Tip
	records       map[string]*TipRecord
	currentSession int
	enabled       bool
}

// NewTipScheduler creates a tip scheduler with the given tips.
func NewTipScheduler(tips []Tip) *TipScheduler {
	return &TipScheduler{
		tips:    tips,
		records: make(map[string]*TipRecord),
		enabled: true,
	}
}

// SetEnabled toggles tip scheduling on or off.
func (s *TipScheduler) SetEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.enabled = enabled
}

// IsEnabled returns whether tips are enabled.
func (s *TipScheduler) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

// AdvanceSession increments the session counter.
func (s *TipScheduler) AdvanceSession() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.currentSession++
}

// GetTip returns the best tip for the given context, or nil if none available.
// Uses LRU selection — picks the tip with the longest time since last shown.
func (s *TipScheduler) GetTip(ctx TipContext) *Tip {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.enabled {
		return nil
	}

	relevant := s.getRelevantTips(ctx)
	if len(relevant) == 0 {
		return nil
	}

	return s.selectLRU(relevant)
}

// RecordShown marks a tip as shown.
func (s *TipScheduler) RecordShown(tipID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.records[tipID]
	if !ok {
		rec = &TipRecord{TipID: tipID}
		s.records[tipID] = rec
	}
	rec.LastShownAt = time.Now()
	rec.ShowCount++
	rec.SessionsShown = s.currentSession
}

// GetRecord returns the display record for a tip.
func (s *TipScheduler) GetRecord(tipID string) *TipRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec, ok := s.records[tipID]
	if !ok {
		return nil
	}
	copy := *rec
	return &copy
}

// TotalTips returns the number of registered tips.
func (s *TipScheduler) TotalTips() int {
	return len(s.tips)
}

// AddTip registers a new tip.
func (s *TipScheduler) AddTip(tip Tip) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tips = append(s.tips, tip)
}

// Reset clears all display records.
func (s *TipScheduler) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = make(map[string]*TipRecord)
}

// getRelevantTips filters tips by context and cooldown. Must be called with mu held.
func (s *TipScheduler) getRelevantTips(ctx TipContext) []Tip {
	var result []Tip
	for _, tip := range s.tips {
		if !s.tipMatchesContext(tip, ctx) {
			continue
		}
		if s.tipInCooldown(tip) {
			continue
		}
		result = append(result, tip)
	}
	return result
}

// tipMatchesContext checks if a tip is relevant for the given context.
func (s *TipScheduler) tipMatchesContext(tip Tip, ctx TipContext) bool {
	if len(tip.Contexts) == 0 {
		return true // no context restriction = always relevant
	}
	for _, c := range tip.Contexts {
		if c == ctx {
			return true
		}
	}
	return false
}

// tipInCooldown checks if a tip is still in its cooldown period.
func (s *TipScheduler) tipInCooldown(tip Tip) bool {
	if tip.CooldownSessions <= 0 {
		return false
	}
	rec, ok := s.records[tip.ID]
	if !ok {
		return false
	}
	return s.currentSession-rec.SessionsShown < tip.CooldownSessions
}

// selectLRU picks the tip with the longest time since last shown.
func (s *TipScheduler) selectLRU(tips []Tip) *Tip {
	var best *Tip
	var bestTime time.Time
	first := true

	for i := range tips {
		rec, shown := s.records[tips[i].ID]
		if !shown {
			// Never shown — highest priority.
			return &tips[i]
		}
		if first || rec.LastShownAt.Before(bestTime) {
			best = &tips[i]
			bestTime = rec.LastShownAt
			first = false
		}
	}

	return best
}
