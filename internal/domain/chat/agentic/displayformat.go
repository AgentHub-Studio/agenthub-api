package agentic

import (
	"fmt"
	"math"
)

// Display formatting utilities for human-readable output.
//
// Inspired by Claude Code's format.ts — pure display formatters for
// file sizes, durations, and numbers with compact notation. Useful
// for chat UI, logs, and API responses that need human-readable
// representations of metrics.

// FormatFileSize formats a byte count as a human-readable string.
// Examples: 512 → "512 bytes", 1536 → "1.5KB", 2097152 → "2MB"
func FormatFileSize(sizeInBytes int64) string {
	kb := float64(sizeInBytes) / 1024
	if kb < 1 {
		return fmt.Sprintf("%d bytes", sizeInBytes)
	}
	if kb < 1024 {
		return formatCompact(kb) + "KB"
	}
	mb := kb / 1024
	if mb < 1024 {
		return formatCompact(mb) + "MB"
	}
	gb := mb / 1024
	return formatCompact(gb) + "GB"
}

// FormatDuration formats milliseconds as a human-readable duration.
// Examples: 500 → "0s", 1500 → "1s", 65000 → "1m 5s", 3661000 → "1h 1m 1s"
func FormatDuration(ms int64) string {
	if ms < 60000 {
		if ms == 0 {
			return "0s"
		}
		if ms < 1000 {
			return fmt.Sprintf("%.1fs", float64(ms)/1000)
		}
		return fmt.Sprintf("%ds", ms/1000)
	}

	days := ms / 86400000
	hours := (ms % 86400000) / 3600000
	minutes := (ms % 3600000) / 60000
	seconds := int64(math.Round(float64(ms%60000) / 1000))

	// Handle rounding carry-over
	if seconds == 60 {
		seconds = 0
		minutes++
	}
	if minutes == 60 {
		minutes = 0
		hours++
	}
	if hours == 24 {
		hours = 0
		days++
	}

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	return fmt.Sprintf("%dm %ds", minutes, seconds)
}

// FormatDurationCompact formats milliseconds as the most significant
// unit only. Examples: 500 → "0s", 65000 → "1m", 7200000 → "2h"
func FormatDurationCompact(ms int64) string {
	if ms < 60000 {
		return fmt.Sprintf("%ds", ms/1000)
	}
	days := ms / 86400000
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	hours := ms / 3600000
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	minutes := ms / 60000
	return fmt.Sprintf("%dm", minutes)
}

// FormatNumber formats a number with compact notation.
// Examples: 900 → "900", 1321 → "1.3k", 1500000 → "1.5m"
func FormatNumber(n int64) string {
	abs := n
	if abs < 0 {
		abs = -abs
	}

	switch {
	case abs >= 1_000_000_000:
		return fmt.Sprintf("%.1fb", float64(n)/1_000_000_000)
	case abs >= 1_000_000:
		return fmt.Sprintf("%.1fm", float64(n)/1_000_000)
	case abs >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// FormatTokens formats a token count with compact notation, removing
// trailing ".0". Examples: 1000 → "1k", 1500 → "1.5k"
func FormatTokens(count int64) string {
	s := FormatNumber(count)
	// Remove .0 suffix (e.g., "1.0k" → "1k")
	for _, suffix := range []string{".0k", ".0m", ".0b"} {
		if len(s) > len(suffix) && s[len(s)-len(suffix):] == suffix {
			return s[:len(s)-len(suffix)] + s[len(s)-1:]
		}
	}
	return s
}

// formatCompact formats a float with 1 decimal, removing trailing ".0".
func formatCompact(v float64) string {
	s := fmt.Sprintf("%.1f", v)
	if len(s) >= 2 && s[len(s)-2:] == ".0" {
		return s[:len(s)-2]
	}
	return s
}
