package agentic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
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
}

// ToolSchemaBuilder converts the skills/tools linked to an agent into LLMTool
// definitions compatible with the LLM function-calling API.
type ToolSchemaBuilder struct {
	skills SkillLister
	tools  ToolsBySkillLister
	kbs    KBLister
}

// NewToolSchemaBuilder creates a ToolSchemaBuilder.
func NewToolSchemaBuilder(skills SkillLister, tools ToolsBySkillLister, kbs KBLister) *ToolSchemaBuilder {
	return &ToolSchemaBuilder{skills: skills, tools: tools, kbs: kbs}
}

// Build returns the tool definitions for the given agent.
func (b *ToolSchemaBuilder) Build(ctx context.Context, agentID uuid.UUID) ([]LLMTool, error) {
	skills, err := b.skills.ListByAgentID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("toolschema: list skills: %w", err)
	}

	var tools []LLMTool
	for _, sk := range skills {
		t, err := b.skillToLLMTool(ctx, sk)
		if err != nil {
			return nil, err
		}
		tools = append(tools, t)
	}

	// Builtin: document_search — added when the agent has linked knowledge bases.
	if b.kbs != nil {
		kbs, err := b.kbs.ListByAgentID(ctx, agentID)
		if err != nil {
			return nil, fmt.Errorf("toolschema: list knowledge bases: %w", err)
		}
		if len(kbs) > 0 {
			tools = append(tools, documentSearchTool(kbs))
		}
	}

	// Builtin: memory_store — always available.
	tools = append(tools, memoryStoreTool())

	if tools == nil {
		tools = []LLMTool{}
	}
	return tools, nil
}

// skillToLLMTool converts a single skill (plus its first active tool's config) into an LLMTool.
func (b *ToolSchemaBuilder) skillToLLMTool(ctx context.Context, sk skill.Skill) (LLMTool, error) {
	description := sk.Description
	if description == "" {
		description = sk.Name
	}

	inputSchema := normaliseSchema(sk.InputSchema)

	// If the skill has no inputSchema, try to derive one from the first active tool.
	if len(inputSchema) == 0 && b.tools != nil {
		bindings, boundTools, err := b.tools.ListBySkill(ctx, sk.ID)
		if err != nil {
			return LLMTool{}, fmt.Errorf("toolschema: list tools for skill %s: %w", sk.Slug, err)
		}
		for i, bt := range bindings {
			if bt.IsActive && i < len(boundTools) {
				derived := deriveSchemaFromToolConfig(boundTools[i])
				if len(derived) > 0 {
					inputSchema = derived
				}
				break
			}
		}
	}

	if len(inputSchema) == 0 {
		inputSchema = json.RawMessage(`{"type":"object","properties":{}}`)
	}

	return LLMTool{
		Name:        sk.Slug,
		Description: description,
		InputSchema: inputSchema,
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
	}
}

// memoryStoreTool returns the builtin memory_store tool definition.
func memoryStoreTool() LLMTool {
	return LLMTool{
		Name:        "memory_store",
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
