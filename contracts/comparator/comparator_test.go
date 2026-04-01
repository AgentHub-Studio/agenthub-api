package comparator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiff_Equal(t *testing.T) {
	a := []byte(`{"name":"test","status":"ACTIVE"}`)
	b := []byte(`{"name":"test","status":"ACTIVE"}`)
	diffs := Diff(a, b, DefaultOptions())
	assert.Empty(t, diffs)
}

func TestDiff_IgnoresTimestamps(t *testing.T) {
	a := []byte(`{"name":"test","createdAt":"2025-01-01T00:00:00Z"}`)
	b := []byte(`{"name":"test","createdAt":"2025-06-01T12:00:00Z"}`)
	diffs := Diff(a, b, DefaultOptions())
	assert.Empty(t, diffs, "timestamps should be ignored")
}

func TestDiff_IgnoresIDs(t *testing.T) {
	a := []byte(`{"id":"uuid-a","name":"test"}`)
	b := []byte(`{"id":"uuid-b","name":"test"}`)
	diffs := Diff(a, b, DefaultOptions())
	assert.Empty(t, diffs, "ids should be ignored")
}

func TestDiff_DetectsFieldDifference(t *testing.T) {
	a := []byte(`{"name":"expected","status":"ACTIVE"}`)
	b := []byte(`{"name":"different","status":"ACTIVE"}`)
	diffs := Diff(a, b, DefaultOptions())
	assert.NotEmpty(t, diffs)
	assert.Contains(t, diffs[0], "name")
}

func TestDiff_DetectsMissingField(t *testing.T) {
	a := []byte(`{"name":"test","extra":"field"}`)
	b := []byte(`{"name":"test"}`)
	diffs := Diff(a, b, DefaultOptions())
	assert.NotEmpty(t, diffs)
}

func TestDiff_NestedObjects(t *testing.T) {
	a := []byte(`{"data":{"key":"value"}}`)
	b := []byte(`{"data":{"key":"value"}}`)
	diffs := Diff(a, b, DefaultOptions())
	assert.Empty(t, diffs)
}

func TestDiff_NestedObjectDifference(t *testing.T) {
	a := []byte(`{"data":{"key":"value1"}}`)
	b := []byte(`{"data":{"key":"value2"}}`)
	diffs := Diff(a, b, DefaultOptions())
	assert.NotEmpty(t, diffs)
	assert.Contains(t, diffs[0], "data.key")
}

func TestDiff_ArrayLengthMismatch(t *testing.T) {
	a := []byte(`{"items":[1,2,3]}`)
	b := []byte(`{"items":[1,2]}`)
	diffs := Diff(a, b, DefaultOptions())
	assert.NotEmpty(t, diffs)
}

func TestEqual(t *testing.T) {
	a := []byte(`{"name":"test"}`)
	b := []byte(`{"name":"test"}`)
	assert.True(t, Equal(a, b, DefaultOptions()))
}
