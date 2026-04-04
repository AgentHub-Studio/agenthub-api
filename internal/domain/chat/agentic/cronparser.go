package agentic

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Minimal cron expression parsing and next-run calculation.
//
// Inspired by Claude Code's cron.ts — supports the standard 5-field
// cron subset: minute hour day-of-month month day-of-week.
// Field syntax: wildcard, N, step (*/N), range (N-M), list (N,M,...).
// No L, W, ?, or name aliases. All times are in the given location.

// CronFields holds expanded values for each cron field.
type CronFields struct {
	Minute     []int
	Hour       []int
	DayOfMonth []int
	Month      []int
	DayOfWeek  []int
}

type fieldRange struct {
	min, max int
}

var cronFieldRanges = []fieldRange{
	{0, 59},  // minute
	{0, 23},  // hour
	{1, 31},  // dayOfMonth
	{1, 12},  // month
	{0, 6},   // dayOfWeek (0=Sunday; 7 accepted as Sunday alias)
}

// expandCronField parses a single cron field into sorted values.
func expandCronField(field string, r fieldRange) ([]int, error) {
	out := make(map[int]struct{})
	isDow := r.min == 0 && r.max == 6

	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)

		// wildcard or */N
		if strings.HasPrefix(part, "*") {
			step := 1
			if strings.Contains(part, "/") {
				rest := strings.TrimPrefix(part, "*/")
				s, err := strconv.Atoi(rest)
				if err != nil || s < 1 {
					return nil, fmt.Errorf("invalid step in %q", part)
				}
				step = s
			}
			for i := r.min; i <= r.max; i += step {
				out[i] = struct{}{}
			}
			continue
		}

		// N-M or N-M/S
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "/", 2)
			bounds := strings.SplitN(rangeParts[0], "-", 2)
			if len(bounds) != 2 {
				return nil, fmt.Errorf("invalid range %q", part)
			}
			lo, err1 := strconv.Atoi(bounds[0])
			hi, err2 := strconv.Atoi(bounds[1])
			if err1 != nil || err2 != nil {
				return nil, fmt.Errorf("invalid range %q", part)
			}
			step := 1
			if len(rangeParts) == 2 {
				s, err := strconv.Atoi(rangeParts[1])
				if err != nil || s < 1 {
					return nil, fmt.Errorf("invalid step in %q", part)
				}
				step = s
			}
			effMax := r.max
			if isDow {
				effMax = 7
			}
			if lo > hi || lo < r.min || hi > effMax {
				return nil, fmt.Errorf("range out of bounds %q", part)
			}
			for i := lo; i <= hi; i += step {
				v := i
				if isDow && v == 7 {
					v = 0
				}
				out[v] = struct{}{}
			}
			continue
		}

		// plain N
		n, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid value %q", part)
		}
		if isDow && n == 7 {
			n = 0
		}
		if n < r.min || n > r.max {
			return nil, fmt.Errorf("value %d out of range [%d,%d]", n, r.min, r.max)
		}
		out[n] = struct{}{}
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("empty field")
	}

	result := make([]int, 0, len(out))
	for v := range out {
		result = append(result, v)
	}
	sort.Ints(result)
	return result, nil
}

// ParseCronExpression parses a 5-field cron expression into expanded
// number arrays. Returns an error if the expression is invalid.
func ParseCronExpression(expr string) (*CronFields, error) {
	parts := strings.Fields(strings.TrimSpace(expr))
	if len(parts) != 5 {
		return nil, fmt.Errorf("expected 5 fields, got %d", len(parts))
	}

	expanded := make([][]int, 5)
	for i := 0; i < 5; i++ {
		vals, err := expandCronField(parts[i], cronFieldRanges[i])
		if err != nil {
			return nil, fmt.Errorf("field %d (%q): %w", i, parts[i], err)
		}
		expanded[i] = vals
	}

	return &CronFields{
		Minute:     expanded[0],
		Hour:       expanded[1],
		DayOfMonth: expanded[2],
		Month:      expanded[3],
		DayOfWeek:  expanded[4],
	}, nil
}

