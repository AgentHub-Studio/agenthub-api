package agentic

import "errors"

// BackgroundAgentSchedulingModelRegistry catalogues the scheduling trigger
// profiles that govern how background agents are activated without an
// interactive REPL session.
//
// Sources drawn from arXiv:2604.14228v1:
//   - §10 / Table 3 "Comparative Analysis" (OpenClaw sub-agent delegation,
//     configurable nesting depth, thread-bound sessions, background runs with
//     configurable tool policy by depth)
//   - §11.6 "Emerging Directions — Proactive Architectures" (KAIROS tick-based
//     heartbeats, SleepTool economic throttling, terminal focus awareness)
//   - §12.3 "Harness Boundary Evolution" (*when* the harness acts)
//   - §12.4 "Horizon Scaling" (multi-session autonomous programs)
//
// This registry is a pure read-only catalogue — no goroutines, no timers.
// It complements:
//   - background_run.go  — lifecycle state machine for individual runs
//   - kairos_heartbeat.go — stateful KAIROS tick evaluator
//   - cronparser.go      — cron expression parsing
//
// FEAT-042 — BackgroundAgentSchedulingModelRegistry §10/§11.6/§12.3/§12.4.

// BackgroundScheduleTrigger is a stable slug identifying a scheduling trigger type.
type BackgroundScheduleTrigger string

const (
	// TriggerManual represents an explicitly user- or operator-initiated background
	// run.  Requires user presence at dispatch time; no autonomous scheduling.
	// Source: §10 table — sub-agent delegation requires explicit invocation.
	TriggerManual BackgroundScheduleTrigger = "manual"

	// TriggerCron represents a time-based recurring schedule expressed as a
	// cron expression (e.g. "0 8 * * 1-5" — Monday–Friday 08:00).
	// Source: §12.3 "when the harness acts" / §12.4 multi-session autonomous programs.
	TriggerCron BackgroundScheduleTrigger = "cron"

	// TriggerEvent represents activation by an external signal arriving on a
	// subscription channel (webhook, queue message, topic event).
	// Source: §10 comparative analysis — OpenClaw thread-bound sessions on supported
	// channels, deterministic binding rules routing events to agents.
	TriggerEvent BackgroundScheduleTrigger = "event"

	// TriggerWebhook represents a push-based HTTP callback delivering a payload.
	// Specialisation of event for direct HTTP delivery; distinct from queue-based.
	// Source: §10 OpenClaw channel routing; §12.3 harness boundary — "with whom".
	TriggerWebhook BackgroundScheduleTrigger = "webhook"

	// TriggerKairosTick represents the KAIROS proactive tick model: the harness
	// injects periodic <tick> prompts when no user message is pending; the model
	// decides whether to act or sleep via SleepTool.
	// Source: §11.6 "Proactive architectures — KAIROS".
	TriggerKairosTick BackgroundScheduleTrigger = "kairos_tick"

	// TriggerThreshold represents activation when a monitored metric or budget
	// crosses a configured threshold (e.g. error rate > 5%, token budget < 20%).
	// Source: background_run.go BackgroundTriggerThresholdCrossed; §12.4 horizon scaling.
	TriggerThreshold BackgroundScheduleTrigger = "threshold"

	// TriggerAbsenceTimeout represents activation when an expected signal fails
	// to arrive within a configured window (dead-man-switch pattern).
	// Source: background_run.go BackgroundTriggerAbsenceTimeout; §12.3/§12.4.
	TriggerAbsenceTimeout BackgroundScheduleTrigger = "absence_timeout"
)

// ResourceBudgetTier classifies the relative resource intensity of a scheduling
// trigger profile.  Tiers inform which triggers are safe for high-frequency use.
type ResourceBudgetTier string

const (
	// ResourceBudgetLow — minimal token / compute cost; safe at high frequency.
	ResourceBudgetLow ResourceBudgetTier = "low"

	// ResourceBudgetMedium — moderate cost; should be throttled or rate-limited.
	ResourceBudgetMedium ResourceBudgetTier = "medium"

	// ResourceBudgetHigh — significant inference cost per activation; economic
	// throttling (e.g. KAIROS SleepTool) is recommended.
	ResourceBudgetHigh ResourceBudgetTier = "high"
)

