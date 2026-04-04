package agentic

import (
	"fmt"
	"time"
)

// Brief timestamp formatting for chat message labels.
//
// Inspired by Claude Code's formatBriefTimestamp.ts — display scales
// with age (like a messaging app):
//   - same day:      "13:30"
//   - within 6 days: "Sunday, 13:30"
//   - older:         "Sunday, Feb 20, 13:30"

// FormatBriefTimestamp formats a time for display in a chat message
// label. The display detail scales with age relative to now:
//   - same day: time only ("15:04")
//   - within 6 days: weekday + time ("Monday, 15:04")
//   - older: weekday + month + day + time ("Monday, Jan 2, 15:04")
//
// The now parameter is injectable for testability.
func FormatBriefTimestamp(t time.Time, now time.Time) string {
	if t.IsZero() {
		return ""
	}

	daysAgo := daysBetween(now, t)

	if daysAgo == 0 {
		return t.Format("15:04")
	}

	if daysAgo > 0 && daysAgo < 7 {
		return t.Format("Monday, 15:04")
	}

	return t.Format("Monday, Jan 2, 15:04")
}

// FormatBriefTimestampFromISO parses an ISO 8601 string and formats it.
// Returns empty string for invalid input.
func FormatBriefTimestampFromISO(isoString string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, isoString)
	if err != nil {
		// Try without timezone
		t, err = time.Parse("2006-01-02T15:04:05", isoString)
		if err != nil {
			return ""
		}
	}
	return FormatBriefTimestamp(t, now)
}

// FormatRelativeTime formats a duration as a human-readable relative
// time string (e.g., "just now", "5 minutes ago", "2 hours ago",
// "3 days ago", "2 months ago").
func FormatRelativeTime(t time.Time, now time.Time) string {
	diff := now.Sub(t)

	if diff < 0 {
		return "in the future"
	}

	seconds := int(diff.Seconds())
	minutes := int(diff.Minutes())
	hours := int(diff.Hours())
	days := hours / 24

	switch {
	case seconds < 60:
		return "just now"
	case minutes == 1:
		return "1 minute ago"
	case minutes < 60:
		return fmt.Sprintf("%d minutes ago", minutes)
	case hours == 1:
		return "1 hour ago"
	case hours < 24:
		return fmt.Sprintf("%d hours ago", hours)
	case days == 1:
		return "yesterday"
	case days < 30:
		return fmt.Sprintf("%d days ago", days)
	case days < 365:
		months := days / 30
		if months == 1 {
			return "1 month ago"
		}
		return fmt.Sprintf("%d months ago", months)
	default:
		years := days / 365
		if years == 1 {
			return "1 year ago"
		}
		return fmt.Sprintf("%d years ago", years)
	}
}

// daysBetween returns the number of calendar days between now and t.
// Returns 0 if same day, positive if t is in the past.
func daysBetween(now, t time.Time) int {
	nowDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tDate := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	diff := nowDate.Sub(tDate)
	days := int(diff.Hours() / 24)
	if days < 0 {
		return -days
	}
	return days
}
