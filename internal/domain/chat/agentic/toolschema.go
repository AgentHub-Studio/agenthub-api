package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// ToolsBySkillLister returns the tools bound to a skill.
type ToolsBySkillLister interface {
	ListBySkill(ctx context.Context, skillID uuid.UUID) ([]tool.SkillTool, []tool.Tool, error)
}

// LLMTool represents a tool definition sent to the LLM in the tools[] array.
// The structure matches the Anthropic / OpenAI function-calling schema.
type LLMTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
	// ReadOnly indicates whether this tool only reads data and has no side effects.
	// Read-only tools can be executed in parallel; write tools run serially.
	// Inspired by Claude Code's isConcurrencySafe / isReadOnly per-tool flags.
	ReadOnly bool `json:"readOnly,omitempty"`
	// MaxResultChars overrides the global MaxToolResultChars for this tool.
	// Zero means use the global default from RunConfig.
	// Inspired by Claude Code's per-tool maxResultSizeChars.
	MaxResultChars int `json:"-"` // not sent to LLM
	// Builtin marks tools that are built into the platform (document_search,
	// memory_store, agent) as opposed to user-configured skills/MCP tools.
	// Used for stable sort ordering: builtins form a contiguous prefix,
	// skill/MCP tools form a suffix — both sorted alphabetically within
	// their partition. This prevents MCP tools from interleaving with builtins
	// and invalidating the LLM's prompt cache.
	// Inspired by Claude Code's assembleToolPool in tools.ts.
	Builtin bool `json:"-"` // not sent to LLM
	// ShouldDefer indicates this tool's full schema should not be included in
	// the initial prompt. The LLM loads it on demand via tool_search builtin.
	// Reduces prompt size for agents with many tools.
	// Inspired by Claude Code's shouldDefer flag (Tool.ts).
	ShouldDefer bool `json:"-"` // not sent to LLM
	// IsDestructive flags tools that perform irreversible operations (delete,
	// drop, overwrite). Used to auto-require confirmation even in permissive modes.
	// Inspired by Claude Code's isDestructive per-tool flag (Tool.ts).
	IsDestructive bool `json:"-"` // not sent to LLM
	// SearchHint is a short keyword phrase for tool_search matching.
	// Inspired by Claude Code's searchHint per-tool string (Tool.ts).
	SearchHint string `json:"-"` // not sent to LLM
	// AllowedTools restricts which tools this skill can use when invoked
	// in coordinator/worker mode. Empty means all tools are allowed.
	// Propagated from skill.AllowedTools.
	// Inspired by Claude Code's BundledSkillDefinition.allowedTools.
	AllowedTools []string `json:"-"` // not sent to LLM
	// AlwaysLoad prevents this tool from being deferred even when ToolSearch is active.
	// Use for tools the LLM must see on turn 1 without a tool_search round-trip.
	// Builtins are implicitly always-load; this flag is for MCP/skill tools that
	// need the same treatment.
	// Inspired by Claude Code's alwaysLoad flag (Tool.ts, MCP _meta['anthropic/alwaysLoad']).
	AlwaysLoad bool `json:"-"` // not sent to LLM
	// DisableModelInvocation marks skills that should only be user-invocable
	// (via slash commands), not LLM-invocable. Their descriptions are excluded
	// from the system prompt to save context tokens.
	// Inspired by Claude Code's BundledSkillDefinition.disableModelInvocation.
	DisableModelInvocation bool `json:"-"` // not sent to LLM
	// ConcurrencySafe indicates this tool can run in parallel even if not read-only.
	// When false, the tool is only parallelized if ReadOnly is true.
	// Inspired by Claude Code's isConcurrencySafe independent of isReadOnly.
	ConcurrencySafe bool `json:"-"` // not sent to LLM
	// ContextMode determines how this skill executes: "inline" (default) runs in
	// the current conversation context; "fork" runs in a sub-agent with its own context.
	// Fork mode prevents diagnostic skills from polluting the main conversation.
	// Inspired by Claude Code's BundledSkillDefinition.context ('inline' | 'fork').
	ContextMode string `json:"-"` // not sent to LLM
	// WhenToUse provides WHEN guidance separate from description (WHAT).
	// Injected into the system prompt as a sub-bullet under the tool listing.
	// Inspired by Claude Code's BundledSkillDefinition.whenToUse.
	WhenToUse string `json:"-"` // not sent to LLM; injected into system prompt
	// InterruptBehavior is "block" or "" (meaning cancel).
	// "block" tools complete before the run stops; "" tools are aborted immediately.
	// Inspired by Claude Code's Tool.ts interruptBehavior(): 'cancel' | 'block'.
	InterruptBehavior string `json:"-"` // not sent to LLM
	// IsSearchOrRead marks tools whose results auto-collapse in the UI.
	// Inspired by Claude Code's Tool.ts isSearchOrReadCommand().
	IsSearchOrRead bool `json:"-"` // not sent to LLM
}