// BackgroundAgentScheduleProfile is the immutable descriptor for one scheduling
// trigger model.
type BackgroundAgentScheduleProfile struct {
	// TriggerID is the stable slug for this trigger type.
	TriggerID BackgroundScheduleTrigger

	// Label is the short human-readable name.
	Label string

	// Description explains when and why this trigger fires.
	Description string

	// PDFSection is the primary section reference in arXiv:2604.14228v1.
	PDFSection string

	// TriggerType is the broad category: "cron", "event", "webhook", "manual",
	// or "proactive".
	TriggerType string

	// IsPolling is true when the harness must actively poll or tick to detect
	// the trigger condition (rather than receiving a push signal).
	// KAIROS tick and threshold triggers are polling; webhook and event are not.
	IsPolling bool

	// DefaultIntervalHint is a human-readable hint for the polling cadence when
	// IsPolling is true (e.g. "5 minutes").  Empty for push-based triggers.
	DefaultIntervalHint string

	// SupportsCheckpointing is true when the trigger model natively integrates
	// with file-history checkpoints (§9.2 --rewind-files snapshots).
	// Cron and KairosTick triggers support checkpointing for long-horizon runs.
	SupportsCheckpointing bool

	// RequiresUserPresence is true when the trigger must be dispatched while
	// the user is actively present at the terminal.  Manual trigger requires
	// this; autonomous triggers do not.
	RequiresUserPresence bool

	// ResourceBudgetTier classifies the relative resource intensity per activation.
	ResourceBudgetTier ResourceBudgetTier

	// SupportsNestingDepthPolicy is true when the trigger respects a configurable
	// nesting-depth policy (cf. OpenClaw §10 sub-agent delegation: max 5, default 1,
	// recommended 2).
	SupportsNestingDepthPolicy bool

	// IsEconomicallyThrottled is true when the trigger is subject to economic
	// throttling via a sleep mechanism (KAIROS SleepTool — each wake-up after
	// inactivity costs a prompt-cache miss in addition to inference cost).
	IsEconomicallyThrottled bool
}

// BackgroundAgentSchedulingRegistry is the read-only registry of all scheduling
// trigger profiles.
type BackgroundAgentSchedulingRegistry struct {
	profiles []BackgroundAgentScheduleProfile
}

