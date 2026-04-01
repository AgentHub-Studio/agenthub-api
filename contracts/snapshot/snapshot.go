// Package snapshot implements structural snapshot testing for REST API contracts.
// It compares JSON response structure (not values) against saved baseline snapshots.
package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TypeKind represents the JSON type of a value.
type TypeKind string

const (
	TypeString  TypeKind = "string"
	TypeNumber  TypeKind = "number"
	TypeBoolean TypeKind = "boolean"
	TypeArray   TypeKind = "array"
	TypeObject  TypeKind = "object"
	TypeNull    TypeKind = "null"
)

// Schema represents the structural schema of a JSON value.
type Schema map[string]TypeKind

// ExtractSchema extracts the structural schema from a JSON byte slice.
// It records field names and their JSON type kinds, recursively for objects.
func ExtractSchema(data []byte) (map[string]any, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("snapshot: unmarshal: %w", err)
	}
	return extractValue("", v), nil
}

func extractValue(prefix string, v any) map[string]any {
	result := make(map[string]any)
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			result[path] = typeKindOf(child)
			if obj, ok := child.(map[string]any); ok {
				for subPath, subKind := range extractValue(path, obj) {
					result[subPath] = subKind
				}
			}
		}
	}
	return result
}

func typeKindOf(v any) TypeKind {
	switch v.(type) {
	case string:
		return TypeString
	case float64:
		return TypeNumber
	case bool:
		return TypeBoolean
	case []any:
		return TypeArray
	case map[string]any:
		return TypeObject
	case nil:
		return TypeNull
	default:
		return TypeString
	}
}

// SaveDir is the directory where snapshots are stored.
const SaveDir = "testdata/snapshots"

// Save writes a schema snapshot to disk.
func Save(t *testing.T, name string, data []byte) {
	t.Helper()
	schema, err := ExtractSchema(data)
	if err != nil {
		t.Fatalf("snapshot.Save: %v", err)
	}
	if err := os.MkdirAll(SaveDir, 0755); err != nil {
		t.Fatalf("snapshot.Save: mkdir: %v", err)
	}
	path := filepath.Join(SaveDir, sanitizeName(name)+".json")
	out, _ := json.MarshalIndent(schema, "", "  ")
	if err := os.WriteFile(path, out, 0644); err != nil {
		t.Fatalf("snapshot.Save: write: %v", err)
	}
	t.Logf("snapshot saved: %s", path)
}

// Assert compares a JSON response against a saved snapshot.
// If the snapshot doesn't exist, it creates it and the test passes.
func Assert(t *testing.T, name string, data []byte) {
	t.Helper()
	path := filepath.Join(SaveDir, sanitizeName(name)+".json")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Logf("snapshot not found, creating: %s", path)
		Save(t, name, data)
		return
	}

	savedData, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("snapshot.Assert: read snapshot: %v", err)
	}

	var saved map[string]any
	if err := json.Unmarshal(savedData, &saved); err != nil {
		t.Fatalf("snapshot.Assert: parse snapshot: %v", err)
	}

	actual, err := ExtractSchema(data)
	if err != nil {
		t.Fatalf("snapshot.Assert: extract schema: %v", err)
	}

	var diffs []string

	// Check for missing fields (in snapshot but not in actual)
	for field, expectedType := range saved {
		actualType, ok := actual[field]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("MISSING field %q (expected type: %s)", field, expectedType))
			continue
		}
		if actualType != expectedType {
			diffs = append(diffs, fmt.Sprintf("TYPE MISMATCH %q: expected %s, got %s", field, expectedType, actualType))
		}
	}

	// Check for extra fields (in actual but not in snapshot)
	for field := range actual {
		if _, ok := saved[field]; !ok {
			diffs = append(diffs, fmt.Sprintf("EXTRA field %q (type: %s)", field, actual[field]))
		}
	}

	if len(diffs) > 0 {
		t.Errorf("contract regression for %q:\n%s", name, strings.Join(diffs, "\n"))
	}
}

func sanitizeName(name string) string {
	replacer := strings.NewReplacer("/", "_", " ", "_", ":", "_")
	return replacer.Replace(name)
}
