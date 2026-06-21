// Package sanitize provides input sanitization helpers shared across services.
//
// Bug 179 / Bug 180: stripHTML originally lived inline in agent/service.go
// (P-C280-1). Cross-cutting XSS prevention now needs the same logic in 8+
// entities, so the helper is centralized here.
package sanitize

import (
	"regexp"
	"strings"
)

var htmlDangerousPattern = regexp.MustCompile(`(?is)<(script|style|iframe|object|embed|noscript)[^>]*>.*?</(script|style|iframe|object|embed|noscript)>`)
var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)
var htmlNamedTagPattern = regexp.MustCompile(`(?is)</?[a-z][a-z0-9:-]*(?:\s[^>]*)?/?>`)

// StripHTML removes dangerous HTML elements (scripts, iframes, etc) including
// their text content, then strips all remaining HTML tags but preserves their
// inner text. Trims surrounding whitespace.
//
// Use on user-supplied free-text fields (name, description) that are rendered
// in the UI. Prevents stored XSS when frontend escaping is missing or partial.
func StripHTML(s string) string {
	s = htmlDangerousPattern.ReplaceAllString(s, "")
	s = htmlTagPattern.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// ContainsHTML reports whether s contains an actual HTML tag.
// It intentionally does not treat plain comparison text like "2 < 3" as HTML.
func ContainsHTML(s string) bool {
	return htmlNamedTagPattern.MatchString(s)
}