// NewBackgroundAgentSchedulingRegistry returns a registry pre-seeded with all
// seven scheduling trigger profiles derived from §10, §11.6, §12.3, and §12.4.
func NewBackgroundAgentSchedulingRegistry() *BackgroundAgentSchedulingRegistry {
	return &BackgroundAgentSchedulingRegistry{
		profiles: []BackgroundAgentScheduleProfile{
			{
				TriggerID:   TriggerManual,
				Label:       "Manual Dispatch",
				Description: "A human or operator explicitly initiates the background run. " +
					"No autonomous scheduling; requires user presence at the terminal at " +
					"dispatch time.  Maps to the default sub-agent invocation model in §10 " +
					"(Table 3): both Claude Code and OpenClaw require explicit delegation to " +
					"start a sub-agent run.",
				PDFSection:                "10 / Table 3",
				TriggerType:               "manual",
				IsPolling:                 false,
				DefaultIntervalHint:       "",
				SupportsCheckpointing:     false,
				RequiresUserPresence:      true,
				ResourceBudgetTier:        ResourceBudgetLow,
				SupportsNestingDepthPolicy: false,
				IsEconomicallyThrottled:   false,
			},
			{
				TriggerID:   TriggerCron,
				Label:       "Cron Schedule",
				Description: "A recurring time-based schedule expressed as a cron expression " +
					"(e.g. '0 8 * * 1-5' for Monday–Friday 08:00).  The harness polls the " +
					"clock at its tick cadence and fires the run when the expression matches. " +
					"Supports file-history checkpoints for long-horizon runs.  Drawn from " +
					"§12.3 'when the harness acts' and §12.4 multi-session autonomous programs.",
				PDFSection:                "12.3 / 12.4",
				TriggerType:               "cron",
				IsPolling:                 true,
				DefaultIntervalHint:       "1 minute resolution",
				SupportsCheckpointing:     true,
				RequiresUserPresence:      false,
				ResourceBudgetTier:        ResourceBudgetMedium,
				SupportsNestingDepthPolicy: true,
				IsEconomicallyThrottled:   false,
			},
			{
				TriggerID:   TriggerEvent,
				Label:       "External Event",
				Description: "An external signal arriving on a subscription channel (queue " +
					"message, topic event) activates the run.  Derived from the OpenClaw " +
					"§10 comparative analysis: thread-bound sessions on supported channels, " +
					"routed via deterministic binding rules.  Push-based — no polling required.",
				PDFSection:                "10 / Table 3",
				TriggerType:               "event",
				IsPolling:                 false,
				DefaultIntervalHint:       "",
				SupportsCheckpointing:     false,
				RequiresUserPresence:      false,
				ResourceBudgetTier:        ResourceBudgetMedium,
				SupportsNestingDepthPolicy: true,
				IsEconomicallyThrottled:   false,
			},
			{
				TriggerID:   TriggerWebhook,
				Label:       "Webhook Callback",
				Description: "A push-based HTTP POST delivers a payload directly to the harness " +
					"endpoint, activating the run immediately.  Distinguished from queue-based " +
					"events by direct HTTP delivery semantics.  Relates to §10 OpenClaw " +
					"channel routing and §12.3 'with whom the agent coordinates'.",
				PDFSection:                "10 / 12.3",
				TriggerType:               "webhook",
				IsPolling:                 false,
				DefaultIntervalHint:       "",
				SupportsCheckpointing:     false,
				RequiresUserPresence:      false,
				ResourceBudgetTier:        ResourceBudgetMedium,
				SupportsNestingDepthPolicy: false,
				IsEconomicallyThrottled:   false,
			},
			{
				TriggerID:   TriggerKairosTick,
				Label:       "KAIROS Proactive Tick",
				Description: "The harness injects a periodic <tick> prompt when no user message " +
					"is pending; the model decides whether to act or sleep via SleepTool. " +
					"Economic throttling binds activity to real API cost: each wake-up after " +
					"inactivity costs a full prompt-cache miss (cache TTL ~5 minutes) in " +
					"addition to the inference call.  Terminal-focus awareness maximises " +
					"autonomous action when the user is away and increases collaboration when " +
					"present.  Source: §11.6 'Proactive architectures — KAIROS'.",
				PDFSection:                "11.6",
				TriggerType:               "proactive",
				IsPolling:                 true,
				DefaultIntervalHint:       "5 minutes",
				SupportsCheckpointing:     true,
				RequiresUserPresence:      false,
				ResourceBudgetTier:        ResourceBudgetHigh,
				SupportsNestingDepthPolicy: false,
				IsEconomicallyThrottled:   true,
			},
			{
				TriggerID:   TriggerThreshold,
				Label:       "Metric Threshold Crossed",
				Description: "The run activates when a monitored metric or budget crosses a " +
					"configured threshold (e.g. error rate > 5%, remaining token budget < 20%). " +
					"The harness polls the metric source at its cadence.  Supports nesting-depth " +
					"policy for escalation runs.  Relates to §12.4 horizon scaling — long-horizon " +
					"dependability requires detecting when quality degrades across sessions.",
				PDFSection:                "12.4",
				TriggerType:               "cron",
				IsPolling:                 true,
				DefaultIntervalHint:       "configurable (e.g. 1 minute)",
				SupportsCheckpointing:     false,
				RequiresUserPresence:      false,
				ResourceBudgetTier:        ResourceBudgetLow,
				SupportsNestingDepthPolicy: true,
				IsEconomicallyThrottled:   false,
			},
			{
				TriggerID:   TriggerAbsenceTimeout,
				Label:       "Absence Timeout (Dead-Man Switch)",
				Description: "The run activates when an expected signal fails to arrive within " +
					"a configured window — a dead-man-switch pattern for reliability monitoring. " +
					"The harness polls a last-seen timestamp and fires when the interval elapses. " +
					"Relates to §12.4 horizon scaling: long-horizon autonomous programs must " +
					"detect stalled dependencies or missing heartbeats across sessions.",
				PDFSection:                "12.3 / 12.4",
				TriggerType:               "cron",
				IsPolling:                 true,
				DefaultIntervalHint:       "configurable (e.g. 15 minutes)",
				SupportsCheckpointing:     false,
				RequiresUserPresence:      false,
				ResourceBudgetTier:        ResourceBudgetLow,
				SupportsNestingDepthPolicy: false,
				IsEconomicallyThrottled:   false,
			},
		},
	}
}

// FindTriggerByID returns the profile for the given trigger ID, or
// (zero-value, false) when not found.
func (r *BackgroundAgentSchedulingRegistry) FindTriggerByID(id BackgroundScheduleTrigger) (BackgroundAgentScheduleProfile, bool) {
	for _, p := range r.profiles {
		if p.TriggerID == id {
			return p, true
		}
	}
	return BackgroundAgentScheduleProfile{}, false
}

// AllTriggers returns all profiles in registration order.
func (r *BackgroundAgentSchedulingRegistry) AllTriggers() []BackgroundAgentScheduleProfile {
	out := make([]BackgroundAgentScheduleProfile, len(r.profiles))
	copy(out, r.profiles)
	return out
}

