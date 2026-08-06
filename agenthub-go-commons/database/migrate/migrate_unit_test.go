package migrate

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDiscoverMigrationFilesSortsByVersionAndDirection(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "000010_second.up.sql", "SELECT 10;")
	writeTestFile(t, dir, "000001_first.up.sql", "SELECT 1;")
	writeTestFile(t, dir, "000002_first.down.sql", "SELECT 2;")
	writeTestFile(t, dir, "README.md", "ignore")

	migrations, err := discoverMigrationFiles(dir, "up")

	require.NoError(t, err)
	require.Len(t, migrations, 2)
	require.Equal(t, int64(1), migrations[0].version)
	require.Equal(t, filepath.Join(dir, "000001_first.up.sql"), migrations[0].path)
	require.Equal(t, int64(10), migrations[1].version)
}

func TestDiscoverMigrationFilesRejectsDuplicateVersions(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "000001_first.up.sql", "SELECT 1;")
	writeTestFile(t, dir, "000001_duplicate.up.sql", "SELECT 2;")

	_, err := discoverMigrationFiles(dir, "up")

	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate migration version 1")
}

func TestQuoteIdentifierEscapesDoubleQuotes(t *testing.T) {
	quoted, err := quoteIdentifier(`tenant"schema`)

	require.NoError(t, err)
	require.Equal(t, `"tenant""schema"`, quoted)
}

func TestQuoteIdentifierRejectsEmptyAndNUL(t *testing.T) {
	_, err := quoteIdentifier(" \t ")
	require.Error(t, err)

	_, err = quoteIdentifier("tenant\x00schema")
	require.Error(t, err)
}

func writeTestFile(t *testing.T, dir, name, contents string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(contents), 0o600))
}
