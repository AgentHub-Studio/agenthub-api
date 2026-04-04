package agentic

import "strings"

// XML/HTML character escaping for safe interpolation.
//
// Inspired by Claude Code's xml.ts — escapes special characters for
// safe use in XML/HTML element text content and attribute values.
// Use when untrusted strings (process output, user input, external
// data) are interpolated into XML or HTML.

// EscapeXML escapes XML/HTML special characters for safe interpolation
// into element text content (between tags). Escapes &, <, >.
func EscapeXML(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}

// EscapeXMLAttr escapes for interpolation into a quoted attribute value.
// Escapes &, <, >, ", and ' in addition to the text content characters.
func EscapeXMLAttr(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

// UnescapeXML reverses EscapeXML, converting XML entities back to
// their original characters.
func UnescapeXML(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&apos;", "'",
	)
	return r.Replace(s)
}
