package agentic

import (
	"strconv"
	"strings"
)

// Semantic versioning comparison utilities.
//
// Inspired by Claude Code's semver.ts — provides pure-Go semver
// parsing and comparison without external dependencies. Supports
// standard M.m.p format with optional pre-release suffixes for
// ordering. Loose parsing tolerates missing patch/minor versions.

// SemVer represents a parsed semantic version.
type SemVer struct {
	Major      int
	Minor      int
	Patch      int
	PreRelease string // e.g. "alpha.1", "rc.2"
}

// ParseSemVer parses a version string like "1.2.3" or "1.2.3-beta.1".
// Tolerates "v" prefix and missing minor/patch (loose mode).
// Returns nil if unparseable.
func ParseSemVer(version string) *SemVer {
	s := strings.TrimSpace(version)
	s = strings.TrimPrefix(s, "v")
	if s == "" {
		return nil
	}

	// Split off pre-release suffix.
	preRelease := ""
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		preRelease = s[idx+1:]
		s = s[:idx]
		// Strip build metadata from pre-release.
		if bi := strings.IndexByte(preRelease, '+'); bi >= 0 {
			preRelease = preRelease[:bi]
		}
	} else if bi := strings.IndexByte(s, '+'); bi >= 0 {
		s = s[:bi] // strip build metadata
	}

	parts := strings.Split(s, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return nil
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return nil
	}
	minor := 0
	if len(parts) >= 2 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil || minor < 0 {
			return nil
		}
	}
	patch := 0
	if len(parts) >= 3 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil || patch < 0 {
			return nil
		}
	}

	return &SemVer{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		PreRelease: preRelease,
	}
}

// CompareSemVer compares two version strings.
// Returns -1, 0, or 1. Returns 0 if either is unparseable.
func CompareSemVer(a, b string) int {
	va := ParseSemVer(a)
	vb := ParseSemVer(b)
	if va == nil || vb == nil {
		return 0
	}
	return va.Compare(vb)
}

// Compare returns -1, 0, or 1 per semver precedence rules.
// A version with pre-release has LOWER precedence than the
// same version without pre-release (1.0.0-alpha < 1.0.0).
func (v *SemVer) Compare(other *SemVer) int {
	if c := cmpInt(v.Major, other.Major); c != 0 {
		return c
	}
	if c := cmpInt(v.Minor, other.Minor); c != 0 {
		return c
	}
	if c := cmpInt(v.Patch, other.Patch); c != 0 {
		return c
	}
	return comparePreRelease(v.PreRelease, other.PreRelease)
}

// SemVerGT returns true if a > b.
func SemVerGT(a, b string) bool { return CompareSemVer(a, b) == 1 }

// SemVerGTE returns true if a >= b.
func SemVerGTE(a, b string) bool { return CompareSemVer(a, b) >= 0 }

// SemVerLT returns true if a < b.
func SemVerLT(a, b string) bool { return CompareSemVer(a, b) == -1 }

// SemVerLTE returns true if a <= b.
func SemVerLTE(a, b string) bool { return CompareSemVer(a, b) <= 0 }

// SemVerEQ returns true if a == b.
func SemVerEQ(a, b string) bool { return CompareSemVer(a, b) == 0 }

// String returns the canonical string representation.
func (v *SemVer) String() string {
	s := strconv.Itoa(v.Major) + "." + strconv.Itoa(v.Minor) + "." + strconv.Itoa(v.Patch)
	if v.PreRelease != "" {
		s += "-" + v.PreRelease
	}
	return s
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// comparePreRelease implements semver pre-release precedence:
// - No pre-release > has pre-release (1.0.0 > 1.0.0-alpha)
// - Numeric identifiers compared as integers
// - String identifiers compared lexicographically
// - Numeric < string at same position
// - Fewer fields < more fields if all preceding are equal
func comparePreRelease(a, b string) int {
	if a == "" && b == "" {
		return 0
	}
	if a == "" {
		return 1 // no pre-release has higher precedence
	}
	if b == "" {
		return -1
	}

	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	n := len(aParts)
	if len(bParts) < n {
		n = len(bParts)
	}

	for i := 0; i < n; i++ {
		aNum, aErr := strconv.Atoi(aParts[i])
		bNum, bErr := strconv.Atoi(bParts[i])

		switch {
		case aErr == nil && bErr == nil:
			if c := cmpInt(aNum, bNum); c != 0 {
				return c
			}
		case aErr == nil:
			return -1 // numeric < string
		case bErr == nil:
			return 1
		default:
			if aParts[i] < bParts[i] {
				return -1
			}
			if aParts[i] > bParts[i] {
				return 1
			}
		}
	}

	return cmpInt(len(aParts), len(bParts))
}
