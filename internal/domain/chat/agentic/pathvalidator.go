package agentic

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Secure path validation with symlink traversal defense.
//
// Inspired by Claude Code's teamMemPaths.ts — implements multi-layer
// path validation: string-level checks (null bytes, URL-encoded traversals,
// Unicode normalization attacks), then filesystem-level checks (symlink
// resolution). Defense-in-depth prevents directory escape attacks.

// PathTraversalError indicates a path traversal attempt was detected.
type PathTraversalError struct {
	Path   string
	Reason string
}

func (e *PathTraversalError) Error() string {
	return fmt.Sprintf("path traversal blocked: %s (path: %s)", e.Reason, e.Path)
}

// ValidatePath checks if the given path is safely contained within baseDir.
// Returns the cleaned, resolved absolute path on success.
// Performs both string-level and filesystem-level validation.
func ValidatePath(path, baseDir string) (string, error) {
	if path == "" {
		return "", &PathTraversalError{Path: path, Reason: "empty path"}
	}

	// Phase 1: String-level checks
	if err := validatePathString(path); err != nil {
		return "", err
	}

	// Clean and resolve to absolute
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolve base dir: %w", err)
	}

	var absPath string
	if filepath.IsAbs(path) {
		absPath = filepath.Clean(path)
	} else {
		absPath = filepath.Clean(filepath.Join(absBase, path))
	}

	// Phase 2: String-level containment check (pre-realpath)
	if !isContainedIn(absPath, absBase) {
		return "", &PathTraversalError{
			Path:   path,
			Reason: "path escapes base directory",
		}
	}

	// Phase 3: Filesystem-level check (symlink resolution)
	resolvedPath, err := realpathDeepestExisting(absPath)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks: %w", err)
	}

	resolvedBase, err := filepath.EvalSymlinks(absBase)
	if err != nil {
		// If base dir doesn't exist, we can't verify containment
		if os.IsNotExist(err) {
			return "", fmt.Errorf("base directory does not exist: %s", absBase)
		}
		return "", fmt.Errorf("resolve base symlinks: %w", err)
	}

	if !isContainedIn(resolvedPath, resolvedBase) {
		return "", &PathTraversalError{
			Path:   path,
			Reason: "symlink traversal escapes base directory",
		}
	}

	return absPath, nil
}

// ValidateRelativeKey validates a relative key (no absolute paths, no parent traversal).
func ValidateRelativeKey(key string) error {
	if key == "" {
		return &PathTraversalError{Path: key, Reason: "empty key"}
	}

	if err := validatePathString(key); err != nil {
		return err
	}

	if filepath.IsAbs(key) {
		return &PathTraversalError{
			Path:   key,
			Reason: "absolute paths not allowed as keys",
		}
	}

	cleaned := filepath.Clean(key)
	if strings.HasPrefix(cleaned, "..") {
		return &PathTraversalError{
			Path:   key,
			Reason: "parent directory traversal not allowed",
		}
	}

	return nil
}

// validatePathString performs string-level checks for common attack patterns.
func validatePathString(path string) error {
	// Null byte injection
	if strings.ContainsRune(path, '\x00') {
		return &PathTraversalError{Path: path, Reason: "null bytes not allowed"}
	}

	// URL-encoded traversal
	lower := strings.ToLower(path)
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") ||
		strings.Contains(lower, "%00") || strings.Contains(lower, "%2e") {
		return &PathTraversalError{Path: path, Reason: "URL-encoded path components not allowed"}
	}

	// Backslash (Windows path separator in Unix context)
	if strings.Contains(path, "\\") {
		return &PathTraversalError{Path: path, Reason: "backslashes not allowed"}
	}

	return nil
}

// isContainedIn checks if childPath is within parentPath.
// Uses separator-aware prefix check to prevent "/foo/bar-evil" matching "/foo/bar".
func isContainedIn(childPath, parentPath string) bool {
	// Exact match is contained
	if childPath == parentPath {
		return true
	}
	// Must be under parent with separator
	parent := parentPath
	if !strings.HasSuffix(parent, string(filepath.Separator)) {
		parent += string(filepath.Separator)
	}
	return strings.HasPrefix(childPath, parent)
}

// realpathDeepestExisting resolves symlinks on the deepest existing ancestor.
// For paths where the target doesn't exist yet, walks up until a real path
// is found and verifies containment there.
func realpathDeepestExisting(absPath string) (string, error) {
	// Try the full path first
	resolved, err := filepath.EvalSymlinks(absPath)
	if err == nil {
		return resolved, nil
	}

	if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	// Walk up to find the deepest existing ancestor
	current := absPath
	var tail []string

	for {
		parent := filepath.Dir(current)
		if parent == current {
			// Hit filesystem root
			break
		}

		tail = append([]string{filepath.Base(current)}, tail...)
		current = parent

		resolved, err = filepath.EvalSymlinks(current)
		if err == nil {
			// Reconstruct the full path from the resolved ancestor
			parts := append([]string{resolved}, tail...)
			return filepath.Join(parts...), nil
		}

		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
	}

	// Nothing existed — return the cleaned path
	return filepath.Clean(absPath), nil
}

// IsPathTraversalError returns true if err is a PathTraversalError.
func IsPathTraversalError(err error) bool {
	var pte *PathTraversalError
	return errors.As(err, &pte)
}
