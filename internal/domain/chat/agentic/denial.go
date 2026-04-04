package agentic

import (
	"fmt"
	"sync"
)

// DenialRecord tracks denial counts for a specific tool.
type DenialRecord struct {
	ToolName       string
	Count          int
	Escalated      bool
	LastInput      string
	EscalationHint string
}

// DenialTracker records tool denials and triggers escalation after a threshold.
// It is scoped to a single agentic run (not shared across sessions).
type DenialTracker struct {
	mu        sync.Mutex
	records   map[string]*DenialRecord
	threshold int
}

// NewDenialTracker creates a DenialTracker that escalates after threshold
// consecutive denials of the same tool. If threshold <= 0, defaults to 3.
func NewDenialTracker(threshold int) *DenialTracker {
	if threshold <= 0 {
		threshold = 3
	}
	return &DenialTracker{
		records:   make(map[string]*DenialRecord),
		threshold: threshold,
	}
}

// RecordDenial records a denial of a tool call and returns whether escalation
// was triggered by this denial (i.e., the threshold was just reached).
func (dt *DenialTracker) RecordDenial(toolName, toolInput string) (escalated bool) {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	rec, ok := dt.records[toolName]
	if !ok {
		rec = &DenialRecord{ToolName: toolName}
		dt.records[toolName] = rec
	}

	rec.Count++
	rec.LastInput = toolInput

	if rec.Count >= dt.threshold && !rec.Escalated {
		rec.Escalated = true
		rec.EscalationHint = fmt.Sprintf(
			"Tool '%s' has been denied %d times. Stop attempting to use this tool and find an alternative approach, "+
				"or explain to the user why this tool is needed.",
			toolName, rec.Count,
		)
		return true
	}

	return false
}

// RecordAllow resets the denial counter for a tool (called when a tool succeeds).
func (dt *DenialTracker) RecordAllow(toolName string) {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	delete(dt.records, toolName)
}

// GetRecord returns the denial record for a tool, or nil if none exists.
func (dt *DenialTracker) GetRecord(toolName string) *DenialRecord {
	dt.mu.Lock()
	defer dt.mu.Unlock()
	rec, ok := dt.records[toolName]
	if !ok {
		return nil
	}
	// Return a copy to avoid races.
	copy := *rec
	return &copy
}

// EscalationHints returns context strings for all escalated tools, suitable
// for injection into the LLM messages before the next call.
func (dt *DenialTracker) EscalationHints() []string {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	var hints []string
	for _, rec := range dt.records {
		if rec.Escalated && rec.EscalationHint != "" {
			hints = append(hints, rec.EscalationHint)
		}
	}
	return hints
}

// DeniedTools returns the names of all tools that have been denied at least once.
func (dt *DenialTracker) DeniedTools() map[string]int {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	result := make(map[string]int, len(dt.records))
	for name, rec := range dt.records {
		result[name] = rec.Count
	}
	return result
}

// TotalDenials returns the total number of denials across all tools.
func (dt *DenialTracker) TotalDenials() int {
	dt.mu.Lock()
	defer dt.mu.Unlock()

	total := 0
	for _, rec := range dt.records {
		total += rec.Count
	}
	return total
}
