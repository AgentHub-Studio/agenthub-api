package agentic_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- ValidatePath ---

func TestValidatePath_Simple(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	err := os.WriteFile(f, []byte("data"), 0o644)
	require.NoError(t, err)

	path, err := agentic.ValidatePath("file.txt", dir)
	require.NoError(t, err)
	assert.Equal(t, f, path)
}

func TestValidatePath_Absolute(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	err := os.WriteFile(f, []byte("data"), 0o644)
	require.NoError(t, err)

	path, err := agentic.ValidatePath(f, dir)
	require.NoError(t, err)
	assert.Equal(t, f, path)
}

func TestValidatePath_RejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	_, err := agentic.ValidatePath("../../../etc/passwd", dir)
	assert.Error(t, err)
	assert.True(t, agentic.IsPathTraversalError(err))
}

func TestValidatePath_RejectsNullByte(t *testing.T) {
	dir := t.TempDir()
	_, err := agentic.ValidatePath("file\x00.txt", dir)
	assert.Error(t, err)
	assert.True(t, agentic.IsPathTraversalError(err))
}

func TestValidatePath_RejectsURLEncoded(t *testing.T) {
	dir := t.TempDir()
	_, err := agentic.ValidatePath("..%2f..%2fetc/passwd", dir)
	assert.Error(t, err)
	assert.True(t, agentic.IsPathTraversalError(err))
}

func TestValidatePath_RejectsBackslash(t *testing.T) {
	dir := t.TempDir()
	_, err := agentic.ValidatePath("..\\..\\etc\\passwd", dir)
	assert.Error(t, err)
	assert.True(t, agentic.IsPathTraversalError(err))
}

func TestValidatePath_EmptyPath(t *testing.T) {
	dir := t.TempDir()
	_, err := agentic.ValidatePath("", dir)
	assert.Error(t, err)
	assert.True(t, agentic.IsPathTraversalError(err))
}

func TestValidatePath_Symlink(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	secretFile := filepath.Join(outside, "secret.txt")
	err := os.WriteFile(secretFile, []byte("secret"), 0o644)
	require.NoError(t, err)

	// Create symlink inside dir pointing outside
	link := filepath.Join(dir, "evil")
	err = os.Symlink(outside, link)
	require.NoError(t, err)

	_, err = agentic.ValidatePath("evil/secret.txt", dir)
	assert.Error(t, err)
	assert.True(t, agentic.IsPathTraversalError(err))
}

func TestValidatePath_NonExistentFile(t *testing.T) {
	dir := t.TempDir()
	// Non-existent file within the dir is OK
	path, err := agentic.ValidatePath("newfile.txt", dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "newfile.txt"), path)
}

func TestValidatePath_SubdirectoryContainment(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	err := os.MkdirAll(sub, 0o755)
	require.NoError(t, err)
	f := filepath.Join(sub, "file.txt")
	err = os.WriteFile(f, []byte("data"), 0o644)
	require.NoError(t, err)

	path, err := agentic.ValidatePath("sub/file.txt", dir)
	require.NoError(t, err)
	assert.Equal(t, f, path)
}

// --- ValidateRelativeKey ---

func TestValidateRelativeKey_Valid(t *testing.T) {
	err := agentic.ValidateRelativeKey("myfile.md")
	assert.NoError(t, err)
}

func TestValidateRelativeKey_SubPath(t *testing.T) {
	err := agentic.ValidateRelativeKey("sub/file.md")
	assert.NoError(t, err)
}

func TestValidateRelativeKey_RejectsAbsolute(t *testing.T) {
	err := agentic.ValidateRelativeKey("/etc/passwd")
	assert.Error(t, err)
	assert.True(t, agentic.IsPathTraversalError(err))
}

func TestValidateRelativeKey_RejectsParentTraversal(t *testing.T) {
	err := agentic.ValidateRelativeKey("../secret.md")
	assert.Error(t, err)
	assert.True(t, agentic.IsPathTraversalError(err))
}

func TestValidateRelativeKey_RejectsEmpty(t *testing.T) {
	err := agentic.ValidateRelativeKey("")
	assert.Error(t, err)
}

func TestValidateRelativeKey_RejectsNullByte(t *testing.T) {
	err := agentic.ValidateRelativeKey("file\x00.md")
	assert.Error(t, err)
}

// --- IsPathTraversalError ---

func TestIsPathTraversalError(t *testing.T) {
	pte := &agentic.PathTraversalError{Path: "x", Reason: "test"}
	assert.True(t, agentic.IsPathTraversalError(pte))
	assert.False(t, agentic.IsPathTraversalError(assert.AnError))
}

// --- PathTraversalError ---

func TestPathTraversalError_Message(t *testing.T) {
	err := &agentic.PathTraversalError{Path: "../evil", Reason: "traversal detected"}
	assert.Contains(t, err.Error(), "traversal detected")
	assert.Contains(t, err.Error(), "../evil")
}
