package agentic

import (
	"errors"
	"sync"
	"time"
)

// ModelContextSpec describes the token budget for a specific model.
type ModelContextSpec struct {
	ModelID         string
	ContextWindow   int // total tokens the model can accept in one request
	MaxOutputTokens int // maximum tokens the model can generate per response
}

// Context window alert thresholds used by ContextWindowMonitor.
// These are distinct from the autocompact.go thresholds: warning fires earlier
// so the agent can plan a graceful compaction before hitting critical.
const (
	// CWMonitorWarningBufferTokens triggers a warning alert when this many
	// tokens remain in the effective context window.
	CWMonitorWarningBufferTokens = 40_000

	// CWMonitorCriticalBufferTokens triggers a critical alert when this many
	// tokens remain (matches ErrorThresholdBufferTokens for consistency).
	CWMonitorCriticalBufferTokens = 20_000

	// CWMonitorBlockingBufferTokens triggers a blocking alert (same as
	// ManualCompactBufferTokens — agent must compact before next turn).
	CWMonitorBlockingBufferTokens = ManualCompactBufferTokens
)

// ModelContextRegistry maps model IDs to their context specifications.
// Keyed by the canonical model ID string.
var ModelContextRegistry = map[string]ModelContextSpec{
	// Claude Haiku 4.5
	"claude-haiku-4-5-20251001": {
		ModelID: "claude-haiku-4-5-20251001", ContextWindow: 200_000, MaxOutputTokens: 8_192,
	},
	// Claude Sonnet 4.6
	"claude-sonnet-4-6": {
		ModelID: "claude-sonnet-4-6", ContextWindow: 200_000, MaxOutputTokens: 8_192,
	},
	// Claude Opus 4.7
	"claude-opus-4-7": {
		ModelID: "claude-opus-4-7", ContextWindow: 200_000, MaxOutputTokens: 32_000,
	},
}

// LookupModel returns the ModelContextSpec for modelID, or false when unknown.
func LookupModel(modelID string) (ModelContextSpec, bool) {
	spec, ok := ModelContextRegistry[modelID]
	return spec, ok
}

// AllSupportedModelIDs returns every model ID in the registry.
func AllSupportedModelIDs() []string {
	ids := make([]string, 0, len(ModelContextRegistry))
	for id := range ModelContextRegistry {
		ids = append(ids, id)
	}
	return ids
}

// ContextWindowAlertLevel classifies how close to the limit the usage is.
type ContextWindowAlertLevel string

const (
	ContextWindowAlertWarning  ContextWindowAlertLevel = "warning"
	ContextWindowAlertCritical ContextWindowAlertLevel = "critical"
	ContextWindowAlertBlocking ContextWindowAlertLevel = "blocking"
)

// AllContextWindowAlertLevels lists every valid alert level.
var AllContextWindowAlertLevels = []ContextWindowAlertLevel{
	ContextWindowAlertWarning,
	ContextWindowAlertCritical,
	ContextWindowAlertBlocking,
}

// IsValid returns true for recognised alert levels.
func (l ContextWindowAlertLevel) IsValid() bool {
	for _, v := range AllContextWindowAlertLevels {
		if l == v {
			return true
		}
	}
	return false
}

// ContextWindowAlert is the event payload emitted by the ContextWindowAlert hook
// (HookContextWindowAlertExt). The payload is delivered verbatim to registered
// hook handlers.
type ContextWindowAlert struct {
	SessionID      string                  `json:"sessionId"`
	ModelID        string                  `json:"modelId"`
	TurnNumber     int                     `json:"turnNumber"`
	TokensUsed     int                     `json:"tokensUsed"`
	EffectiveLimit int                     `json:"effectiveLimit"`
	PercentUsed    int                     `json:"percentUsed"`
	Level          ContextWindowAlertLevel `json:"level"`
	EmittedAt      time.Time               `json:"emittedAt"`
}

