package agentic

import (
	"fmt"
	"strings"
	"sync"
)

// String accumulator that safely handles large outputs by truncating
// from the end when a size limit is exceeded.
//
// Inspired by Claude Code's EndTruncatingAccumulator — prevents
// out-of-memory crashes while preserving the beginning of tool output.
// The beginning is most valuable because it typically contains headers,
// command names, and initial results. When truncated, a marker shows
// how much was removed. Thread-safe for concurrent appends.

// DefaultMaxAccumulatorSize is the default max size (32MB).
const DefaultMaxAccumulatorSize = 1 << 25

// TruncAccumulator accumulates string data with end-truncation.
type TruncAccumulator struct {
	mu                 sync.Mutex
	content            strings.Builder
	maxSize            int
	truncated          bool
	totalBytesReceived int
}

// NewTruncAccumulator creates an accumulator with the given max size.
// Use 0 for DefaultMaxAccumulatorSize.
func NewTruncAccumulator(maxSize int) *TruncAccumulator {
	if maxSize <= 0 {
		maxSize = DefaultMaxAccumulatorSize
	}
	return &TruncAccumulator{maxSize: maxSize}
}

// Append adds data to the accumulator. If the total size exceeds
// maxSize, the excess is silently dropped.
func (a *TruncAccumulator) Append(data string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.totalBytesReceived += len(data)

	if a.truncated && a.content.Len() >= a.maxSize {
		return
	}

	if a.content.Len()+len(data) > a.maxSize {
		remaining := a.maxSize - a.content.Len()
		if remaining > 0 {
			a.content.WriteString(data[:remaining])
		}
		a.truncated = true
		return
	}

	a.content.WriteString(data)
}

// String returns the accumulated content with a truncation marker
// if any data was dropped.
func (a *TruncAccumulator) String() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.truncated {
		return a.content.String()
	}

	dropped := a.totalBytesReceived - a.maxSize
	droppedKB := (dropped + 512) / 1024
	return fmt.Sprintf("%s\n... [output truncated - %dKB removed]", a.content.String(), droppedKB)
}

// Len returns the current accumulated size (not including the truncation marker).
func (a *TruncAccumulator) Len() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.content.Len()
}

// IsTruncated returns whether any data has been dropped.
func (a *TruncAccumulator) IsTruncated() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.truncated
}

// TotalBytes returns total bytes received (before truncation).
func (a *TruncAccumulator) TotalBytes() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.totalBytesReceived
}

// Clear resets the accumulator.
func (a *TruncAccumulator) Clear() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.content.Reset()
	a.truncated = false
	a.totalBytesReceived = 0
}

// SafeJoinLines joins strings with a delimiter, truncating if the
// result exceeds maxSize. Uses "...[truncated]" as the truncation marker.
func SafeJoinLines(lines []string, delimiter string, maxSize int) string {
	const marker = "...[truncated]"
	var result strings.Builder

	for i, line := range lines {
		delim := ""
		if i > 0 {
			delim = delimiter
		}
		full := delim + line

		if result.Len()+len(full) <= maxSize {
			result.WriteString(full)
			continue
		}

		remaining := maxSize - result.Len() - len(delim) - len(marker)
		if remaining > 0 {
			result.WriteString(delim)
			result.WriteString(line[:remaining])
			result.WriteString(marker)
		} else {
			result.WriteString(marker)
		}
		return result.String()
	}

	return result.String()
}

// TruncateToLines truncates text to a maximum number of lines.
func TruncateToLines(text string, maxLines int) string {
	lines := strings.SplitN(text, "\n", maxLines+1)
	if len(lines) <= maxLines {
		return text
	}
	return strings.Join(lines[:maxLines], "\n") + "…"
}
