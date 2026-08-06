package tool

import (
	"strings"
	"testing"
)

func TestUpdateToolSQLPreservesExistingTypeWhenPatchOmitsType(t *testing.T) {
	normalized := strings.Join(strings.Fields(updateToolSQL), " ")

	if !strings.Contains(normalized, "type=COALESCE(NULLIF($3, ''), type)") {
		t.Fatalf("update query must preserve existing type when the repository receives an empty type: %s", normalized)
	}
	if strings.Contains(normalized, " type=$3,") {
		t.Fatalf("update query must not write an empty type directly: %s", normalized)
	}
}
