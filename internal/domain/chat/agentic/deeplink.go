package agentic

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Deep link URI parsing with security validation.
//
// Inspired by Claude Code's parseDeepLink.ts — parses custom protocol
// URIs with query parameters, applying security checks: ASCII control
// character rejection, absolute path validation, length caps, and
// GitHub slug format validation. Useful for custom protocol handlers
// or API parameter validation with security constraints.

const (
	// DeepLinkProtocol is the custom protocol scheme.
	DeepLinkProtocol = "agenthub"

	maxQueryLength = 5000
	maxCWDLength   = 4096
)

var repoSlugPattern = regexp.MustCompile(`^[\w.\-]+/[\w.\-]+$`)

// DeepLinkAction holds the parsed deep link parameters.
type DeepLinkAction struct {
	Query string // pre-filled prompt (optional)
	CWD   string // working directory (optional, must be absolute)
	Repo  string // owner/name slug (optional)
}

// ParseDeepLink parses an agenthub:// URI into a structured action.
// Returns an error if the URI is malformed or contains dangerous characters.
func ParseDeepLink(uri string) (*DeepLinkAction, error) {
	// Normalize protocol
	prefix := DeepLinkProtocol + "://"
	if !strings.HasPrefix(uri, prefix) {
		altPrefix := DeepLinkProtocol + ":"
		if strings.HasPrefix(uri, altPrefix) {
			uri = prefix + strings.TrimPrefix(uri, altPrefix)
		} else {
			return nil, fmt.Errorf("invalid deep link: expected %s:// scheme, got %q", DeepLinkProtocol, uri)
		}
	}

	parsed, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid deep link URL: %q", uri)
	}

	if parsed.Host != "open" {
		return nil, fmt.Errorf("unknown deep link action: %q", parsed.Host)
	}

	params := parsed.Query()
	cwd := params.Get("cwd")
	repo := params.Get("repo")
	rawQuery := params.Get("q")

	// Validate cwd
	if cwd != "" {
		if !isAbsolutePath(cwd) {
			return nil, fmt.Errorf("invalid cwd in deep link: must be an absolute path, got %q", cwd)
		}
		if ContainsControlChars(cwd) {
			return nil, fmt.Errorf("deep link cwd contains disallowed control characters")
		}
		if len(cwd) > maxCWDLength {
			return nil, fmt.Errorf("deep link cwd exceeds %d characters (got %d)", maxCWDLength, len(cwd))
		}
	}

	// Validate repo slug
	if repo != "" && !repoSlugPattern.MatchString(repo) {
		return nil, fmt.Errorf("invalid repo in deep link: expected \"owner/repo\", got %q", repo)
	}

	// Validate and sanitize query
	var query string
	if rawQuery != "" {
		trimmed := strings.TrimSpace(rawQuery)
		if trimmed != "" {
			sanitized := MustSanitizeUnicode(trimmed)
			if ContainsControlChars(sanitized) {
				return nil, fmt.Errorf("deep link query contains disallowed control characters")
			}
			if len(sanitized) > maxQueryLength {
				return nil, fmt.Errorf("deep link query exceeds %d characters (got %d)", maxQueryLength, len(sanitized))
			}
			query = sanitized
		}
	}

	return &DeepLinkAction{
		Query: query,
		CWD:   cwd,
		Repo:  repo,
	}, nil
}

// BuildDeepLink constructs a deep link URI from an action.
func BuildDeepLink(action DeepLinkAction) string {
	u := &url.URL{
		Scheme: DeepLinkProtocol,
		Host:   "open",
	}
	params := url.Values{}
	if action.Query != "" {
		params.Set("q", action.Query)
	}
	if action.CWD != "" {
		params.Set("cwd", action.CWD)
	}
	if action.Repo != "" {
		params.Set("repo", action.Repo)
	}
	u.RawQuery = params.Encode()
	return u.String()
}

// ContainsControlChars returns true if the string contains ASCII
// control characters (0x00-0x1F, 0x7F) that can act as command
// separators in shells.
func ContainsControlChars(s string) bool {
	for _, r := range s {
		if r <= 0x1F || r == 0x7F {
			return true
		}
	}
	return false
}

// isAbsolutePath checks if a path is absolute (Unix or Windows).
func isAbsolutePath(path string) bool {
	if strings.HasPrefix(path, "/") {
		return true
	}
	// Windows: C:\ or C:/
	if len(path) >= 3 && path[1] == ':' &&
		((path[0] >= 'a' && path[0] <= 'z') || (path[0] >= 'A' && path[0] <= 'Z')) &&
		(path[2] == '/' || path[2] == '\\') {
		return true
	}
	return false
}
