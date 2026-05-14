package agentic

import (
	"errors"
	"time"
)

// KairosHeartbeat implements the proactive background agent loop described
// in PDF arXiv:2604.14228v1 §11.6.
//
// KAIROS injects periodic <tick> prompts when no user message is pending.
// The model decides whether to act or sleep.  Economic throttling (SleepTool)
// couples agent activity to real API cost: the prompt cache expires after
// ~5 minutes of inactivity, so each wake-up after a sleep costs a full-cache
// miss in addition to the inference call.
//
// This implementation is a pure-state machine — no goroutines or timers.
// The caller drives ticks by calling NextDecision at its chosen cadence.

// KairosDecisionType signals whether the agent should act or sleep.
type KairosDecisionType string

const (
	KairosAct   KairosDecisionType = "act"
	KairosSleep KairosDecisionType = "sleep"
)

// KairosDecision is the outcome of a single tick evaluation.
type KairosDecision struct {
	Decision          KairosDecisionType
	Reason            string
	EstimatedCostUSD  float64 // cumulative cost of ticks so far (act ticks only)
	TickCount         int
	SleepUntil        time.Time // only meaningful when Decision == KairosSleep
}

// PresenceSignal carries the last user activity timestamp.
// A zero LastMessageAt means the user is considered away.
type PresenceSignal struct {
	LastMessageAt time.Time
}

// IsPresent returns true if the user was active within the presence window
// relative to the provided now timestamp.
func (p PresenceSignal) IsPresent(now time.Time, window time.Duration) bool {
	if p.LastMessageAt.IsZero() {
		return false
	}
	return now.Sub(p.LastMessageAt) <= window
}

// KairosConfig configures the heartbeat scheduler.
type KairosConfig struct {
	// TickInterval is the base cadence between ticks (default: 5 minutes).
	TickInterval time.Duration
	// PresenceWindow is how recent a user message must be to count as "present"
	// (default: 30 seconds).
	PresenceWindow time.Duration
	// MaxActTicksBeforeForcedSleep limits consecutive act decisions before
	// a mandatory sleep is injected (default: 12 → 1 hour at 5m ticks).
	MaxActTicksBeforeForcedSleep int
	// SleepDuration is how long the agent sleeps after a forced or chosen sleep
	// (default: TickInterval × 3).
	SleepDuration time.Duration
	// EstimatedAPICallCostUSD is the cost of a single inference call used
	// to calculate cumulative cost in decisions (default: 0.001).
	EstimatedAPICallCostUSD float64
	// EconomicBudgetUSD caps total cumulative act cost before forcing sleep
	// (0 = unlimited).
	EconomicBudgetUSD float64
}

// DefaultKairosConfig returns sensible defaults inspired by §11.6 values.
func DefaultKairosConfig() KairosConfig {
	return KairosConfig{
		TickInterval:                 5 * time.Minute,
		PresenceWindow:               30 * time.Second,
		MaxActTicksBeforeForcedSleep: 12,
		SleepDuration:                15 * time.Minute,
		EstimatedAPICallCostUSD:      0.001,
		EconomicBudgetUSD:            0,
	}
}

// Validate returns an error if config values are invalid.
func (c KairosConfig) Validate() error {
	if c.TickInterval <= 0 {
		return errors.New("kairos: TickInterval must be positive")
	}
	if c.PresenceWindow < 0 {
		return errors.New("kairos: PresenceWindow must be non-negative")
	}
	if c.MaxActTicksBeforeForcedSleep < 0 {
		return errors.New("kairos: MaxActTicksBeforeForcedSleep must be non-negative")
	}
	if c.EstimatedAPICallCostUSD < 0 {
		return errors.New("kairos: EstimatedAPICallCostUSD must be non-negative")
	}
	if c.EconomicBudgetUSD < 0 {
		return errors.New("kairos: EconomicBudgetUSD must be non-negative")
	}
	return nil
}