// Count returns the total number of registered trigger profiles.
func (r *BackgroundAgentSchedulingRegistry) Count() int {
	return len(r.profiles)
}

// IsValidTriggerID reports whether id corresponds to a registered profile.
func (r *BackgroundAgentSchedulingRegistry) IsValidTriggerID(id BackgroundScheduleTrigger) bool {
	_, ok := r.FindTriggerByID(id)
	return ok
}

// PollingTriggers returns profiles where IsPolling is true — the harness must
// actively poll to detect the trigger condition.
func (r *BackgroundAgentSchedulingRegistry) PollingTriggers() []BackgroundAgentScheduleProfile {
	var out []BackgroundAgentScheduleProfile
	for _, p := range r.profiles {
		if p.IsPolling {
			out = append(out, p)
		}
	}
	return out
}

// EventDrivenTriggers returns profiles where IsPolling is false — the harness
// receives a push signal rather than polling for it.
func (r *BackgroundAgentSchedulingRegistry) EventDrivenTriggers() []BackgroundAgentScheduleProfile {
	var out []BackgroundAgentScheduleProfile
	for _, p := range r.profiles {
		if !p.IsPolling {
			out = append(out, p)
		}
	}
	return out
}

// TriggersByType returns profiles whose TriggerType matches the given string.
func (r *BackgroundAgentSchedulingRegistry) TriggersByType(triggerType string) []BackgroundAgentScheduleProfile {
	var out []BackgroundAgentScheduleProfile
	for _, p := range r.profiles {
		if p.TriggerType == triggerType {
			out = append(out, p)
		}
	}
	return out
}

// TriggersRequiringUserPresence returns profiles where RequiresUserPresence is true.
func (r *BackgroundAgentSchedulingRegistry) TriggersRequiringUserPresence() []BackgroundAgentScheduleProfile {
	var out []BackgroundAgentScheduleProfile
	for _, p := range r.profiles {
		if p.RequiresUserPresence {
			out = append(out, p)
		}
	}
	return out
}

// TriggersWithCheckpointing returns profiles where SupportsCheckpointing is true.
func (r *BackgroundAgentSchedulingRegistry) TriggersWithCheckpointing() []BackgroundAgentScheduleProfile {
	var out []BackgroundAgentScheduleProfile
	for _, p := range r.profiles {
		if p.SupportsCheckpointing {
			out = append(out, p)
		}
	}
	return out
}

// LeastResourceIntensiveTrigger returns the first profile with ResourceBudgetTier == "low".
// Returns (zero-value, false) if no low-budget trigger is registered.
func (r *BackgroundAgentSchedulingRegistry) LeastResourceIntensiveTrigger() (BackgroundAgentScheduleProfile, bool) {
	for _, p := range r.profiles {
		if p.ResourceBudgetTier == ResourceBudgetLow {
			return p, true
		}
	}
	return BackgroundAgentScheduleProfile{}, false
}

// --- Invariant validators ---

// ErrSchedulingInvariant is returned when a registry invariant is violated.
var ErrSchedulingInvariant = errors.New("background scheduling registry: invariant violated")

// AtLeastOneTrigger asserts the registry contains at least one profile.
func (r *BackgroundAgentSchedulingRegistry) AtLeastOneTrigger() error {
	if len(r.profiles) == 0 {
		return errors.New("background scheduling registry: must have at least one trigger profile")
	}
	return nil
}

// ManualTriggerRequiresUserPresence asserts that the manual trigger (if present)
// has RequiresUserPresence == true.
func (r *BackgroundAgentSchedulingRegistry) ManualTriggerRequiresUserPresence() error {
	p, ok := r.FindTriggerByID(TriggerManual)
	if !ok {
		return nil // no manual trigger registered — invariant vacuously satisfied
	}
	if !p.RequiresUserPresence {
		return errors.New("background scheduling registry: manual trigger must require user presence")
	}
	return nil
}

// CheckpointingTriggersAreNotUserPresent asserts that no profile has both
// SupportsCheckpointing == true and RequiresUserPresence == true.
// Checkpointing is relevant only for autonomous long-horizon runs.
func (r *BackgroundAgentSchedulingRegistry) CheckpointingTriggersAreNotUserPresent() error {
	for _, p := range r.profiles {
		if p.SupportsCheckpointing && p.RequiresUserPresence {
			return errors.New(
				"background scheduling registry: trigger " + string(p.TriggerID) +
					" must not require user presence when checkpointing is supported",
			)
		}
	}
	return nil
}
