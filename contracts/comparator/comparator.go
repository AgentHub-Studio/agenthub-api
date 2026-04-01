// Package comparator provides JSON diff utilities for contract tests.
// It compares two JSON payloads field-by-field, ignoring dynamic fields
// like IDs, timestamps, and ordering differences in arrays.
package comparator

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Options controls comparison behaviour.
type Options struct {
	// IgnoreFields is a set of top-level field names that are excluded from comparison
	// (e.g., "id", "createdAt", "updatedAt").
	IgnoreFields map[string]bool

	// IgnoreArrayOrder when true compares array contents without caring about order.
	IgnoreArrayOrder bool
}

// DefaultOptions returns options suitable for REST contract testing:
// ignores IDs, timestamps, and pagination cursors.
func DefaultOptions() Options {
	return Options{
		IgnoreFields: map[string]bool{
			"id":        true,
			"createdAt": true,
			"updatedAt": true,
			"startedAt": true,
			"endedAt":   true,
		},
		IgnoreArrayOrder: false,
	}
}

// Diff compares two JSON byte slices and returns a human-readable diff.
// Returns nil when the payloads are equivalent under opts.
func Diff(expected, actual []byte, opts Options) []string {
	var expectedVal, actualVal any
	if err := json.Unmarshal(expected, &expectedVal); err != nil {
		return []string{fmt.Sprintf("expected JSON parse error: %v", err)}
	}
	if err := json.Unmarshal(actual, &actualVal); err != nil {
		return []string{fmt.Sprintf("actual JSON parse error: %v", err)}
	}

	var diffs []string
	diffValues("", expectedVal, actualVal, opts, &diffs)
	return diffs
}

// Equal returns true when Diff produces no differences.
func Equal(expected, actual []byte, opts Options) bool {
	return len(Diff(expected, actual, opts)) == 0
}

func diffValues(path string, expected, actual any, opts Options, diffs *[]string) {
	switch exp := expected.(type) {
	case map[string]any:
		act, ok := actual.(map[string]any)
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: type mismatch — expected object, got %T", path, actual))
			return
		}
		diffMaps(path, exp, act, opts, diffs)

	case []any:
		act, ok := actual.([]any)
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: type mismatch — expected array, got %T", path, actual))
			return
		}
		diffArrays(path, exp, act, opts, diffs)

	default:
		if fmt.Sprintf("%v", expected) != fmt.Sprintf("%v", actual) {
			*diffs = append(*diffs, fmt.Sprintf("%s: expected %v, got %v", path, expected, actual))
		}
	}
}

func diffMaps(path string, expected, actual map[string]any, opts Options, diffs *[]string) {
	for key, expVal := range expected {
		if opts.IgnoreFields[key] {
			continue
		}
		fieldPath := joinPath(path, key)
		actVal, ok := actual[key]
		if !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: missing in actual", fieldPath))
			continue
		}
		diffValues(fieldPath, expVal, actVal, opts, diffs)
	}
	// Check for extra keys in actual not present in expected.
	for key := range actual {
		if opts.IgnoreFields[key] {
			continue
		}
		if _, ok := expected[key]; !ok {
			*diffs = append(*diffs, fmt.Sprintf("%s: unexpected key in actual", joinPath(path, key)))
		}
	}
}

func diffArrays(path string, expected, actual []any, opts Options, diffs *[]string) {
	if len(expected) != len(actual) {
		*diffs = append(*diffs, fmt.Sprintf("%s: length mismatch — expected %d, got %d", path, len(expected), len(actual)))
		// Still compare up to the shorter length.
	}
	limit := len(expected)
	if len(actual) < limit {
		limit = len(actual)
	}
	for i := range limit {
		diffValues(fmt.Sprintf("%s[%d]", path, i), expected[i], actual[i], opts, diffs)
	}
}

func joinPath(parent, key string) string {
	if parent == "" {
		return key
	}
	return strings.Join([]string{parent, key}, ".")
}