// ToolSchemaBuilder converts the skills/tools linked to an agent into LLMTool
// definitions compatible with the LLM function-calling API.
type ToolSchemaBuilder struct {
	skills             SkillLister
	tools              ToolsBySkillLister
	kbs                KBLister
	mcpBridge          *MCPToolBridge
	currentDepth       int
	maxDepth           int
	lastUserOnlySkills []skill.Skill
}

// NewToolSchemaBuilder creates a ToolSchemaBuilder.
func NewToolSchemaBuilder(skills SkillLister, tools ToolsBySkillLister, kbs KBLister) *ToolSchemaBuilder {
	return &ToolSchemaBuilder{skills: skills, tools: tools, kbs: kbs, maxDepth: 3}
}

// WithMCPBridge attaches an MCP tool bridge so that MCP tools are included
// alongside internal skills in the tool definitions sent to the LLM.
func (b *ToolSchemaBuilder) WithMCPBridge(bridge *MCPToolBridge) *ToolSchemaBuilder {
	b.mcpBridge = bridge
	return b
}

// WithDepthLimits sets the current and max depth for sub-agent tool availability.
func (b *ToolSchemaBuilder) WithDepthLimits(currentDepth, maxDepth int) *ToolSchemaBuilder {
	b.currentDepth = currentDepth
	b.maxDepth = maxDepth
	return b
}

// ToolBuildResult contains both the loaded and deferred tools from a Build call.
type ToolBuildResult struct {
	// Loaded contains tools whose full schema is sent to the LLM.
	Loaded []LLMTool
	// Deferred contains tools that require tool_search to load.
	// Only their names are announced to the LLM in a system message.
	Deferred []LLMTool
	// All contains every tool (loaded + deferred) for execution lookups.
	All []LLMTool
	// UserOnlySkills contains skills with DisableModelInvocation=true.
	// These are announced in the system prompt so the LLM can suggest them
	// to the user, but they are not included in the tools[] array.
	UserOnlySkills []skill.Skill
}

// DeferredToolThreshold is the minimum number of total tools before deferred
// loading is activated. Below this count, all tools are loaded immediately.
// Inspired by Claude Code's isToolSearchEnabled threshold.
const DeferredToolThreshold = 15

