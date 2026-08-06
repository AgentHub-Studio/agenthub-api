package chat_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionSnapshotMigrationDefinesCanonicalRT01Columns(t *testing.T) {
	sql := readSchemaUpMigrations(t)
	normalizedSQL := regexp.MustCompile(`\s+`).ReplaceAllString(strings.ToLower(sql), " ")

	require.Contains(t, normalizedSQL, "chat_session", "RT-01 snapshot columns must belong to chat_session migrations")
	require.Contains(t, normalizedSQL, "agent_snapshot jsonb", "RT-01 requires chat_session.agent_snapshot JSONB")
	require.Contains(t, normalizedSQL, "agent_snapshot_hash varchar(64)", "RT-01 requires chat_session.agent_snapshot_hash VARCHAR(64)")
}

func readSchemaUpMigrations(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))

	files, err := filepath.Glob(filepath.Join(repoRoot, "migrations", "schemas", "*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files)

	var builder strings.Builder
	for _, path := range files {
		content, err := os.ReadFile(path)
		require.NoError(t, err)
		builder.Write(content)
		builder.WriteByte('\n')
	}
	return builder.String()
}
