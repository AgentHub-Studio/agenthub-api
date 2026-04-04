package agentic

import (
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

// Glob pattern base directory extraction.
//
// Inspired by Claude Code's glob.ts — extracts the static base
// directory from a glob pattern (everything before the first glob
// special character), returning the directory and the remaining
// relative pattern. Useful for converting absolute glob patterns
// to a base directory + relative pattern for tools like ripgrep.

var globCharsPattern = regexp.MustCompile(`[*?\[{]`)

// GlobBaseDir holds the result of extracting a glob's base directory.
type GlobBaseDir struct {
	BaseDir         string
	RelativePattern string
}

// ExtractGlobBaseDirectory extracts the static base directory from a
// glob pattern. The base directory is everything before the first
// glob special character (*, ?, [, {). If the pattern has no glob
// characters, the directory portion and filename are returned.
func ExtractGlobBaseDirectory(pattern string) GlobBaseDir {
	loc := globCharsPattern.FindStringIndex(pattern)

	if loc == nil {
		// No glob characters — literal path.
		dir := filepath.Dir(pattern)
		file := filepath.Base(pattern)
		return GlobBaseDir{BaseDir: dir, RelativePattern: file}
	}

	staticPrefix := pattern[:loc[0]]

	// Find the last path separator in the static prefix.
	lastSlash := strings.LastIndex(staticPrefix, "/")
	lastBackslash := strings.LastIndex(staticPrefix, `\`)
	lastSep := lastSlash
	if lastBackslash > lastSep {
		lastSep = lastBackslash
	}

	if lastSep == -1 {
		return GlobBaseDir{BaseDir: "", RelativePattern: pattern}
	}

	baseDir := staticPrefix[:lastSep]
	relativePattern := pattern[lastSep+1:]

	// Handle root directory patterns (e.g., /*.txt on Unix).
	if baseDir == "" && lastSep == 0 {
		baseDir = "/"
	}

	// Handle Windows drive root paths (e.g., C:/*.txt).
	if runtime.GOOS == "windows" && len(baseDir) == 2 &&
		baseDir[1] == ':' &&
		((baseDir[0] >= 'a' && baseDir[0] <= 'z') ||
			(baseDir[0] >= 'A' && baseDir[0] <= 'Z')) {
		baseDir = baseDir + string(filepath.Separator)
	}

	return GlobBaseDir{BaseDir: baseDir, RelativePattern: relativePattern}
}
