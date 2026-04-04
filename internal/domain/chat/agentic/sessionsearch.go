package agentic

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// AI-powered session semantic search.
//
// Inspired by Claude Code's agenticSessionSearch.ts — uses a small LLM
// to semantically search past sessions. Constructs session summaries
// and a ranked matching prompt for relevance ordering.

// Session search configuration.
const (
	// MaxMessagesToScan is the maximum number of messages to extract per session.
	MaxMessagesToScan = 100
	// MaxSessionsToSearch is the maximum number of sessions to consider.
	MaxSessionsToSearch = 100
	// MaxExcerptChars is the maximum character count for a session excerpt.
	MaxExcerptChars = 2000
)

// SessionSearchEntry represents a session prepared for search.
type SessionSearchEntry struct {
	// Index is the position in the search batch (used for result mapping).
	Index int `json:"index"`
	// SessionID is the unique session identifier.
	SessionID string `json:"sessionId"`
	// Title is the session title or first user message.
	Title string `json:"title,omitempty"`
	// Tags are metadata tags associated with the session.
	Tags []string `json:"tags,omitempty"`
	// Summary is a brief session summary.
	Summary string `json:"summary,omitempty"`
	// Excerpt is the sampled conversation text.
	Excerpt string `json:"excerpt"`
	// CreatedAt is when the session was created.
	CreatedAt time.Time `json:"createdAt"`
	// MessageCount is the total number of messages.
	MessageCount int `json:"messageCount"`
}

// SessionSearchResult holds the ranked search results.
type SessionSearchResult struct {
	// Indices are the session indices in relevance order.
	Indices []int `json:"indices"`
	// Reasoning is the LLM's explanation of the ranking.
	Reasoning string `json:"reasoning,omitempty"`
}

// BuildSessionSearchPrompt constructs the system prompt for session search.
//
// The prompt instructs the LLM to rank sessions by:
// 1. Exact tag match (highest priority)
// 2. Title match
// 3. Semantic content match (lowest priority)
func BuildSessionSearchPrompt() string {
	return `You are a session search engine. Given a user query and a list of session summaries, rank the sessions by relevance.

Ranking priority (highest to lowest):
1. Exact tag match — if the query matches a session tag exactly
2. Title match — if the query words appear in the session title
3. Semantic match — if the session content is semantically related to the query

Rules:
- Return too many results rather than too few — err on the side of inclusion
- Only exclude sessions that are clearly irrelevant
- Return a JSON object with "indices" (array of session indices in relevance order) and "reasoning" (brief explanation)
- If no sessions match, return empty indices array

Response format:
{"indices": [3, 1, 7], "reasoning": "Session 3 has exact tag match, session 1 title matches, session 7 has related content"}`
}

// BuildSessionSearchUserPrompt constructs the user prompt with the query and sessions.
func BuildSessionSearchUserPrompt(query string, sessions []SessionSearchEntry) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Query: %s\n\nSessions:\n", query))

	for _, s := range sessions {
		b.WriteString(fmt.Sprintf("\n--- Session %d ---\n", s.Index))
		if s.Title != "" {
			b.WriteString(fmt.Sprintf("Title: %s\n", s.Title))
		}
		if len(s.Tags) > 0 {
			b.WriteString(fmt.Sprintf("Tags: %s\n", strings.Join(s.Tags, ", ")))
		}
		if s.Summary != "" {
			b.WriteString(fmt.Sprintf("Summary: %s\n", s.Summary))
		}
		b.WriteString(fmt.Sprintf("Messages: %d | Created: %s\n", s.MessageCount, s.CreatedAt.Format("2006-01-02")))
		if s.Excerpt != "" {
			b.WriteString(fmt.Sprintf("Excerpt:\n%s\n", s.Excerpt))
		}
	}

	return b.String()
}

// ExtractSessionExcerpt creates a condensed excerpt from conversation messages.
// Takes the first N and last N messages, truncating to MaxExcerptChars.
func ExtractSessionExcerpt(messages json.RawMessage, headCount, tailCount int) string {
	if len(messages) == 0 {
		return ""
	}

	var parsed []map[string]interface{}
	if err := json.Unmarshal(messages, &parsed); err != nil {
		return ""
	}

	if headCount <= 0 {
		headCount = 5
	}
	if tailCount <= 0 {
		tailCount = 5
	}

	var parts []string

	// Head messages.
	headEnd := headCount
	if headEnd > len(parsed) {
		headEnd = len(parsed)
	}
	for i := 0; i < headEnd; i++ {
		line := formatSearchMessage(parsed[i])
		if line != "" {
			parts = append(parts, line)
		}
	}

	// Gap indicator.
	if len(parsed) > headCount+tailCount {
		parts = append(parts, fmt.Sprintf("... (%d messages omitted) ...", len(parsed)-headCount-tailCount))
	}

	// Tail messages.
	if len(parsed) > headCount+tailCount {
		tailStart := len(parsed) - tailCount
		for i := tailStart; i < len(parsed); i++ {
			line := formatSearchMessage(parsed[i])
			if line != "" {
				parts = append(parts, line)
			}
		}
	}

	excerpt := strings.Join(parts, "\n")

	// Truncate to MaxExcerptChars.
	if utf8.RuneCountInString(excerpt) > MaxExcerptChars {
		runes := []rune(excerpt)
		excerpt = string(runes[:MaxExcerptChars]) + "..."
	}

	return excerpt
}

// formatSearchMessage formats a single message for the search excerpt.
func formatSearchMessage(msg map[string]interface{}) string {
	role, _ := msg["role"].(string)
	content, _ := msg["content"].(string)

	if content == "" {
		return ""
	}

	// Truncate very long messages.
	if utf8.RuneCountInString(content) > 200 {
		runes := []rune(content)
		content = string(runes[:200]) + "..."
	}

	if role != "" {
		return fmt.Sprintf("[%s] %s", role, content)
	}
	return content
}

// ParseSessionSearchResult parses the LLM response into a structured result.
func ParseSessionSearchResult(response string) (SessionSearchResult, error) {
	// Try to find JSON in the response.
	start := strings.Index(response, "{")
	end := strings.LastIndex(response, "}")
	if start == -1 || end == -1 || end <= start {
		return SessionSearchResult{}, fmt.Errorf("no JSON found in response")
	}

	jsonStr := response[start : end+1]

	var result SessionSearchResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return SessionSearchResult{}, fmt.Errorf("parse search result: %w", err)
	}

	return result, nil
}