// Build returns the tool definitions for the given agent.
func (b *ToolSchemaBuilder) Build(ctx context.Context, agentID uuid.UUID) ([]LLMTool, error) {
	// 1. Load explicitly linked skills (legacy)
	skills, err := b.skills.ListByAgentID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("toolschema: list skills: %w", err)
	}

	// 2. Load integration-based skills automatically (simplification)
	// We include all skills from 'INTEGRATION_HTTP' and 'INTEGRATION_DATABASE'
	// as they are managed via the Integrations tab and should be globally available.
	integrationCategories := []string{"INTEGRATION_HTTP", "INTEGRATION_DATABASE"}
	for _, cat := range integrationCategories {
		content, _, err := b.skills.List(ctx, &cat, pagination.PageRequest{Page: 0, Size: 100})
		if err == nil {
			for _, skItem := range content {
				// Avoid duplicates if already explicitly linked
				exists := false
				for _, sk := range skills {
					if sk.ID == skItem.ID {
						exists = true
						break
					}
				}
				if !exists {
					skills = append(skills, skItem)
				}
			}
		}
	}

	var tools []LLMTool
	var userOnlySkills []skill.Skill
	for _, sk := range skills {
		// Skills marked DisableModelInvocation are user-only (slash commands).
		// Exclude from LLM tool list to save context tokens.
		// Inspired by Claude Code's BundledSkillDefinition.disableModelInvocation.
		if sk.DisableModelInvocation {
			userOnlySkills = append(userOnlySkills, sk)
			continue
		}
		t, err := b.skillToLLMTool(ctx, sk)
		if err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}
	// Store user-only skills for prompt announcement (accessible via lastUserOnlySkills).
	b.lastUserOnlySkills = userOnlySkills

		// Builtin: document_search — added when the agent has linked knowledge bases
	// or if we decide to include all tenant KBs automatically.
	if b.kbs != nil {
		kbsResp, err := b.kbs.ListByAgentID(ctx, agentID)
		if err != nil {
			return nil, fmt.Errorf("toolschema: list knowledge bases: %w", err)
		}

		// Convert legacy knowledgebase.KnowledgeBase to the internal type if needed
		// or use the response format.
		var kbs []knowledgebase.KnowledgeBase = kbsResp

		// Auto-include tenant-wide KBs that are not explicitly linked
		allKbs, _, err := b.kbs.List(ctx, pagination.PageRequest{Page: 0, Size: 100})
		if err == nil {
			for _, kbItem := range allKbs {
				exists := false
				for _, existing := range kbs {
					if existing.ID == kbItem.ID {
						exists = true
						break
					}
				}
				if !exists {
					kbs = append(kbs, kbItem)
				}
			}
		}

		if len(kbs) > 0 {
			tools = append(tools, documentSearchTool(kbs))
		}
	}

	// Builtin: memory_store — always available.
	tools = append(tools, memoryStoreTool())

	// Builtin: ask_user — always available; lets the LLM request structured input from the user.
	tools = append(tools, askUserTool())

	// Builtin: agent — available when depth < maxDepth (enables sub-agent spawning).
	if b.currentDepth < b.maxDepth {
		tools = append(tools, agentTool(b.maxDepth-b.currentDepth))
	}

	// Builtin: send_message — available to sub-agents (depth > 0) for inter-agent messaging.
	if b.currentDepth > 0 {
		tools = append(tools, sendMessageTool())
	}

	// MCP tools — fetched from external MCP servers via the bridge.
	if b.mcpBridge != nil {
		mcpTools, err := b.mcpBridge.ListTools(ctx)
		if err != nil {
			// Non-fatal: log and continue without MCP tools.
			_ = err
		} else {
			tools = append(tools, mcpTools...)
		}
	}

	if tools == nil {
		tools = []LLMTool{}
	}

	// Stable sort: builtins as contiguous prefix, skill/MCP tools as suffix.
	// Both partitions are sorted alphabetically within their group.
	// This prevents external tools from interleaving with builtins and
	// invalidating the LLM's prompt cache prefix on every MCP tool change.
	// Inspired by Claude Code's assembleToolPool (tools.ts).
	sort.SliceStable(tools, func(i, j int) bool {
		if tools[i].Builtin != tools[j].Builtin {
			return tools[i].Builtin // builtins first
		}
		return tools[i].Name < tools[j].Name
	})

	return tools, nil
}

// BuildWithDeferred returns loaded and deferred tools separately.
// When the total tool count exceeds DeferredToolThreshold, tools marked
// ShouldDefer=true are separated and require tool_search to load.
// The tool_search builtin is automatically added when deferred tools exist.
//
// Inspired by Claude Code's isDeferredTool + ToolSearchTool pattern.
func (b *ToolSchemaBuilder) BuildWithDeferred(ctx context.Context, agentID uuid.UUID) (*ToolBuildResult, error) {
	all, err := b.Build(ctx, agentID)
	if err != nil {
		return nil, err
	}

	// Only activate deferred loading when there are enough tools.
	if len(all) < DeferredToolThreshold {
		return &ToolBuildResult{Loaded: all, All: all, UserOnlySkills: b.lastUserOnlySkills}, nil
	}

	var loaded, deferred []LLMTool
	for _, t := range all {
		// Builtins and always-load tools are never deferred — they must be available on turn 1.
		// Inspired by Claude Code's alwaysLoad flag (Tool.ts).
		if t.ShouldDefer && !t.Builtin && !t.AlwaysLoad {
			deferred = append(deferred, t)
		} else {
			loaded = append(loaded, t)
		}
	}

	// Add tool_search builtin when there are deferred tools.
	if len(deferred) > 0 {
		loaded = append(loaded, toolSearchTool(deferred))
	}

	return &ToolBuildResult{
		Loaded:         loaded,
		Deferred:       deferred,
		All:            all,
		UserOnlySkills: b.lastUserOnlySkills,
	}, nil
}

