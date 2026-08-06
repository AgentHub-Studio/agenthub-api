package agentic_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- GetSkillDescriptionCatalog ---

func TestGetSkillDescriptionCatalog_NotEmpty(t *testing.T) {
	catalog := agentic.GetSkillDescriptionCatalog()
	// 10 generic/utility + 9 platform + 6 diagnostic/optimization/workflow = 25
	assert.GreaterOrEqual(t, len(catalog), 25)
}

func TestGetSkillDescriptionCatalog_AllHaveRequiredFields(t *testing.T) {
	catalog := agentic.GetSkillDescriptionCatalog()
	for _, sd := range catalog {
		assert.NotEmpty(t, sd.Slug, "slug should not be empty")
		assert.NotEmpty(t, sd.Name, "name should not be empty for %s", sd.Slug)
		assert.NotEmpty(t, sd.Description, "description should not be empty for %s", sd.Slug)
		assert.NotEmpty(t, sd.Category, "category should not be empty for %s", sd.Slug)
	}
}

func TestGetSkillDescriptionCatalog_DescriptionsFollowPattern(t *testing.T) {
	catalog := agentic.GetSkillDescriptionCatalog()
	for _, sd := range catalog {
		suggestions := agentic.ValidateDescription(sd.Description)
		assert.Empty(t, suggestions,
			"catalog description for %s should follow the pattern, got suggestions: %v", sd.Slug, suggestions)
	}
}

func TestGetSkillDescriptionCatalog_ReturnsCopy(t *testing.T) {
	c1 := agentic.GetSkillDescriptionCatalog()
	c2 := agentic.GetSkillDescriptionCatalog()
	c1[0].Slug = "mutated"
	assert.NotEqual(t, "mutated", c2[0].Slug, "should return a copy, not a reference")
}

// --- GetSkillDescription ---

func TestGetSkillDescription_KnownSlug(t *testing.T) {
	desc := agentic.GetSkillDescription("document-search")
	assert.Contains(t, desc, "semantic similarity")
	assert.Contains(t, desc, "descriptive sentence")
}

func TestGetSkillDescription_UnknownSlug(t *testing.T) {
	desc := agentic.GetSkillDescription("unknown-tool")
	assert.Empty(t, desc)
}

// --- EnrichDescription ---

func TestEnrichDescription_KnownSlug(t *testing.T) {
	desc := agentic.EnrichDescription("execute-sql", "Runs SQL queries")
	assert.Contains(t, desc, "PostgreSQL datasources")
	assert.NotEqual(t, "Runs SQL queries", desc)
}

func TestEnrichDescription_UnknownSlug_FallsBack(t *testing.T) {
	desc := agentic.EnrichDescription("custom-tool", "My custom tool description")
	assert.Equal(t, "My custom tool description", desc)
}

func TestEnrichDescription_UnknownSlug_EmptyOriginal(t *testing.T) {
	desc := agentic.EnrichDescription("custom-tool", "")
	assert.Empty(t, desc)
}

// --- ValidateDescription ---

func TestValidateDescription_Empty(t *testing.T) {
	suggestions := agentic.ValidateDescription("")
	assert.Len(t, suggestions, 1)
	assert.Contains(t, suggestions[0], "empty")
}

func TestValidateDescription_TooShort(t *testing.T) {
	suggestions := agentic.ValidateDescription("Searches documents.")
	assert.NotEmpty(t, suggestions)
}

func TestValidateDescription_Good(t *testing.T) {
	desc := "Searches documents in the knowledge base. " +
		"Use when the user asks about company docs. " +
		"The 'query' parameter should be descriptive. " +
		"Returns relevant text excerpts."
	suggestions := agentic.ValidateDescription(desc)
	assert.Empty(t, suggestions)
}

func TestValidateDescription_MissingWhen(t *testing.T) {
	desc := "Searches documents in the knowledge base. " +
		"The 'query' parameter should be descriptive. " +
		"Returns relevant text excerpts."
	suggestions := agentic.ValidateDescription(desc)
	hasWhenSuggestion := false
	for _, s := range suggestions {
		if strings.Contains(s, "usage guidance") {
			hasWhenSuggestion = true
		}
	}
	assert.True(t, hasWhenSuggestion)
}

func TestValidateDescription_MissingReturns(t *testing.T) {
	desc := "Searches documents. Use when the user asks about docs. The 'query' parameter is text."
	suggestions := agentic.ValidateDescription(desc)
	hasReturnsSuggestion := false
	for _, s := range suggestions {
		if strings.Contains(s, "output description") {
			hasReturnsSuggestion = true
		}
	}
	assert.True(t, hasReturnsSuggestion)
}

// --- GenerateUpdateSQL ---