// Sentinel errors for ContextWindowMonitor.
var (
	ErrCWMonitorModelUnknown        = errors.New("context_window_monitor: model not in registry")
	ErrCWMonitorLimitPctOutOfRange  = errors.New("context_window_monitor: context_window_pct must be 1–100")
	ErrCWMonitorNegativeTokens      = errors.New("context_window_monitor: token count must be non-negative")
)

// ContextWindowMonitor tracks per-session token usage and emits
// ContextWindowAlert events when configurable thresholds are breached.
//
// CONTEXT-001: bridges the frontmatter context_window_pct field with the
// existing CalculateTokenWarningState machinery and the ContextWindowAlert hook.
type ContextWindowMonitor struct {
	mu sync.Mutex

	sessionID  string
	modelID    string
	spec       ModelContextSpec
	limitPct   int // 1-100 from frontmatter context_window_pct (100 = full window)

	effectiveLimit int // pre-computed: spec.EffectiveContextWindow * limitPct / 100
	turnNumber     int
	totalUsed      int
	lastAlert      *ContextWindowAlert
}

// NewContextWindowMonitor creates a monitor for the given session and model.
// limitPct is the value from the agent frontmatter's context_window_pct field (1–100).
// Pass 100 to allow the full effective context window.
func NewContextWindowMonitor(sessionID, modelID string, limitPct int) (*ContextWindowMonitor, error) {
	spec, ok := LookupModel(modelID)
	if !ok {
		return nil, ErrCWMonitorModelUnknown
	}
	if limitPct < 1 || limitPct > 100 {
		return nil, ErrCWMonitorLimitPctOutOfRange
	}
	effective := GetEffectiveContextWindowSize(spec.ContextWindow, spec.MaxOutputTokens)
	capped := effective * limitPct / 100
	return &ContextWindowMonitor{
		sessionID:      sessionID,
		modelID:        modelID,
		spec:           spec,
		limitPct:       limitPct,
		effectiveLimit: capped,
	}, nil
}

// RecordTurn records the cumulative token count after a model response turn
// and returns an alert if a threshold was breached, or nil if none was crossed.
func (m *ContextWindowMonitor) RecordTurn(cumulativeTokensUsed int) (*ContextWindowAlert, error) {
	if cumulativeTokensUsed < 0 {
		return nil, ErrCWMonitorNegativeTokens
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.turnNumber++
	m.totalUsed = cumulativeTokensUsed

	var level ContextWindowAlertLevel
	switch {
	case cumulativeTokensUsed >= m.effectiveLimit-CWMonitorBlockingBufferTokens:
		level = ContextWindowAlertBlocking
	case cumulativeTokensUsed >= m.effectiveLimit-CWMonitorCriticalBufferTokens:
		level = ContextWindowAlertCritical
	case cumulativeTokensUsed >= m.effectiveLimit-CWMonitorWarningBufferTokens:
		level = ContextWindowAlertWarning
	default:
		return nil, nil
	}

	state := CalculateTokenWarningState(cumulativeTokensUsed, m.effectiveLimit, false)

	percentUsed := 100 - state.PercentLeft

	alert := &ContextWindowAlert{
		SessionID:      m.sessionID,
		ModelID:        m.modelID,
		TurnNumber:     m.turnNumber,
		TokensUsed:     cumulativeTokensUsed,
		EffectiveLimit: m.effectiveLimit,
		PercentUsed:    percentUsed,
		Level:          level,
		EmittedAt:      time.Now().UTC(),
	}
	m.lastAlert = alert
	return alert, nil
}

// Snapshot returns the current usage totals without mutating state.
func (m *ContextWindowMonitor) Snapshot() (turnNumber, tokensUsed, effectiveLimit int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.turnNumber, m.totalUsed, m.effectiveLimit
}

// LastAlert returns the most recent alert emitted, or nil if none was emitted.
func (m *ContextWindowMonitor) LastAlert() *ContextWindowAlert {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastAlert
}
