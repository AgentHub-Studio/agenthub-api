package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestExtractGlobBaseDirectory_NoGlob(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("/home/user/file.txt")
	assert.Equal(t, "/home/user", result.BaseDir)
	assert.Equal(t, "file.txt", result.RelativePattern)
}

func TestExtractGlobBaseDirectory_StarInFilename(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("/home/user/*.txt")
	assert.Equal(t, "/home/user", result.BaseDir)
	assert.Equal(t, "*.txt", result.RelativePattern)
}

func TestExtractGlobBaseDirectory_DoubleStarDeep(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("/home/user/**/*.go")
	assert.Equal(t, "/home/user", result.BaseDir)
	assert.Equal(t, "**/*.go", result.RelativePattern)
}

func TestExtractGlobBaseDirectory_QuestionMark(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("/src/file?.ts")
	assert.Equal(t, "/src", result.BaseDir)
	assert.Equal(t, "file?.ts", result.RelativePattern)
}

func TestExtractGlobBaseDirectory_BraceExpansion(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("/src/*.{ts,tsx}")
	assert.Equal(t, "/src", result.BaseDir)
	assert.Equal(t, "*.{ts,tsx}", result.RelativePattern)
}

func TestExtractGlobBaseDirectory_BracketRange(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("/logs/app[0-9].log")
	assert.Equal(t, "/logs", result.BaseDir)
	assert.Equal(t, "app[0-9].log", result.RelativePattern)
}

func TestExtractGlobBaseDirectory_RelativeNoSep(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("*.go")
	assert.Equal(t, "", result.BaseDir)
	assert.Equal(t, "*.go", result.RelativePattern)
}

func TestExtractGlobBaseDirectory_RelativeWithDir(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("src/utils/*.ts")
	assert.Equal(t, "src/utils", result.BaseDir)
	assert.Equal(t, "*.ts", result.RelativePattern)
}

func TestExtractGlobBaseDirectory_RootGlob(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("/*.txt")
	assert.Equal(t, "/", result.BaseDir)
	assert.Equal(t, "*.txt", result.RelativePattern)
}

func TestExtractGlobBaseDirectory_GlobInMiddleDir(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("/home/*/projects/*.go")
	assert.Equal(t, "/home", result.BaseDir)
	assert.Equal(t, "*/projects/*.go", result.RelativePattern)
}

func TestExtractGlobBaseDirectory_DeepStaticPrefix(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("/a/b/c/d/**/*.ts")
	assert.Equal(t, "/a/b/c/d", result.BaseDir)
	assert.Equal(t, "**/*.ts", result.RelativePattern)
}

func TestExtractGlobBaseDirectory_NoGlobRelative(t *testing.T) {
	result := agentic.ExtractGlobBaseDirectory("README.md")
	assert.Equal(t, ".", result.BaseDir)
	assert.Equal(t, "README.md", result.RelativePattern)
}