// ComputeNextCronRun returns the next time strictly after from that matches
// the cron fields. Uses the given location for timezone. Returns zero time
// if no match within 366 days (should not happen for valid cron).
func ComputeNextCronRun(fields *CronFields, from time.Time, loc *time.Location) time.Time {
	minuteSet := intSet(fields.Minute)
	hourSet := intSet(fields.Hour)
	domSet := intSet(fields.DayOfMonth)
	monthSet := intSet(fields.Month)
	dowSet := intSet(fields.DayOfWeek)

	domWild := len(fields.DayOfMonth) == 31
	dowWild := len(fields.DayOfWeek) == 7

	// Round up to next whole minute
	t := from.In(loc)
	t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute()+1, 0, 0, loc)

	maxIter := 366 * 24 * 60
	for i := 0; i < maxIter; i++ {
		month := int(t.Month())
		if !monthSet[month] {
			t = time.Date(t.Year(), t.Month()+1, 1, 0, 0, 0, 0, loc)
			continue
		}

		dom := t.Day()
		dow := int(t.Weekday())
		var dayMatches bool
		if domWild && dowWild {
			dayMatches = true
		} else if domWild {
			dayMatches = dowSet[dow]
		} else if dowWild {
			dayMatches = domSet[dom]
		} else {
			dayMatches = domSet[dom] || dowSet[dow]
		}

		if !dayMatches {
			t = time.Date(t.Year(), t.Month(), t.Day()+1, 0, 0, 0, 0, loc)
			continue
		}

		if !hourSet[t.Hour()] {
			t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour()+1, 0, 0, 0, loc)
			continue
		}

		if !minuteSet[t.Minute()] {
			t = t.Add(time.Minute)
			continue
		}

		return t
	}

	return time.Time{}
}

func intSet(vals []int) map[int]bool {
	m := make(map[int]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

// CronToHuman converts a cron expression to a human-readable string.
// Covers common patterns; falls back to the raw expression for complex ones.
func CronToHuman(cron string) string {
	parts := strings.Fields(strings.TrimSpace(cron))
	if len(parts) != 5 {
		return cron
	}
	minute, hour, dom, month, dow := parts[0], parts[1], parts[2], parts[3], parts[4]

	dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}

	// Every N minutes: */N * * * *
	if strings.HasPrefix(minute, "*/") && hour == "*" && dom == "*" && month == "*" && dow == "*" {
		n, err := strconv.Atoi(strings.TrimPrefix(minute, "*/"))
		if err == nil {
			if n == 1 {
				return "Every minute"
			}
			return fmt.Sprintf("Every %d minutes", n)
		}
	}

	// Every hour: N * * * *
	if isDigits(minute) && hour == "*" && dom == "*" && month == "*" && dow == "*" {
		m, _ := strconv.Atoi(minute)
		if m == 0 {
			return "Every hour"
		}
		return fmt.Sprintf("Every hour at :%02d", m)
	}

	// Every N hours: N */H * * *
	if isDigits(minute) && strings.HasPrefix(hour, "*/") && dom == "*" && month == "*" && dow == "*" {
		n, err := strconv.Atoi(strings.TrimPrefix(hour, "*/"))
		m, _ := strconv.Atoi(minute)
		if err == nil {
			suffix := ""
			if m != 0 {
				suffix = fmt.Sprintf(" at :%02d", m)
			}
			if n == 1 {
				return "Every hour" + suffix
			}
			return fmt.Sprintf("Every %d hours%s", n, suffix)
		}
	}

	if !isDigits(minute) || !isDigits(hour) {
		return cron
	}
	m, _ := strconv.Atoi(minute)
	h, _ := strconv.Atoi(hour)
	timeStr := formatTimeHM(h, m)

	// Daily: M H * * *
	if dom == "*" && month == "*" && dow == "*" {
		return "Every day at " + timeStr
	}

	// Specific day of week: M H * * D
	if dom == "*" && month == "*" && isDigits(dow) {
		d, _ := strconv.Atoi(dow)
		d = d % 7
		if d >= 0 && d < 7 {
			return fmt.Sprintf("Every %s at %s", dayNames[d], timeStr)
		}
	}

	// Weekdays: M H * * 1-5
	if dom == "*" && month == "*" && dow == "1-5" {
		return "Weekdays at " + timeStr
	}

	return cron
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func formatTimeHM(hour, minute int) string {
	period := "AM"
	h := hour
	if h >= 12 {
		period = "PM"
	}
	if h == 0 {
		h = 12
	} else if h > 12 {
		h -= 12
	}
	return fmt.Sprintf("%d:%02d %s", h, minute, period)
}
