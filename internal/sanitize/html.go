package sanitize

import (
	"regexp"
	"strings"
)

var htmlDangerousPattern = regexp.MustCompile(`(?is)<(script|style|iframe|object|embed|noscript)[^>]*>.*?</(script|style|iframe|object|embed|noscript)>`)
var htmlTagPattern = regexp.MustCompile(`</?[A-Za-z][A-Za-z0-9:-]*(?:\s+[^<>]*)?/?>`)

// ContainsHTML performs best-effort detection of HTML-like markup in user text.
func ContainsHTML(s string) bool {
	return htmlTagPattern.MatchString(s)
}

// StripHTML removes dangerous HTML elements (scripts, iframes, etc) including
// their text content, then strips all remaining HTML tags while preserving
// the inner text.
func StripHTML(s string) string {
	s = strings.ToValidUTF8(s, "")
	for {
		next := htmlDangerousPattern.ReplaceAllString(s, "")
		next = htmlTagPattern.ReplaceAllString(next, "")
		if next == s {
			return strings.TrimSpace(next)
		}
		s = next
	}
}
