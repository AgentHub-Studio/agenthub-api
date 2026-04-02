package agentic_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- TemplateStore ---

func TestNewTemplateStore_LoadsAllTemplates(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	templates := store.List()
	assert.GreaterOrEqual(t, len(templates), 6) // 4 system + compact + memory_eval
}

func TestTemplateStore_List_Sorted(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	templates := store.List()
	for i := 1; i < len(templates); i++ {
		assert.Less(t, templates[i-1].ID, templates[i].ID,
			"templates should be sorted by ID")
	}
}

func TestTemplateStore_GetByID_Found(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	tmpl, ok := store.GetByID("general_assistant")
	assert.True(t, ok)
	assert.Equal(t, "general_assistant", tmpl.ID)
	assert.Equal(t, "General Assistant", tmpl.Name)
	assert.NotEmpty(t, tmpl.Description)
	assert.Contains(t, tmpl.Content, "# Identity")
}

func TestTemplateStore_GetByID_NotFound(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	_, ok := store.GetByID("nonexistent")
	assert.False(t, ok)
}

func TestTemplateStore_GetContent(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	content := store.GetContent("rag_assistant")
	assert.Contains(t, content, "document_search")
}

func TestTemplateStore_GetContent_NotFound(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	content := store.GetContent("nonexistent")
	assert.Empty(t, content)
}

func TestTemplateStore_AllKnownTemplatesHaveMetadata(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	expected := []string{
		"api_integration",
		"compact_prompt",
		"data_analyst",
		"general_assistant",
		"memory_eval_prompt",
		"rag_assistant",
	}

	for _, id := range expected {
		tmpl, ok := store.GetByID(id)
		require.True(t, ok, "template %s should exist", id)
		assert.NotEmpty(t, tmpl.Name, "template %s should have a name", id)
		assert.NotEmpty(t, tmpl.Description, "template %s should have a description", id)
		assert.NotEmpty(t, tmpl.Content, "template %s should have content", id)
	}
}

// --- Template content validation ---

func TestTemplate_GeneralAssistant_Content(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	tmpl, _ := store.GetByID("general_assistant")
	assert.Contains(t, tmpl.Content, "# Identity")
	assert.Contains(t, tmpl.Content, "# Instructions")
	assert.Contains(t, tmpl.Content, "# Tool Usage")
	assert.Contains(t, tmpl.Content, "# Constraints")
}

func TestTemplate_RagAssistant_Content(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	tmpl, _ := store.GetByID("rag_assistant")
	assert.Contains(t, tmpl.Content, "document_search")
	assert.Contains(t, tmpl.Content, "cite")
}

func TestTemplate_DataAnalyst_Content(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	tmpl, _ := store.GetByID("data_analyst")
	assert.Contains(t, tmpl.Content, "SQL")
	assert.Contains(t, tmpl.Content, "execute_sql")
	assert.Contains(t, tmpl.Content, "confirm")
}

func TestTemplate_ApiIntegration_Content(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	tmpl, _ := store.GetByID("api_integration")
	assert.Contains(t, tmpl.Content, "HTTP")
	assert.Contains(t, tmpl.Content, "rate")
}

func TestTemplate_CompactPrompt_Content(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	tmpl, _ := store.GetByID("compact_prompt")
	assert.Contains(t, tmpl.Content, "Summarize")
	assert.Contains(t, tmpl.Content, "{{conversation}}")
}

func TestTemplate_MemoryEvalPrompt_Content(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	tmpl, _ := store.GetByID("memory_eval_prompt")
	assert.Contains(t, tmpl.Content, "JSON array")
	assert.Contains(t, tmpl.Content, "snake_case")
	assert.Contains(t, tmpl.Content, "{{conversation}}")
}

// --- HTTP Handler ---

func TestTemplateHandler_ListTemplates(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	handler := agentic.NewTemplateHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/prompt-templates", nil)
	w := httptest.NewRecorder()

	handler.ListTemplates(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var templates []agentic.PromptTemplate
	require.NoError(t, json.NewDecoder(w.Body).Decode(&templates))
	assert.GreaterOrEqual(t, len(templates), 6)

	// Verify all have required fields.
	for _, tmpl := range templates {
		assert.NotEmpty(t, tmpl.ID)
		assert.NotEmpty(t, tmpl.Content)
	}
}

func TestTemplateHandler_GetTemplate_Found(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	handler := agentic.NewTemplateHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/prompt-templates/general_assistant", nil)
	// Simulate path value (Go 1.22+ ServeMux style).
	req.SetPathValue("id", "general_assistant")
	w := httptest.NewRecorder()

	handler.GetTemplate(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var tmpl agentic.PromptTemplate
	require.NoError(t, json.NewDecoder(w.Body).Decode(&tmpl))
	assert.Equal(t, "general_assistant", tmpl.ID)
	assert.Equal(t, "General Assistant", tmpl.Name)
}

func TestTemplateHandler_GetTemplate_NotFound(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	handler := agentic.NewTemplateHandler(store)
	req := httptest.NewRequest(http.MethodGet, "/api/prompt-templates/nonexistent", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	handler.GetTemplate(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "template not found")
}

func TestTemplateHandler_GetTemplate_FallbackPathParsing(t *testing.T) {
	store, err := agentic.NewTemplateStore()
	require.NoError(t, err)

	handler := agentic.NewTemplateHandler(store)
	// Don't set PathValue — test fallback URL path parsing.
	req := httptest.NewRequest(http.MethodGet, "/api/prompt-templates/data_analyst", nil)
	w := httptest.NewRecorder()

	handler.GetTemplate(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var tmpl agentic.PromptTemplate
	require.NoError(t, json.NewDecoder(w.Body).Decode(&tmpl))
	assert.Equal(t, "data_analyst", tmpl.ID)
}

// --- JSON roundtrip ---

func TestPromptTemplate_JSONRoundtrip(t *testing.T) {
	original := agentic.PromptTemplate{
		ID:          "test",
		Name:        "Test Template",
		Description: "A test template",
		Content:     "Hello {{name}}",
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded agentic.PromptTemplate
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, original, decoded)
}
