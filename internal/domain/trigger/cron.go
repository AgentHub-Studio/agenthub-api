package trigger

import (
	"fmt"
	"strconv"
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

// fieldRange holds the numeric bounds for one cron field.
type fieldRange struct {
	name     string
	min, max int
}

// cronFieldRanges defines the valid range for each of the 5 cron fields.
// Bug 106: prior to this, Validate() only counted fields — accepting `99 * * * *`
// (minute=99) or `abc def ghi jkl mno` (garbage). Both then silently fell back
// to "next run = from + 1h" in NextRun, so triggers persisted but never fired
// on the cadence the user expected.
var cronFieldRanges = []fieldRange{
	{"minute", 0, 59},
	{"hour", 0, 23},
	{"day-of-month", 1, 31},
	{"month", 1, 12},
	{"day-of-week", 0, 6},
}

// Validate parses each field and rejects values outside the field's range or
// using unsupported tokens. Supported token forms per field:
//   - "*"
//   - "*/N" (step), where N >= 1
//   - "N" (literal)
//   - "N-M" (range), where N <= M
//
// Lists ("N,M,O") and step-of-range ("N-M/S") are not supported by this
// minimal parser; they return an explicit error rather than silently
// passing.
func (p *SimpleCronParser) Validate(cronExpr string) error {
	fields := strings.Fields(cronExpr)
	if len(fields) != 5 {
		return fmt.Errorf("cron expression must have exactly 5 fields, got %d", len(fields))
	}
	for i, f := range fields {
		if f == "" {
			return fmt.Errorf("cron fields must not be empty")
		}
		if err := validateCronField(f, cronFieldRanges[i]); err != nil {
			return err
		}
	}
	return nil
}

// validateCronField checks a single cron field token against its allowed range.
func validateCronField(field string, r fieldRange) error {
	if field == "*" {
		return nil
	}
	// "*/N" — step. N must be >= 1; we don't bound it against r.max because
	// `*/120` for minutes would just match no values, but that's the user's
	// problem (semantically empty schedule), not an injection risk.
	if strings.HasPrefix(field, "*/") {
		nStr := field[2:]
		n, err := strconv.Atoi(nStr)
		if err != nil || n < 1 {
			return fmt.Errorf("cron %s: invalid step %q", r.name, field)
		}
		return nil
	}
	// "N-M" — range. Both bounds must be in [r.min, r.max] and N <= M.
	if dash := strings.Index(field, "-"); dash > 0 {
		lo, err1 := strconv.Atoi(field[:dash])
		hi, err2 := strconv.Atoi(field[dash+1:])
		if err1 != nil || err2 != nil {
			return fmt.Errorf("cron %s: invalid range %q", r.name, field)
		}
		if lo < r.min || hi > r.max || lo > hi {
			return fmt.Errorf("cron %s: range %q out of bounds [%d,%d]", r.name, field, r.min, r.max)
		}
		return nil
	}
	// Literal value.
	v, err := strconv.Atoi(field)
	if err != nil {
		return fmt.Errorf("cron %s: invalid value %q", r.name, field)
	}
	if v < r.min || v > r.max {
		return fmt.Errorf("cron %s: %d out of range [%d,%d]", r.name, v, r.min, r.max)
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
