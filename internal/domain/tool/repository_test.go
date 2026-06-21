package tool

import (
	"strings"
	"testing"
)

func TestRepositoryUpdateSQLPreservesTypeWhenPatchTypeEmpty(t *testing.T) {
	normalized := strings.Join(strings.Fields(updateToolSQL), " ")
	if !strings.Contains(normalized, "type=COALESCE(NULLIF($3, ''), type)") {
		t.Fatalf("update SQL must preserve existing tool type when the patch type is empty: %s", normalized)
	}
}