// DeferredToolNames returns the names of deferred tools for system message injection.
func (r *ToolBuildResult) DeferredToolNames() []string {
	names := make([]string, len(r.Deferred))
	for i, t := range r.Deferred {
		names[i] = t.Name
	}
	return names
}

// skillToLLMTool converts a single skill (plus its first active tool's config) into an LLMTool.
func (b *ToolSchemaBuilder) skillToLLMTool(ctx context.Context, sk skill.Skill) (LLMTool, error) {
	// Prefer the description stored in the database so skill behaviour can be
	// adjusted without redeploying. Fall back to the legacy static catalog only
	// when the DB description is empty.
	description := strings.TrimSpace(sk.Description)
	if description == "" {
		description = EnrichDescription(sk.Slug, sk.Description)
	}
	if description == "" {
		description = sk.Name
	}

	// 4. Use instructions as part of the description if available.
	if sk.Instructions != "" {
		description = fmt.Sprintf("%s\n\nInstructions:\n%s", description, sk.Instructions)
	}

	inputSchema := json.RawMessage(`{"type":"object","properties":{}}`)
	readOnly := IsReadOnlyTool(sk.Slug)
	shouldDefer := false
	isDestructive := false
	searchHint := ""
	alwaysLoad := false
	concurrencySafe := false
	maxResultChars := 0
	interruptBehavior := ""
	isSearchOrRead := false

	// Load bound tools once — used for schema derivation, read-only, deferred, and destructive detection.
	if b.tools != nil {
		bindings, boundTools, err := b.tools.ListBySkill(ctx, sk.ID)
		if err != nil {
			return LLMTool{}, fmt.Errorf("toolschema: list tools for skill %s: %w", sk.Slug, err)
		}
		for i, bt := range bindings {
			if bt.IsActive && i < len(boundTools) {
				// Derive schema from tool config.
				derived := deriveSchemaFromToolConfig(boundTools[i])
				if len(derived) > 0 {
					inputSchema = derived
				}
				// Use DB flags from bound tool.
				if boundTools[i].ReadOnly {
					readOnly = true
				}
				if boundTools[i].ShouldDefer {
					shouldDefer = true
				}
				if boundTools[i].IsDestructive {
					isDestructive = true
				}
				if boundTools[i].SearchHint != nil && *boundTools[i].SearchHint != "" {
					searchHint = *boundTools[i].SearchHint
				}
				if boundTools[i].AlwaysLoad {
					alwaysLoad = true
				}
				if boundTools[i].ConcurrencySafe != nil && *boundTools[i].ConcurrencySafe {
					concurrencySafe = true
				}
				if boundTools[i].MaxResultChars != nil && *boundTools[i].MaxResultChars > 0 {
					maxResultChars = *boundTools[i].MaxResultChars
				}
				if boundTools[i].InterruptBehavior != nil && *boundTools[i].InterruptBehavior != "" {
					interruptBehavior = *boundTools[i].InterruptBehavior
				}
				if boundTools[i].IsSearchOrRead {
					isSearchOrRead = true
				}
				break
			}
		}
	}

	if len(inputSchema) == 0 {
		inputSchema = json.RawMessage(`{"type":"object","properties":{}}`)
	}

	return LLMTool{
		Name:                   sk.Slug,
		Description:            description,
		InputSchema:            inputSchema,
		ReadOnly:               readOnly,
		MaxResultChars:         maxResultChars,
		ShouldDefer:            shouldDefer,
		IsDestructive:          isDestructive,
		SearchHint:             searchHint,
		AllowedTools:           sk.AllowedTools,
		AlwaysLoad:             alwaysLoad,
		DisableModelInvocation: sk.DisableModelInvocation,
		ConcurrencySafe:        concurrencySafe,
		InterruptBehavior:      interruptBehavior,
		IsSearchOrRead:         isSearchOrRead,
	}, nil
}

