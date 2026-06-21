package chat_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDSR04MigrationAddsModeAgentCheckConstraint(t *testing.T) {
	up := readSchemaMigration(t, "000070_chat_session_mode_check.up.sql")
	down := readSchemaMigration(t, "000070_chat_session_mode_check.down.sql")

	for _, want := range []string{
		"WHEN agent_id IS NULL THEN 'DYNAMIC_SKILL'",
		"ELSE 'AGENT_FIXED'",
		"ALTER COLUMN mode SET DEFAULT 'AGENT_FIXED'",
		"ADD CONSTRAINT chk_chat_session_mode_agent",
		"(mode = 'AGENT_FIXED' AND agent_id IS NOT NULL)",
		"(mode = 'DYNAMIC_SKILL' AND agent_id IS NULL)",
	} {
		if !strings.Contains(up, want) {
			t.Fatalf("up migration missing %q", want)
		}
	}

	for _, want := range []string{
		"DROP CONSTRAINT IF EXISTS chk_chat_session_mode_agent",
		"ALTER COLUMN mode SET DEFAULT 'AGENT'",
	} {
		if !strings.Contains(down, want) {
			t.Fatalf("down migration missing %q", want)
		}
	}
}

func readSchemaMigration(t *testing.T, name string) string {
	t.Helper()

	path := filepath.Join("..", "..", "..", "migrations", "schemas", name)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migration %s: %v", name, err)
	}
	return string(content)
}
