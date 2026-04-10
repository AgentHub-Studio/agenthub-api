package tool

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ToolType represents the implementation type of a tool.
type ToolType = string

const (
	ToolTypeHTTP           ToolType = "HTTP"
	ToolTypeSQL            ToolType = "SQL"
	ToolTypeDocumentSearch ToolType = "DOCUMENT_SEARCH"
	ToolTypeCustom         ToolType = "CUSTOM"
	ToolTypeBlockly        ToolType = "BLOCKLY"
	ToolTypeComposite      ToolType = "COMPOSITE"
	// ToolTypeCode represents a scripted code tool (Groovy/Python/JS).
	ToolTypeCode      ToolType = "CODE"
	// ToolTypeDatabase is an alias for SQL (used by the frontend).
	ToolTypeDatabase  ToolType = "DATABASE"
	// ToolTypeDocuments is an alias for DOCUMENT_SEARCH (used by the frontend).
	ToolTypeDocuments ToolType = "DOCUMENTS"
)

// validToolTypes is used for type validation.
var validToolTypes = map[ToolType]bool{
	ToolTypeHTTP:           true,
	ToolTypeSQL:            true,
	ToolTypeDocumentSearch: true,
	ToolTypeCustom:         true,
	ToolTypeBlockly:        true,
	ToolTypeComposite:      true,
	ToolTypeCode:           true,
	ToolTypeDatabase:       true,
	ToolTypeDocuments:      true,
}

// IsValidToolType returns true when the given type string is a known ToolType.
func IsValidToolType(t string) bool {
	return validToolTypes[t]
}

// Tool is a concrete implementation of a skill capability.
type Tool struct {
	ID            uuid.UUID       `db:"id"`
	Name          string          `db:"name"`
	Type          ToolType        `db:"type"`
	Config        []byte          `db:"config"`
	// InputSchema is an explicit JSON Schema for this tool's input parameters.
	// When set, the runner uses this schema verbatim instead of auto-deriving one
	// from the tool config. P-C175-1/P-C175-2.
	InputSchema   json.RawMessage `db:"input_schema"`
	Description   string          `db:"description"`
	Labels        []string        `db:"labels"`
	ReadOnly      bool            `db:"read_only"`
	// ShouldDefer marks tools whose full schema should not be sent to the LLM
	// in the initial prompt. The LLM loads them on demand via tool_search.
	// Inspired by Claude Code's shouldDefer flag (Tool.ts).
	ShouldDefer   bool      `db:"should_defer"`
	// IsDestructive flags tools that perform irreversible operations.
	// Used to auto-require confirmation even in permissive modes.
	// Inspired by Claude Code's isDestructive per-tool flag (Tool.ts).
	IsDestructive bool      `db:"is_destructive"`
	// SearchHint is a short keyword phrase for tool_search matching.
	// Inspired by Claude Code's searchHint per-tool string (Tool.ts).
	SearchHint    *string   `db:"search_hint"`
	// AlwaysLoad prevents this tool from being deferred even when ToolSearch
	// is active. Use for MCP tools that must be available on turn 1.
	// Inspired by Claude Code's alwaysLoad flag (Tool.ts).
	AlwaysLoad    bool      `db:"always_load"`
	// ConcurrencySafe indicates this tool can run in parallel with other tools,
	// even if it's not strictly read-only. When nil, falls back to ReadOnly.
	// Inspired by Claude Code's isConcurrencySafe per-tool flag (Tool.ts).
	ConcurrencySafe *bool   `db:"concurrency_safe"`
	// MaxResultChars overrides the global max result size for this tool.
	// When nil, uses the global DefaultMaxResultSizeChars.
	// Inspired by Claude Code's per-tool maxResultSizeChars.
	MaxResultChars  *int    `db:"max_result_chars"`
	// InterruptBehavior controls what happens when the user interrupts while this
	// tool is running. "block" waits for completion (use for destructive/long-running
	// tools); nil/"cancel" aborts immediately (safe for read-only tools).
	// Inspired by Claude Code's Tool.ts interruptBehavior(): 'cancel' | 'block'.
	InterruptBehavior *string `db:"interrupt_behavior"`
	// IsSearchOrRead marks tools whose results should auto-collapse in the UI.
	// Set for list, search, and read-only query tools where users prefer the
	// synthesized answer over the raw result.
	// Inspired by Claude Code's Tool.ts isSearchOrReadCommand().
	IsSearchOrRead bool `db:"is_search_or_read"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

// SkillTool represents the binding between a skill and a tool.
type SkillTool struct {
	ID        uuid.UUID `db:"id"`
	SkillID   uuid.UUID `db:"skill_id"`
	ToolID    uuid.UUID `db:"tool_id"`
	Priority  int       `db:"priority"`
	IsActive  bool      `db:"is_active"`
	CreatedAt time.Time `db:"created_at"`
}