// KairosHeartbeat is a stateful proactive agent heartbeat tracker.
type KairosHeartbeat struct {
	config          KairosConfig
	tickCount       int     // total ticks evaluated
	actTickCount    int     // consecutive act decisions
	cumulativeCostUSD float64
	sleepingUntil   time.Time
}

// NewKairosHeartbeat creates a new heartbeat tracker.
func NewKairosHeartbeat(config KairosConfig) (*KairosHeartbeat, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if config.SleepDuration <= 0 {
		config.SleepDuration = config.TickInterval * 3
	}
	return &KairosHeartbeat{config: config}, nil
}

// NextDecision evaluates the current tick and returns an act/sleep decision.
// now is the current wall-clock time; pass time.Now() in production.
func (h *KairosHeartbeat) NextDecision(now time.Time, signal PresenceSignal) KairosDecision {
	h.tickCount++

	// Enforce active sleep window — still cooling down from a previous sleep.
	if !h.sleepingUntil.IsZero() && now.Before(h.sleepingUntil) {
		return KairosDecision{
			Decision:  KairosSleep,
			Reason:    "cooling down from previous sleep window",
			TickCount: h.tickCount,
			SleepUntil: h.sleepingUntil,
		}
	}

	// Reset sleep window if elapsed.
	if !h.sleepingUntil.IsZero() && !now.Before(h.sleepingUntil) {
		h.sleepingUntil = time.Time{}
		h.actTickCount = 0
	}

	// User is present — sleep to preserve collaboration mode.
	if signal.IsPresent(now, h.config.PresenceWindow) {
		h.actTickCount = 0
		return KairosDecision{
			Decision:  KairosSleep,
			Reason:    "user is present — deferring to collaboration mode",
			TickCount: h.tickCount,
		}
	}

	// Economic budget exhausted — mandatory sleep.
	if h.config.EconomicBudgetUSD > 0 && h.cumulativeCostUSD >= h.config.EconomicBudgetUSD {
		h.actTickCount = 0
		sleepUntil := now.Add(h.config.SleepDuration)
		h.sleepingUntil = sleepUntil
		return KairosDecision{
			Decision:         KairosSleep,
			Reason:           "economic budget exhausted",
			EstimatedCostUSD: h.cumulativeCostUSD,
			TickCount:        h.tickCount,
			SleepUntil:       sleepUntil,
		}
	}

	// Consecutive act limit reached — forced sleep to prevent runaway loops.
	maxAct := h.config.MaxActTicksBeforeForcedSleep
	if maxAct > 0 && h.actTickCount >= maxAct {
		h.actTickCount = 0
		sleepUntil := now.Add(h.config.SleepDuration)
		h.sleepingUntil = sleepUntil
		return KairosDecision{
			Decision:         KairosSleep,
			Reason:           "consecutive act limit reached — forced sleep",
			EstimatedCostUSD: h.cumulativeCostUSD,
			TickCount:        h.tickCount,
			SleepUntil:       sleepUntil,
		}
	}

	// User away and budget not exhausted — act.
	h.actTickCount++
	h.cumulativeCostUSD += h.config.EstimatedAPICallCostUSD
	return KairosDecision{
		Decision:         KairosAct,
		Reason:           "user away — executing proactive tick",
		EstimatedCostUSD: h.cumulativeCostUSD,
		TickCount:        h.tickCount,
	}
}

// TickCount returns total ticks evaluated.
func (h *KairosHeartbeat) TickCount() int { return h.tickCount }

// CumulativeCostUSD returns total cumulative act cost.
func (h *KairosHeartbeat) CumulativeCostUSD() float64 { return h.cumulativeCostUSD }

// Reset clears all state, preserving config.
func (h *KairosHeartbeat) Reset() {
	h.tickCount = 0
	h.actTickCount = 0
	h.cumulativeCostUSD = 0
	h.sleepingUntil = time.Time{}
}

// IsSleeping returns true if the heartbeat is in a forced sleep window.
func (h *KairosHeartbeat) IsSleeping(now time.Time) bool {
	return !h.sleepingUntil.IsZero() && now.Before(h.sleepingUntil)
}
