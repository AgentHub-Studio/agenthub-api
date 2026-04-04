package agentic_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- Constants ---

func TestSessionSearch_Constants(t *testing.T) {
	assert.Equal(t, 100, agentic.MaxMessagesToScan)
	assert.Equal(t, 100, agentic.MaxSessionsToSearch)
	assert.Equal(t, 2000, agentic.MaxExcerptChars)
}

// --- BuildSessionSearchPrompt ---

func TestBuildSessionSearchPrompt(t *testing.T) {
	prompt := agentic.BuildSessionSearchPrompt()
	assert.Contains(t, prompt, "session search engine")
	assert.Contains(t, prompt, "Exact tag match")
	assert.Contains(t, prompt, "Title match")
	assert.Contains(t, prompt, "Semantic match")
	assert.Contains(t, prompt, "indices")
}

// --- BuildSessionSearchUserPrompt ---

func TestBuildSessionSearchUserPrompt_Basic(t *testing.T) {
	sessions := []agentic.SessionSearchEntry{
		{
			Index:        0,
			SessionID:    "sess-1",
			Title:        "Auth refactor",
			Tags:         []string{"auth", "refactor"},
			MessageCount: 25,
			CreatedAt:    time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC),
			Excerpt:      "[user] Help me refactor auth\n[assistant] Sure!",
		},
	}

	prompt := agentic.BuildSessionSearchUserPrompt("auth changes", sessions)
	assert.Contains(t, prompt, "Query: auth changes")
	assert.Contains(t, prompt, "Session 0")
	assert.Contains(t, prompt, "Auth refactor")
	assert.Contains(t, prompt, "auth, refactor")
	assert.Contains(t, prompt, "2026-03-15")
}

func TestBuildSessionSearchUserPrompt_Empty(t *testing.T) {
	prompt := agentic.BuildSessionSearchUserPrompt("test", nil)
	assert.Contains(t, prompt, "Query: test")
}

func TestBuildSessionSearchUserPrompt_MultipleSessions(t *testing.T) {
	sessions := []agentic.SessionSearchEntry{
		{Index: 0, Title: "First"},
		{Index: 1, Title: "Second"},
		{Index: 2, Title: "Third"},
	}

	prompt := agentic.BuildSessionSearchUserPrompt("query", sessions)
	assert.Contains(t, prompt, "Session 0")
	assert.Contains(t, prompt, "Session 1")
	assert.Contains(t, prompt, "Session 2")
}

// --- ExtractSessionExcerpt ---

func TestExtractSessionExcerpt_Empty(t *testing.T) {
	assert.Empty(t, agentic.ExtractSessionExcerpt(nil, 5, 5))
}

func TestExtractSessionExcerpt_InvalidJSON(t *testing.T) {
	assert.Empty(t, agentic.ExtractSessionExcerpt(json.RawMessage(`not json`), 5, 5))
}

func TestExtractSessionExcerpt_FewMessages(t *testing.T) {
	msgs := []map[string]interface{}{
		{"role": "user", "content": "hello"},
		{"role": "assistant", "content": "hi there"},
	}
	raw, _ := json.Marshal(msgs)

	excerpt := agentic.ExtractSessionExcerpt(raw, 5, 5)
	assert.Contains(t, excerpt, "[user] hello")
	assert.Contains(t, excerpt, "[assistant] hi there")
	assert.NotContains(t, excerpt, "omitted")
}

func TestExtractSessionExcerpt_ManyMessages(t *testing.T) {
	msgs := make([]map[string]interface{}, 20)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = map[string]interface{}{"role": "user", "content": "msg"}
		} else {
			msgs[i] = map[string]interface{}{"role": "assistant", "content": "reply"}
		}
	}
	raw, _ := json.Marshal(msgs)

	excerpt := agentic.ExtractSessionExcerpt(raw, 3, 3)
	assert.Contains(t, excerpt, "omitted")
}

func TestExtractSessionExcerpt_Truncation(t *testing.T) {
	// Create messages with very long content.
	msgs := make([]map[string]interface{}, 50)
	longContent := strings.Repeat("a", 500)
	for i := range msgs {
		msgs[i] = map[string]interface{}{"role": "user", "content": longContent}
	}
	raw, _ := json.Marshal(msgs)

	excerpt := agentic.ExtractSessionExcerpt(raw, 10, 10)
	// Message content should be truncated to 200 chars, and total to MaxExcerptChars.
	assert.LessOrEqual(t, len([]rune(excerpt)), agentic.MaxExcerptChars+10) // +margin for "..."
}

// --- ParseSessionSearchResult ---

func TestParseSessionSearchResult_Valid(t *testing.T) {
	response := `{"indices": [3, 1, 7], "reasoning": "Session 3 matches best"}`
	result, err := agentic.ParseSessionSearchResult(response)
	require.NoError(t, err)
	assert.Equal(t, []int{3, 1, 7}, result.Indices)
	assert.Contains(t, result.Reasoning, "Session 3")
}

func TestParseSessionSearchResult_WithSurroundingText(t *testing.T) {
	response := `Here are the results: {"indices": [0, 2], "reasoning": "matched"} that's my answer`
	result, err := agentic.ParseSessionSearchResult(response)
	require.NoError(t, err)
	assert.Equal(t, []int{0, 2}, result.Indices)
}

func TestParseSessionSearchResult_EmptyIndices(t *testing.T) {
	response := `{"indices": [], "reasoning": "no matches"}`
	result, err := agentic.ParseSessionSearchResult(response)
	require.NoError(t, err)
	assert.Empty(t, result.Indices)
}

func TestParseSessionSearchResult_NoJSON(t *testing.T) {
	_, err := agentic.ParseSessionSearchResult("no json here")
	assert.Error(t, err)
}

func TestParseSessionSearchResult_InvalidJSON(t *testing.T) {
	_, err := agentic.ParseSessionSearchResult("{invalid json}")
	assert.Error(t, err)
}

// --- SessionSearchEntry fields ---

func TestSessionSearchEntry_Fields(t *testing.T) {
	entry := agentic.SessionSearchEntry{
		Index:        5,
		SessionID:    "sess-abc",
		Title:        "Test Session",
		Tags:         []string{"test", "debug"},
		Summary:      "Debugging auth issues",
		MessageCount: 42,
		CreatedAt:    time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	}
	assert.Equal(t, 5, entry.Index)
	assert.Equal(t, "sess-abc", entry.SessionID)
	assert.Len(t, entry.Tags, 2)
}
