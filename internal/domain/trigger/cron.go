package trigger

import (
	"fmt"
	"strings"
	"time"
)

// SimpleCronParser is a minimal cron parser for 5-field cron expressions.
// For production use, consider replacing with github.com/robfig/cron/v3.
type SimpleCronParser struct{}

// NewSimpleCronParser creates a SimpleCronParser.
func NewSimpleCronParser() *SimpleCronParser {
	return &SimpleCronParser{}
}

// Validate checks that the cron expression has 5 fields.
func (p *SimpleCronParser) Validate(cronExpr string) error {
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return fmt.Errorf("cron expression must have exactly 5 fields, got %d", len(fields))
	}
	for _, f := range fields {
		if f == "" {
			return fmt.Errorf("cron fields must not be empty")
		}
	}
	return nil
}

// NextRun computes the next trigger time after `from` for a 5-field cron expression.
// This is a simplified implementation that handles common patterns:
// - */N for minutes and hours
// - Specific values
// - Wildcards (*)
// For full cron semantics, use robfig/cron/v3.
func (p *SimpleCronParser) NextRun(cronExpr string, from time.Time) (time.Time, error) {
	if err := p.Validate(cronExpr); err != nil {
		return time.Time{}, err
	}

	fields := strings.Fields(cronExpr)
	minute := fields[0]
	hour := fields[1]

	// Start from next minute.
	next := from.Truncate(time.Minute).Add(time.Minute)

	// Simple heuristic: try up to 1440 minutes (24h) to find a match.
	for i := 0; i < 1440; i++ {
		if matchField(minute, next.Minute()) && matchField(hour, next.Hour()) {
			return next, nil
		}
		next = next.Add(time.Minute)
	}

	// Fallback: return from + 1 hour.
	return from.Add(time.Hour), nil
}

// matchField checks if a value matches a cron field expression.
func matchField(field string, value int) bool {
	if field == "*" {
		return true
	}

	// */N pattern.
	if strings.HasPrefix(field, "*/") {
		var n int
		if _, err := fmt.Sscanf(field, "*/%d", &n); err == nil && n > 0 {
			return value%n == 0
		}
	}

	// Exact value.
	var exact int
	if _, err := fmt.Sscanf(field, "%d", &exact); err == nil {
		return value == exact
	}

	return false
}
