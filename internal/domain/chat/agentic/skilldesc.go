package agentic

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
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
	{
		Slug:     "troubleshoot",
		Name:     "Troubleshoot",
		Category: "system",
		Description: "Auto-diagnoses errors and failures during tool execution. " +
			"Use when a previous tool call failed and the user needs help understanding what went wrong. " +
			"Analyzes the error, suggests fixes, and optionally retries with corrected parameters. " +
			"Parameters: 'error' (the error message), 'tool_name' (which tool failed), 'original_input' (what was sent). " +
			"Returns diagnosis and suggested next steps.",
	},
	{
		Slug:     "memory-recall",
		Name:     "Memory Recall",
		Category: "system",
		Description: "Searches the agent's long-term memory for relevant facts, preferences, and decisions from past conversations. " +
			"Use when the user references something discussed before, or when context from previous sessions would improve the response. " +
			"Parameters: 'query' (semantic search query), 'limit' (max results). " +
			"Returns matching memories with timestamps and relevance scores.",
	},
	// --- Platform management skills (enriched for LLM tool-calling) ---
	{
		Slug:     "agent-management",
		Name:     "Agent Management",
		Category: "platform",
		Description: "Full CRUD management of agents in the AgentHub platform: create, read, update, delete, publish, archive, clone, and version agents. " +
			"Use when the user asks about agents, wants to create or modify an agent, check versions, manage hooks, or change agent status. " +
			"Key parameters: 'name', 'system_prompt' (agent instructions), 'model_config' (provider/model JSON), optional 'skill_ids' array to bind capabilities. " +
			"Supported operations include listing agents, getting agent details, creating/updating/deleting agents, " +
			"publishing drafts, archiving, cloning, listing versions, and managing agent hooks (create/update/delete). " +
			"ALWAYS confirm destructive operations (delete, archive) with the user before executing. " +
			"Returns agent objects with id, name, status, system prompt, model config, and linked skills.",
	},
	{
		Slug:     "skill-management",
		Name:     "Skill Management",
		Category: "platform",
		Description: "Manages skills (abstract capabilities that group tools): list, get, create, update, delete skills and their tool bindings. " +
			"Use when the user asks about available skills, wants to create or modify a skill, or manage which tools implement a skill. " +
			"Key parameters: 'name', 'slug' (kebab-case identifier), 'description', 'category', 'input_schema' (JSON Schema), optional 'allowed_tools' array. " +
			"When creating or updating a skill, collect all missing fields and the final confirmation in a SINGLE ask_user call whenever possible. " +
			"Do NOT ask for the same confirmation twice or open a second confirmation form after the user already approved the submitted form. " +
			"ALWAYS confirm changes with the user before executing write operations. " +
			"Returns skill objects with id, name, slug, description, category, input schema, and bound tools.",
	},
	{
		Slug:     "tool-management",
		Name:     "Tool Management",
		Category: "platform",
		Description: "Manages tool implementations: list, get, create, update, delete tools (HTTP, SQL, DocumentSearch, Custom). " +
			"Use when the user asks about tool configurations, wants to create or modify a tool endpoint, or check tool details. " +
			"Key parameters: 'name', 'type' (HTTP/SQL/DOCUMENT_SEARCH/CUSTOM), 'config' (type-specific JSON with method, 'url', headers or SQL query), 'labels', 'read_only'. " +
			"ALWAYS confirm changes with the user before executing write operations. " +
			"Returns tool objects with id, name, type, config (method, URL, headers), labels, and read-only flag.",
	},
	{
		Slug:     "knowledge-base-management",
		Name:     "Knowledge Base Management",
		Category: "platform",
		Description: "Manages knowledge bases and their documents: list, get, create, update, delete, sync KBs; upload and manage documents. " +
			"Use when the user asks about knowledge bases, wants to add documents, trigger reindexing, or check KB status. " +
			"Key parameters: 'kb_id' (UUID) to target a specific KB; 'name', 'description' for creation; 'file' for document upload. " +
			"After uploading documents, suggest using sync to trigger reindexing. " +
			"Returns KB objects with id, name, description, document count, and indexing status.",
	},
	{
		Slug:     "mcp-management",
		Name:     "MCP Server Management",
		Category: "platform",
		Description: "Manages MCP (Model Context Protocol) server configurations: list, get, create, update, delete MCP servers. " +
			"Use when the user asks about MCP integrations, wants to add/modify external tool servers, or check MCP status. " +
			"Key parameters: 'name', 'transport_type' (stdio/http), 'http_base_url' for HTTP servers, 'command' and 'args' for stdio servers, 'auto_start', 'enabled'. " +
			"ALWAYS confirm destructive operations (delete) and configuration changes with the user. " +
			"Returns MCP config objects with id, name, transport type, URL, auto-start, and enabled status.",
	},
	{
		Slug:     "chat-management",
		Name:     "Chat Session Management",
		Category: "platform",
		Description: "Manages chat sessions: list sessions, get session details, run conversations, archive sessions, and list messages with pagination. " +
			"Use when the user asks about conversation history, wants to review past messages, or manage active sessions. " +
			"Key parameter: 'session_id' (UUID) to target a specific session; pagination parameters 'page' and 'size' for message lists. " +
			"Returns session objects with id, agent, status, message count, and paginated message lists.",
	},
	{
		Slug:     "datasource-management",
		Name:     "Datasource Management",
		Category: "platform",
		Description: "Manages database datasource connections: list, get, create, update, delete datasources (PostgreSQL, MySQL, SQL Server). " +
			"Use when the user asks about database connections, wants to configure a new datasource, or check connection status. " +
			"Key parameters: 'type' (POSTGRESQL/MYSQL/SQL_SERVER), 'host', 'port', 'database', 'db_user', 'db_password', and optional 'vpn_resource_id'. " +
			"ALWAYS confirm credential changes and destructive operations with the user. " +
			"Returns datasource objects with id, name, type, host, port, database, and VPN resource binding.",
	},
	{
		Slug:     "settings-management",
		Name:     "Settings Management",
		Category: "platform",
		Description: "Manages platform settings: LLM provider configurations, tenant preferences, and system parameters. " +
			"Use when the user asks about LLM configurations, API keys, or system settings. " +
			"ALWAYS confirm changes with the user before modifying settings. " +
			"Returns configuration objects with provider details, model lists, and active status.",
	},
	{
		Slug:     "prompt-template-management",
		Name:     "Prompt Template Management",
		Category: "platform",
		Description: "Manages prompt templates: list, get, create, update, delete reusable system prompt templates. " +
			"Use when the user asks about prompt templates, wants to create or modify templates for agents. " +
			"Key parameters: 'name' (template identifier), 'content' (the system prompt text), 'category', optional 'allowed_tools' array and 'model_override'. " +
			"Templates can include allowed tools restrictions and model overrides. " +
			"Returns template objects with id, name, content, category, allowed tools, and model override.",
	},
	// --- Diagnostic, optimization, and workflow skills ---
	{
		Slug:     "debug-agent",
		Name:     "Debug Agent",
		Category: "diagnostic",
		Description: "Diagnoses agent execution failures by analyzing recent executions, error patterns, and configuration issues. " +
			"Use when an agent is failing, producing incorrect results, or behaving unexpectedly. " +
			"Analyzes execution logs, tool call history, system prompt effectiveness, and skill bindings. " +
			"Parameters: 'agent_id' (target agent UUID), 'issue' (optional description of the problem). " +
			"Returns a structured diagnosis with root cause analysis, affected components, and suggested fixes.",
	},
	{
		Slug:     "optimize-agent",
		Name:     "Optimize Agent",
		Category: "optimization",
		Description: "Reviews an agent's configuration and suggests optimizations for better performance, accuracy, and efficiency. " +
			"Use when the user wants to improve an agent's behavior, reduce token usage, or fix quality issues. " +
			"Analyzes: system prompt quality, skill selection, tool binding efficiency, knowledge base coverage, and model config. " +
			"Parameters: 'agent_id' (target agent UUID), 'focus' (optional: 'prompt', 'tools', 'performance', 'cost'). " +
			"Returns a structured report with specific, actionable recommendations ranked by impact.",
	},
	{
		Slug:     "onboard-agent",
		Name:     "Onboard Agent",
		Category: "wizard",
		Description: "Interactive wizard that guides the user through creating a fully configured agent step by step. " +
			"Use when the user wants to create a new agent from scratch or needs help setting one up properly. " +
			"Steps: (1) Define purpose and identity, (2) Generate system prompt, (3) Select and bind skills, " +
			"(4) Configure knowledge bases, (5) Set model and parameters, (6) Create hooks for automation. " +
			"ALWAYS ask for user confirmation at each step before proceeding. " +
			"Returns a fully configured agent ready for publishing.",
	},
	{
		Slug:     "curate-memory",
		Name:     "Curate Memory",
		Category: "memory",
		Description: "Reviews and curates an agent's long-term memory using the four-type taxonomy: " +
			"user (preferences, expertise), feedback (corrections, confirmed approaches with WHY), " +
			"project (deadlines, decisions — absolute dates), and reference (pointers to external systems). " +
			"Use when the user wants to audit, clean up, or understand what the agent has learned about them. " +
			"Steps: (1) List memories by type, (2) Identify duplicates/conflicts/outdated entries per type, " +
			"(3) Present a structured report grouped by type, (4) Apply requested operations. " +
			"Parameters: 'agent_id' (target agent UUID), 'action' (optional: 'audit', 'deduplicate', 'clean', 'list'). " +
			"Returns a typed memory audit report with per-type counts and recommended operations.",
	},
	{
		Slug:     "health-check",
		Name:     "Health Check",
		Category: "diagnostic",
		Description: "Performs a comprehensive health check of the AgentHub platform: agents, knowledge bases, MCP servers, and recent executions. " +
			"Use when the user asks about platform status, wants to check if everything is working, or suspects issues. " +
			"Checks: (1) Agent status distribution, (2) KB indexing status, (3) MCP server connectivity, " +
			"(4) Recent execution success/failure rates, (5) Pending document processing. " +
			"Parameters: 'scope' (optional: 'agents', 'kbs', 'mcp', 'executions', or 'all'). " +
			"Returns a structured health report with status indicators and any detected issues.",
	},
	{
		Slug:     "data-explorer",
		Name:     "Data Explorer",
		Category: "analysis",
		Description: "Combines SQL queries and document search to answer complex data questions that span structured and unstructured sources. " +
			"Use when the user asks analytical questions that may require both database queries and document lookups. " +
			"Automatically determines which data sources to query based on the question. " +
			"Parameters: 'question' (the analytical question), 'datasource_id' (optional: specific datasource). " +
			"Returns a synthesized analysis combining results from all relevant sources with citations.",
	},
	// --- New skills from migration 000025 ---
	{
		Slug:     "review-agent",
		Name:     "Review Agent",
		Category: "diagnostic",
		Description: "Performs a deep quality review of an agent: analyzes recent execution success/failure rates, " +
			"evaluates system prompt clarity and completeness, verifies skill and KB bindings, " +
			"and produces a structured improvement report with concrete action items. " +
			"Use when the user asks to audit, evaluate, or improve an existing agent. " +
			"Key parameters: 'agent_id' (UUID of the agent), optional 'focus' (prompt|skills|executions|all). " +
			"Runs in fork mode to keep diagnostic data out of the main conversation. " +
			"Returns a structured report with findings and prioritized recommendations.",
	},
	{
		Slug:     "batch-execute",
		Name:     "Batch Execute",
		Category: "orchestration",
		Description: "Orchestrates a large, parallelizable task by researching scope, decomposing into independent work units, " +
			"spawning parallel sub-agents (one per unit), tracking progress, and synthesizing results. " +
			"Use when a task can be split into 5–30 independent parallel workstreams that do not depend on each other. " +
			"Key parameter: 'instruction' (the overall task to parallelize); optional 'max_agents' (default 10). " +
			"Returns a progress table with status per unit and a final synthesis.",
	},
	{
		Slug:     "review-memory",
		Name:     "Review Memory",
		Category: "memory",
		Description: "Reviews an agent's memory landscape: lists entries by type, identifies duplicates and outdated entries, " +
			"checks for conflicts between memory types, and proposes promotions to agent instructions. " +
			"Use when an agent is behaving inconsistently across sessions or memories seem stale or contradictory. " +
			"Key parameters: 'agent_id' (UUID), optional 'action' (audit|deduplicate|clean|promote). " +
			"Presents proposed changes for confirmation — does NOT modify memories without explicit approval. " +
			"Returns a structured report grouped by action type.",
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
	if err := httputil.DecodeSingleJSON(r.Body, &req); err != nil {
		// Bug 282: http.Error usa text/plain mesmo com body JSON
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request body"})
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