// normaliseSchema ensures the raw bytes are a valid JSON Schema object, or returns nil.
func normaliseSchema(raw []byte) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	// Must have "type" to be a valid JSON Schema.
	if _, ok := obj["type"]; !ok {
		return nil
	}
	return json.RawMessage(raw)
}

// deriveSchemaFromToolConfig attempts to extract a usable input schema from a
// tool's config JSON. This covers cases where the skill has no explicit
// inputSchema but the underlying tool config defines parameter shapes.
func deriveSchemaFromToolConfig(t tool.Tool) json.RawMessage {
	if len(t.Config) == 0 {
		return nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(t.Config, &cfg); err != nil {
		return nil
	}
	// If the tool config already has an "inputSchema" field, use it.
	if raw, ok := cfg["inputSchema"]; ok {
		if data, err := json.Marshal(raw); err == nil {
			return data
		}
	}
	return nil
}

// documentSearchTool returns the builtin document_search tool definition.
func documentSearchTool(kbs []knowledgebase.KnowledgeBase) LLMTool {
	var names []string
	for _, kb := range kbs {
		names = append(names, kb.Name)
	}
	desc := fmt.Sprintf(
		"Search documents in the knowledge base(s): %s. "+
			"Use this when the user asks about information contained in the knowledge bases. "+
			"The query parameter should be a descriptive sentence, not keywords.",
		joinNames(names),
	)
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Semantic search query"
			},
			"knowledge_base_id": {
				"type": "string",
				"description": "Optional UUID of a specific knowledge base to search"
			},
			"limit": {
				"type": "integer",
				"description": "Maximum number of results to return (default 5)"
			}
		},
		"required": ["query"]
	}`)
	return LLMTool{
		Name:        "document_search",
		Description: desc,
		InputSchema: schema,
		ReadOnly:    true,
		Builtin:     true,
	}
}

// memoryStoreTool returns the builtin memory_store tool definition.
func memoryStoreTool() LLMTool {
	return LLMTool{
		Name:        "memory_store",
		Builtin:     true,
		Description: "Store a piece of information for long-term recall across sessions. Use this to remember important facts, preferences, or decisions the user shares.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"content": {
					"type": "string",
					"description": "The information to remember"
				},
				"category": {
					"type": "string",
					"description": "Category for the memory (e.g. preference, fact, decision)"
				}
			},
			"required": ["content"]
		}`),
	}
}