func TestGenerateUpdateSQL_NotEmpty(t *testing.T) {
	sql := agentic.GenerateUpdateSQL()
	assert.Contains(t, sql, "UPDATE skill SET description")
	assert.Contains(t, sql, "document-search")
	assert.Contains(t, sql, "execute-sql")
	assert.Contains(t, sql, "http-request")
	// New skills should also be in the generated SQL.
	assert.Contains(t, sql, "debug-agent")
	assert.Contains(t, sql, "optimize-agent")
	assert.Contains(t, sql, "health-check")
}

func TestGenerateUpdateSQL_EscapesSingleQuotes(t *testing.T) {
	sql := agentic.GenerateUpdateSQL()
	// The descriptions contain single-quoted parameter names like 'query'.
	// These should be escaped as '' in SQL.
	assert.Contains(t, sql, "''query''")
}

func TestGenerateUpdateSQL_OnlyUpdatesShortDescriptions(t *testing.T) {
	sql := agentic.GenerateUpdateSQL()
	// Each UPDATE should have a length guard.
	assert.Contains(t, sql, "length(description) <")
}

// --- DescriptionPattern ---

func TestDescriptionPattern_NotEmpty(t *testing.T) {
	assert.NotEmpty(t, agentic.DescriptionPattern)
	assert.Contains(t, agentic.DescriptionPattern, "WHAT")
	assert.Contains(t, agentic.DescriptionPattern, "WHEN")
	assert.Contains(t, agentic.DescriptionPattern, "PARAMETERS")
	assert.Contains(t, agentic.DescriptionPattern, "RETURNS")
}

// --- HTTP Handlers ---

func TestGetSkillDescription_NewDiagnosticSkills(t *testing.T) {
	newSlugs := []string{"debug-agent", "optimize-agent", "onboard-agent", "curate-memory", "health-check", "data-explorer"}
	for _, slug := range newSlugs {
		desc := agentic.GetSkillDescription(slug)
		assert.NotEmpty(t, desc, "description should exist for %s", slug)
		// All new descriptions should follow the pattern.
		suggestions := agentic.ValidateDescription(desc)
		assert.Empty(t, suggestions, "description for %s should follow pattern, got: %v", slug, suggestions)
	}
}

func TestSkillDescHandler_ListDescriptions(t *testing.T) {
	handler := agentic.NewSkillDescHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/skill-descriptions", nil)
	w := httptest.NewRecorder()

	handler.ListDescriptions(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var catalog []agentic.SkillDescription
	require.NoError(t, json.NewDecoder(w.Body).Decode(&catalog))
	assert.GreaterOrEqual(t, len(catalog), 25)
}

func TestSkillDescHandler_GetPattern(t *testing.T) {
	handler := agentic.NewSkillDescHandler()
	req := httptest.NewRequest(http.MethodGet, "/api/skill-descriptions/pattern", nil)
	w := httptest.NewRecorder()

	handler.GetPattern(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.Contains(t, result["pattern"], "WHAT")
}

func TestSkillDescHandler_Validate_Valid(t *testing.T) {
	handler := agentic.NewSkillDescHandler()
	body := `{"description":"Searches documents. Use when user asks. The 'query' parameter is text. Returns results."}`
	req := httptest.NewRequest(http.MethodPost, "/api/skill-descriptions/validate", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.ValidateHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.True(t, result["valid"].(bool))
}

func TestSkillDescHandler_Validate_Invalid(t *testing.T) {
	handler := agentic.NewSkillDescHandler()
	body := `{"description":"Short."}`
	req := httptest.NewRequest(http.MethodPost, "/api/skill-descriptions/validate", strings.NewReader(body))
	w := httptest.NewRecorder()

	handler.ValidateHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var result map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
	assert.False(t, result["valid"].(bool))
	suggestions := result["suggestions"].([]any)
	assert.NotEmpty(t, suggestions)
}

func TestSkillDescHandler_Validate_BadRequest(t *testing.T) {
	handler := agentic.NewSkillDescHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/skill-descriptions/validate", strings.NewReader("not json"))
	w := httptest.NewRecorder()

	handler.ValidateHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSkillDescHandler_ValidateRejectsTrailingJSON(t *testing.T) {
	handler := agentic.NewSkillDescHandler()
	req := httptest.NewRequest(http.MethodPost, "/api/skill-descriptions/validate", strings.NewReader(`{"description":"Searches documents. Use when user asks. The 'query' parameter is text. Returns results."} {"description":"ignored"}`))
	w := httptest.NewRecorder()

	handler.ValidateHandler(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- SkillDescription JSON roundtrip ---

func TestSkillDescription_JSONRoundtrip(t *testing.T) {
	original := agentic.SkillDescription{
		Slug:        "test",
		Name:        "Test",
		Description: "A test skill",
		Category:    "test",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded agentic.SkillDescription
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}
