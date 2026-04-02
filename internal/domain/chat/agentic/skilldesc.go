package agentic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// SkillDescription holds a rich, LLM-oriented description for a known skill.
// Following the pattern: what it does, when to use, parameter guidance, output format.
type SkillDescription struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// DescriptionPattern documents the recommended structure for skill descriptions.
const DescriptionPattern = `Every skill description should follow this four-part structure:

1. WHAT: One sentence explaining what the tool does.
2. WHEN: Specific scenarios when the LLM should use this tool.
3. PARAMETERS: Guidance on how to fill input parameters.
4. RETURNS: What the output looks like.

Example:
"Searches documents in the knowledge base by semantic similarity.
Use when the user asks about information that may exist in company documents.
The 'query' parameter should be a descriptive sentence, not isolated keywords.
Returns relevant text excerpts with similarity scores."
`

// knownSkillDescriptions is the canonical catalog of rich descriptions
// for well-known skills. These are used to:
// - Enrich skill descriptions via migration
// - Serve as reference in the GET /api/skill-descriptions endpoint
// - Validate new skill descriptions against the pattern
var knownSkillDescriptions = []SkillDescription{
	{
		Slug:     "document-search",
		Name:     "Document Search",
		Category: "rag",
		Description: "Searches documents in the knowledge base by semantic similarity. " +
			"Use when the user asks about information that may exist in uploaded documents or company knowledge bases. " +
			"The 'query' parameter should be a descriptive sentence or question, not isolated keywords — " +
			"for example, use 'how to reset a user password' instead of 'password reset'. " +
			"Returns relevant text excerpts with similarity scores, sorted by relevance.",
	},
	{
		Slug:     "execute-sql",
		Name:     "Execute SQL",
		Category: "data",
		Description: "Executes SQL queries against configured PostgreSQL datasources. " +
			"Use when the user needs to query, analyze, or explore structured data. " +
			"The 'query' parameter must be valid SQL. Always use SELECT for reads; " +
			"NEVER execute UPDATE, DELETE, or DROP without explicit user confirmation. " +
			"Use LIMIT to avoid returning excessive rows. " +
			"Returns rows as an array of objects with column names as keys.",
	},
	{
		Slug:     "http-request",
		Name:     "HTTP Request",
		Category: "integration",
		Description: "Makes HTTP requests to external APIs and services. " +
			"Use when the user needs to interact with a REST API, fetch data from a URL, or trigger a webhook. " +
			"Parameters include 'method' (GET/POST/PUT/DELETE), 'url', 'headers' (object), and 'body' (JSON). " +
			"Always include required headers like Content-Type and Authorization. " +
			"Returns the HTTP status code, response headers, and response body.",
	},
	{
		Slug:     "send-email",
		Name:     "Send Email",
		Category: "communication",
		Description: "Sends an email via the configured email service. " +
			"Use when the user explicitly asks to send, forward, or reply to an email. " +
			"ALWAYS confirm recipient, subject, and content with the user before sending. " +
			"Parameters: 'to' (email address), 'subject', 'body' (plain text or HTML), 'cc' (optional). " +
			"Returns a confirmation with the message ID.",
	},
	{
		Slug:     "web-scraper",
		Name:     "Web Scraper",
		Category: "data",
		Description: "Extracts content from a web page given its URL. " +
			"Use when the user provides a URL and wants to read, summarize, or analyze its content. " +
			"The 'url' parameter should be a complete URL including the protocol (https://). " +
			"Returns the page title, main text content, and metadata. " +
			"Note: some pages may block automated access.",
	},
	{
		Slug:     "code-interpreter",
		Name:     "Code Interpreter",
		Category: "compute",
		Description: "Executes code snippets in a sandboxed environment. " +
			"Use when the user needs calculations, data transformations, or script execution. " +
			"The 'code' parameter should be the complete code to run. " +
			"The 'language' parameter specifies the runtime (python, javascript). " +
			"Returns stdout, stderr, and any generated files or visualizations.",
	},
	{
		Slug:     "file-upload",
		Name:     "File Upload",
		Category: "storage",
		Description: "Uploads a file to the agent's storage (MinIO). " +
			"Use when the user wants to save, export, or share a generated file. " +
			"Parameters: 'filename', 'content' (base64-encoded), 'content_type' (MIME type). " +
			"Returns the file URL and metadata (size, type, upload timestamp).",
	},
	{
		Slug:     "calendar-event",
		Name:     "Calendar Event",
		Category: "productivity",
		Description: "Creates, reads, or updates calendar events. " +
			"Use when the user asks about scheduling, meetings, or availability. " +
			"Parameters: 'action' (create/list/update), 'title', 'start_time', 'end_time' (ISO 8601), 'attendees'. " +
			"ALWAYS confirm event details with the user before creating or modifying. " +
			"Returns event details including any conflicts detected.",
	},
}

