package agentic

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed templates/*.txt
var templateFS embed.FS

// PromptTemplate represents a reusable system prompt template.
type PromptTemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// templateMeta holds static metadata for each known template file.
var templateMeta = map[string]struct {
	Name        string
	Description string
}{
	"general_assistant": {
		Name:        "General Assistant",
		Description: "Multi-purpose assistant with tool usage, clear formatting, and safety guidelines.",
	},
	"rag_assistant": {
		Name:        "RAG Assistant",
		Description: "Document search and analysis specialist. Always searches before answering.",
	},
	"data_analyst": {
		Name:        "Data Analyst",
		Description: "SQL-powered data exploration and analysis with safety confirmations for writes.",
	},
	"api_integration": {
		Name:        "API Integration",
		Description: "HTTP API interaction assistant with error handling and rate-limit awareness.",
	},
	"compact_prompt": {
		Name:        "Context Compaction",
		Description: "Prompt used by the ContextManager to summarize conversation history.",
	},
	"memory_eval_prompt": {
		Name:        "Memory Evaluation",
		Description: "Prompt used by the MemoryBridge to extract memorable information from turns.",
	},
}

// TemplateStore provides access to embedded prompt templates.
type TemplateStore struct {
	templates []PromptTemplate
	byID      map[string]PromptTemplate
}

// NewTemplateStore loads all templates from the embedded filesystem.
func NewTemplateStore() (*TemplateStore, error) {
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return nil, fmt.Errorf("templatestore: read dir: %w", err)
	}

	store := &TemplateStore{
		byID: make(map[string]PromptTemplate),
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}

		content, err := templateFS.ReadFile(filepath.Join("templates", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("templatestore: read %s: %w", entry.Name(), err)
		}

		id := strings.TrimSuffix(entry.Name(), ".txt")
		meta, ok := templateMeta[id]
		if !ok {
			meta.Name = id
			meta.Description = ""
		}

		tmpl := PromptTemplate{
			ID:          id,
			Name:        meta.Name,
			Description: meta.Description,
			Content:     string(content),
		}

		store.templates = append(store.templates, tmpl)
		store.byID[id] = tmpl
	}

	sort.Slice(store.templates, func(i, j int) bool {
		return store.templates[i].ID < store.templates[j].ID
	})

	return store, nil
}

// List returns all available templates.
func (s *TemplateStore) List() []PromptTemplate {
	return s.templates
}

// GetByID returns a template by its ID, or false if not found.
func (s *TemplateStore) GetByID(id string) (PromptTemplate, bool) {
	t, ok := s.byID[id]
	return t, ok
}

// GetContent returns the raw content of a template, or empty string if not found.
func (s *TemplateStore) GetContent(id string) string {
	if t, ok := s.byID[id]; ok {
		return t.Content
	}
	return ""
}

// --- HTTP Handler ---

// TemplateHandler serves prompt template endpoints.
type TemplateHandler struct {
	store *TemplateStore
}

// NewTemplateHandler creates a handler backed by the given store.
func NewTemplateHandler(store *TemplateStore) *TemplateHandler {
	return &TemplateHandler{store: store}
}

// ListTemplates handles GET /api/prompt-templates.
func (h *TemplateHandler) ListTemplates(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.store.List())
}

// GetTemplate handles GET /api/prompt-templates/{id}.
func (h *TemplateHandler) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		// Fallback for chi router: try URL path segment.
		parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
		if len(parts) > 0 {
			id = parts[len(parts)-1]
		}
	}

	tmpl, ok := h.store.GetByID(id)
	if !ok {
		http.Error(w, `{"error":"template not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tmpl)
}
