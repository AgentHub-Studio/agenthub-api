// Package abtest implements the Agent A/B Testing framework.
// An ABTest routes a configurable percentage of chat sessions to an alternate
// agent version (the "variant"), enabling data-driven quality comparisons.
package abtest

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when an A/B test record cannot be found.
var ErrNotFound = errors.New("abtest: not found")

// ErrNameConflict is returned when a test name is already taken for the agent.
var ErrNameConflict = errors.New("abtest: name already in use for this agent")

// ErrActiveTestConflict is returned when an agent already has an active A/B test.
var ErrActiveTestConflict = errors.New("abtest: agent already has an active test")

// ErrValidation is the sentinel for request validation failures (HTTP 422).
var ErrValidation = errors.New("abtest: validation error")

// Variant identifies which branch a session was assigned to.
type Variant string

const (
	VariantControl Variant = "control"
	VariantVariant Variant = "variant"
)

// TestStatus represents the lifecycle of an A/B test.
type TestStatus string

const (
	TestStatusActive    TestStatus = "ACTIVE"
	TestStatusPaused    TestStatus = "PAUSED"
	TestStatusConcluded TestStatus = "CONCLUDED"
)

// IsValidTestStatus reports whether status belongs to the A/B test lifecycle
// defined by the public contract.
func IsValidTestStatus(status TestStatus) bool {
	switch status {
	case TestStatusActive, TestStatusPaused, TestStatusConcluded:
		return true
	default:
		return false
	}
}

// ABTest configures an A/B routing experiment for an agent.
// Stored in ah_{tenantID}.agent_ab_test — no tenant_id column.
type ABTest struct {
	ID          uuid.UUID
	AgentID     uuid.UUID
	Name        string
	Description string
	// ControlVersionID optionally pins the control to a specific published version.
	// When nil, the agent's current live configuration is used.
	ControlVersionID *uuid.UUID
	// VariantVersionID is the alternative version to test.
	VariantVersionID uuid.UUID
	// TrafficPercent is the fraction of sessions (0–100) routed to the variant.
	TrafficPercent int
	Status         TestStatus
	StartedAt      time.Time
	EndedAt        *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Assignment records which variant a session was assigned.
// Stored in ah_{tenantID}.agent_ab_assignment.
type Assignment struct {
	ID         uuid.UUID
	TestID     uuid.UUID
	SessionID  uuid.UUID
	Variant    Variant
	AssignedAt time.Time
}