// askUserTool returns the builtin ask_user tool definition.
// This tool lets the LLM request structured input from the user at any point
// during a run. The runner intercepts calls to this tool and routes them through
// the ElicitationHandler, which blocks until the user submits the form.
func askUserTool() LLMTool {
	return LLMTool{
		Name:     "ask_user",
		Builtin:  true,
		ReadOnly: true,
		Description: `Present a structured form to collect user input. You MUST use this tool whenever you need ANY information — never ask via plain text.

Parameters:
- message: Brief instruction shown above the form.
- questions: Array of question objects, each rendered as a form field.

Each question object:
- id: Field identifier (snake_case).
- question: Label displayed to the user.
- type: "text" (default), "select", or "confirm".
- required: Whether the field is mandatory (default true).
- options: Array of {label, description} for "select" type. 2-6 options. User can always type a custom value.

RULES:
- When the field has a known set of valid values (categories, statuses, types, environments), ALWAYS use type "select" with options.
- For each option, include a short description explaining when to choose it.
- Group related fields in a single ask_user call (1-6 questions).
- Use concise labels — the description carries the detail.

Example — creating a skill:
ask_user(
  message="Skill details",
  questions=[
    {id:"name", question:"Skill name", type:"text"},
    {id:"description", question:"Description", type:"text"},
    {id:"category", question:"Category", type:"select", options:[
      {label:"rag", description:"Document search and retrieval"},
      {label:"data", description:"SQL queries and data analysis"},
      {label:"integration", description:"HTTP APIs and external services"},
      {label:"platform", description:"AgentHub management operations"},
      {label:"compute", description:"Code execution and calculations"}
    ]}
  ]
)`,
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"message": {
					"type": "string",
					"description": "Brief instruction shown above the form"
				},
				"questions": {
					"type": "array",
					"description": "Form fields to present to the user",
					"items": {
						"type": "object",
						"properties": {
							"id": {
								"type": "string",
								"description": "Field identifier (snake_case)"
							},
							"question": {
								"type": "string",
								"description": "Label displayed to the user"
							},
							"type": {
								"type": "string",
								"enum": ["text", "select", "confirm"],
								"description": "Field type: text for free input, select for choices, confirm for yes/no"
							},
							"required": {
								"type": "boolean",
								"description": "Whether the field is mandatory (default true)"
							},
							"options": {
								"type": "array",
								"description": "Choices for select type. 2-6 options.",
								"items": {
									"type": "object",
									"properties": {
										"label": {"type": "string", "description": "Option value shown to user"},
										"description": {"type": "string", "description": "Brief explanation of this option"}
									},
									"required": ["label"]
								}
							}
						},
						"required": ["id", "question"]
					}
				}
			},
			"required": ["message", "questions"]
		}`),
	}
}

// agentTool returns the builtin agent tool for sub-agent spawning.
func agentTool(remainingLevels int) LLMTool {
	desc := fmt.Sprintf(
		"Spawn a sub-agent to handle a specific subtask autonomously. "+
			"The sub-agent inherits all your tools, knowledge bases, and permissions. "+
			"Use this to delegate complex subtasks that can be worked on independently. "+
			"The sub-agent will return its result as text. "+
			"You can spawn multiple sub-agents in parallel for independent tasks. "+
			"Remaining delegation depth: %d level(s).",
		remainingLevels,
	)
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"prompt": {
				"type": "string",
				"description": "Clear, specific description of the subtask for the sub-agent to complete"
			},
			"tools": {
				"type": "array",
				"items": {"type": "string"},
				"description": "Optional list of specific tool names the sub-agent should use. If omitted, all tools are available."
			}
		},
		"required": ["prompt"]
	}`)
	return LLMTool{
		Name:        agentToolName,
		Description: desc,
		InputSchema: schema,
		Builtin:     true,
	}
}

// toolSearchTool returns the builtin tool_search tool for loading deferred tools.
// Inspired by Claude Code's ToolSearchTool — enables the LLM to load tool schemas
// on demand rather than including all schemas in the initial prompt.
func toolSearchTool(deferred []LLMTool) LLMTool {
	var names []string
	for _, t := range deferred {
		names = append(names, t.Name)
	}
	desc := fmt.Sprintf(
		"Fetches full schema definitions for deferred tools so they can be called. "+
			"The following tools are available but require loading first: %s. "+
			"Use the 'query' parameter with a tool name to load its schema, "+
			"or use keywords to search for tools by capability.",
		join(names, ", "),
	)
	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"query": {
				"type": "string",
				"description": "Tool name to load, or keywords to search (e.g. 'create agent', 'delete knowledge base')"
			},
			"max_results": {
				"type": "integer",
				"description": "Maximum number of tools to return (default 5)"
			}
		},
		"required": ["query"]
	}`)
	return LLMTool{
		Name:        "tool_search",
		Description: desc,
		InputSchema: schema,
		ReadOnly:    true,
		Builtin:     true,
	}
}

// readOnlySlugs lists skill slugs that are known to be read-only (no side effects).
// Used to determine concurrency safety during tool execution.
var readOnlySlugs = map[string]bool{
	"document-search": true,
	"document_search": true, // builtin uses underscore
	"web-scraper":     true,
	"http-get":        true,
	"memory-recall":   true,
	"troubleshoot":    true, // diagnosis only, no side effects
	"tool_search":     true, // builtin, no side effects
	"ask_user":        true, // builtin, only collects user input — no side effects
}

// IsReadOnlyTool returns true if the tool name is known to be read-only.
func IsReadOnlyTool(name string) bool {
	return readOnlySlugs[name]
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return names[0]
	}
	return fmt.Sprintf("%s and %s", join(names[:len(names)-1], ", "), names[len(names)-1])
}

func join(elems []string, sep string) string {
	result := ""
	for i, e := range elems {
		if i > 0 {
			result += sep
		}
		result += e
	}
	return result
}