// GetSkillDescriptionCatalog returns the full catalog of known skill descriptions.
func GetSkillDescriptionCatalog() []SkillDescription {
	result := make([]SkillDescription, len(knownSkillDescriptions))
	copy(result, knownSkillDescriptions)
	return result
}

// GetSkillDescription returns the rich description for a known skill slug,
// or empty string if not in the catalog.
func GetSkillDescription(slug string) string {
	for _, sd := range knownSkillDescriptions {
		if sd.Slug == slug {
			return sd.Description
		}
	}
	return ""
}

// EnrichDescription returns the catalog description for a known skill slug,
// or falls back to the provided original description.
func EnrichDescription(slug, original string) string {
	if rich := GetSkillDescription(slug); rich != "" {
		return rich
	}
	return original
}

// ValidateDescription checks if a skill description follows the recommended pattern.
// Returns a list of suggestions for improvement.
func ValidateDescription(description string) []string {
	var suggestions []string

	if len(description) == 0 {
		return []string{"description is empty — provide at least a one-sentence summary"}
	}

	if len(description) < 50 {
		suggestions = append(suggestions, "description is very short — consider adding when/how/returns guidance")
	}

	lower := strings.ToLower(description)

	// Check for "when to use" guidance.
	hasWhen := strings.Contains(lower, "use when") ||
		strings.Contains(lower, "use this when") ||
		strings.Contains(lower, "use for")
	if !hasWhen {
		suggestions = append(suggestions, "missing usage guidance — add 'Use when...' to help the LLM decide when to invoke this tool")
	}

	// Check for parameter guidance.
	hasParams := strings.Contains(lower, "parameter") ||
		strings.Contains(lower, "'query'") ||
		strings.Contains(lower, "'url'") ||
		strings.Contains(lower, "'code'")
	if !hasParams {
		suggestions = append(suggestions, "missing parameter guidance — describe key parameters and their expected format")
	}

	// Check for output description.
	hasReturns := strings.Contains(lower, "returns") ||
		strings.Contains(lower, "output")
	if !hasReturns {
		suggestions = append(suggestions, "missing output description — add 'Returns...' to describe what the tool produces")
	}

	return suggestions
}

// --- HTTP Handler ---

// SkillDescHandler serves the skill description catalog.
type SkillDescHandler struct{}

// NewSkillDescHandler creates a handler.
func NewSkillDescHandler() *SkillDescHandler {
	return &SkillDescHandler{}
}

// ListDescriptions handles GET /api/skill-descriptions.
func (h *SkillDescHandler) ListDescriptions(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(GetSkillDescriptionCatalog())
}

// GetPattern handles GET /api/skill-descriptions/pattern.
func (h *SkillDescHandler) GetPattern(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"pattern": DescriptionPattern,
	})
}

// ValidateHandler handles POST /api/skill-descriptions/validate.
func (h *SkillDescHandler) ValidateHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	suggestions := ValidateDescription(req.Description)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"valid":       len(suggestions) == 0,
		"suggestions": suggestions,
	})
}

// GenerateUpdateSQL generates SQL UPDATE statements to enrich skill descriptions
// in the database. This is meant for generating migration content.
func GenerateUpdateSQL() string {
	var sb strings.Builder
	sb.WriteString("-- Auto-generated: enrich skill descriptions for LLM tool-calling.\n")
	sb.WriteString("-- Only updates skills whose current description is shorter than the enriched version.\n\n")

	for _, sd := range knownSkillDescriptions {
		escaped := strings.ReplaceAll(sd.Description, "'", "''")
		fmt.Fprintf(&sb, "UPDATE skill SET description = '%s', updated_at = NOW()\n", escaped)
		fmt.Fprintf(&sb, "  WHERE slug = '%s' AND length(description) < %d;\n\n", sd.Slug, len(sd.Description))
	}

	return sb.String()
}
