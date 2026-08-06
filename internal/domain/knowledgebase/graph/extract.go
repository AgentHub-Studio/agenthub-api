package graph

import (
	"strings"
	"unicode"
)

// ExtractedEntity is a graph node extracted from document text.
type ExtractedEntity struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ExtractedEdge is a directed relation between two extracted entities.
type ExtractedEdge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Relation string `json:"relation"`
	Evidence string `json:"evidence"`
}

// Snapshot is the extracted knowledge graph for a document.
type Snapshot struct {
	Entities []ExtractedEntity `json:"entities"`
	Edges    []ExtractedEdge   `json:"edges"`
}

// CanonicalName returns the case-insensitive key used to de-duplicate entities.
func CanonicalName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// Extract builds a small deterministic graph from plain text. It covers the
// local Graph RAG contract and gives the async LLM extractor a compatible
// persistence target when it is connected.
func Extract(text string) Snapshot {
	snapshot := Snapshot{}
	entityByKey := map[string]ExtractedEntity{}
	edgeByKey := map[string]bool{}

	for _, sentence := range splitSentences(text) {
		source, target, ok := extractReportsTo(sentence)
		if !ok {
			continue
		}

		for _, name := range []string{source, target} {
			key := CanonicalName(name)
			if key == "" {
				continue
			}
			if _, exists := entityByKey[key]; !exists {
				entityByKey[key] = ExtractedEntity{Name: name, Type: "person"}
				snapshot.Entities = append(snapshot.Entities, entityByKey[key])
			}
		}

		edgeKey := CanonicalName(source) + "|reports_to|" + CanonicalName(target)
		if edgeByKey[edgeKey] {
			continue
		}
		edgeByKey[edgeKey] = true
		snapshot.Edges = append(snapshot.Edges, ExtractedEdge{
			Source:   source,
			Target:   target,
			Relation: "reports_to",
			Evidence: sentence,
		})
	}

	return snapshot
}

func splitSentences(text string) []string {
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?' || r == '\n'
	})
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func extractReportsTo(sentence string) (string, string, bool) {
	patterns := []string{" reporta para ", " se reporta a ", " reports to "}
	for _, pattern := range patterns {
		idx := indexASCIIFold(sentence, pattern)
		if idx < 0 {
			continue
		}
		source := cleanEntityName(sentence[:idx])
		target := cleanEntityName(sentence[idx+len(pattern):])
		if source == "" || target == "" {
			return "", "", false
		}
		return source, target, true
	}
	return "", "", false
}

func indexASCIIFold(s, pattern string) int {
	if pattern == "" {
		return 0
	}
	if len(pattern) > len(s) {
		return -1
	}
	for i := 0; i <= len(s)-len(pattern); i++ {
		matched := true
		for j := 0; j < len(pattern); j++ {
			if asciiLower(s[i+j]) != pattern[j] {
				matched = false
				break
			}
		}
		if matched {
			return i
		}
	}
	return -1
}

func asciiLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

func cleanEntityName(raw string) string {
	raw = strings.TrimSpace(strings.Trim(raw, " \t\r\n,;:()[]{}"))
	words := strings.Fields(raw)
	if len(words) == 0 {
		return ""
	}

	start := 0
	for start < len(words) && isArticle(words[start]) {
		start++
	}
	words = words[start:]
	if len(words) == 0 {
		return ""
	}

	selected := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.Trim(word, " \t\r\n,;:()[]{}")
		if word == "" {
			continue
		}
		if len(selected) > 0 && isNameParticle(word) {
			selected = append(selected, word)
			continue
		}
		if startsUpper(word) {
			selected = append(selected, word)
			continue
		}
		if len(selected) > 0 {
			break
		}
	}
	if len(selected) == 0 {
		return ""
	}
	return strings.Join(selected, " ")
}

func isArticle(word string) bool {
	switch strings.ToLower(strings.Trim(word, " ,;:")) {
	case "a", "o", "as", "os", "the":
		return true
	default:
		return false
	}
}

func isNameParticle(word string) bool {
	switch strings.ToLower(strings.Trim(word, " ,;:")) {
	case "da", "de", "do", "das", "dos", "van", "von":
		return true
	default:
		return false
	}
}

func startsUpper(word string) bool {
	for _, r := range word {
		return unicode.IsUpper(r)
	}
	return false
}
