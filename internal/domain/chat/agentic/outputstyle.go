package agentic

import (
	"encoding/json"
	"fmt"
	"strings"
)

// OutputStyleSource identifies where an output style configuration comes from.
type OutputStyleSource string

const (
	OutputStyleBuiltIn  OutputStyleSource = "built-in"
	OutputStyleUser     OutputStyleSource = "user"
	OutputStylePlugin   OutputStyleSource = "plugin"
	OutputStyleAgent    OutputStyleSource = "agent"
)

// OutputStyleConfig defines a pluggable output persona that modifies the LLM's
// response behavior by injecting additional instructions into the system prompt.
//
// Inspired by Claude Code's OutputStyleConfig in constants/outputStyles.ts.
type OutputStyleConfig struct {
	// Name is the display name of the style.
	Name string `json:"name"`
	// Description is a brief explanation of what this style does.
	Description string `json:"description,omitempty"`
	// Prompt is the instruction text injected into the system prompt.
	Prompt string `json:"prompt"`
	// Source identifies where this style was defined.
	Source OutputStyleSource `json:"source"`
	// KeepCodingInstructions when true preserves the default coding instructions
	// alongside the style prompt. When false, the style completely replaces them.
	KeepCodingInstructions bool `json:"keepCodingInstructions,omitempty"`
}

// DefaultOutputStyleName is the key for the baseline (no-modification) style.
const DefaultOutputStyleName = "default"

// builtinOutputStyles provides the standard output styles available to all agents.
//
// Inspired by Claude Code's OUTPUT_STYLE_CONFIG.
var builtinOutputStyles = map[string]*OutputStyleConfig{
	DefaultOutputStyleName: nil, // baseline — no modification
	"explanatory": {
		Name:        "Explanatory",
		Description: "Provides educational insights about implementation choices",
		Prompt: `When responding, include brief educational insights about your implementation choices.
Explain WHY you chose a particular approach, not just WHAT you did.
Use *** delimited sections for educational asides when they add value.
Keep explanations concise — one or two sentences per insight.`,
		Source:                 OutputStyleBuiltIn,
		KeepCodingInstructions: true,
	},
	"concise": {
		Name:        "Concise",
		Description: "Minimal output — code and essential context only",
		Prompt: `Be extremely concise. Output code changes and essential context only.
No explanations unless the user explicitly asks.
No summaries of what you did — the diff speaks for itself.
Skip pleasantries, transitions, and filler.`,
		Source:                 OutputStyleBuiltIn,
		KeepCodingInstructions: true,
	},
	"learning": {
		Name:        "Learning",
		Description: "Collaborative learning — explains concepts and invites practice",
		Prompt: `Adopt a collaborative learning style. When implementing:
- Explain key concepts and patterns you're applying
- Point out learning opportunities in the codebase
- Suggest small exercises the user could try to deepen understanding
- Use questions to encourage critical thinking about design decisions`,
		Source:                 OutputStyleBuiltIn,
		KeepCodingInstructions: true,
	},
}

// OutputStyleRegistry manages the available output styles for an agent.
// Styles are resolved in priority order: agent-specific > user > plugin > built-in.
type OutputStyleRegistry struct {
	styles map[string]*OutputStyleConfig
}

// NewOutputStyleRegistry creates a registry pre-populated with built-in styles.
func NewOutputStyleRegistry() *OutputStyleRegistry {
	reg := &OutputStyleRegistry{
		styles: make(map[string]*OutputStyleConfig),
	}
	for k, v := range builtinOutputStyles {
		reg.styles[k] = v
	}
	return reg
}

// Register adds or overwrites a style in the registry.
func (r *OutputStyleRegistry) Register(key string, config *OutputStyleConfig) {
	r.styles[strings.ToLower(key)] = config
}

// Get retrieves a style by key. Returns nil if the key is "default" or unknown.
func (r *OutputStyleRegistry) Get(key string) *OutputStyleConfig {
	return r.styles[strings.ToLower(key)]
}

// List returns all registered style names (excluding "default").
func (r *OutputStyleRegistry) List() []string {
	var names []string
	for k, v := range r.styles {
		if v != nil {
			names = append(names, k)
		}
	}
	return names
}

// ApplyToSystemPrompt injects the selected output style into a system prompt.
// If the style is nil or "default", returns the original prompt unchanged.
// If KeepCodingInstructions is false, only the style prompt is returned.
func ApplyOutputStyle(basePrompt string, style *OutputStyleConfig) string {
	if style == nil || style.Prompt == "" {
		return basePrompt
	}

	if !style.KeepCodingInstructions {
		return style.Prompt
	}

	return fmt.Sprintf("%s\n\n## Output Style: %s\n\n%s", basePrompt, style.Name, style.Prompt)
}

// --- Conversation Recovery ---

// ConversationRecoveryResult holds the output of recovering/resuming a conversation.
//
// Inspired by Claude Code's DeserializeResult in conversationRecovery.ts.
type ConversationRecoveryResult struct {
	// Messages are the recovered and normalized messages ready for the next query.
	Messages json.RawMessage `json:"messages"`
	// TurnInterruptionKind describes if/how the conversation was interrupted.
	// "none" means clean end; "interrupted_prompt" means user had unsent input.
	TurnInterruptionKind string `json:"turnInterruptionKind"`
	// InterruptedMessage is the unsent user message (if interrupted).
	InterruptedMessage string `json:"interruptedMessage,omitempty"`
}

// RecoverConversationMessages filters and normalizes messages for session resume.
// It removes:
// - Thinking-only messages without content
// - Unresolved tool use blocks (tool_use without matching tool_result)
// - Whitespace-only messages
// - Synthetic/internal messages not meant for the API
//
// Inspired by Claude Code's conversationRecovery.ts recoverConversation().
func RecoverConversationMessages(messages json.RawMessage) (json.RawMessage, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	var parsed []map[string]interface{}
	if err := json.Unmarshal(messages, &parsed); err != nil {
		return messages, err // return original on parse error
	}

	var recovered []map[string]interface{}
	toolUseIDs := make(map[string]bool)
	toolResultIDs := make(map[string]bool)

	// First pass: collect tool_use and tool_result IDs.
	for _, msg := range parsed {
		role, _ := msg["role"].(string)
		if role == "assistant" {
			if toolCalls, ok := msg["toolCalls"].([]interface{}); ok {
				for _, tc := range toolCalls {
					if tcMap, ok := tc.(map[string]interface{}); ok {
						if id, ok := tcMap["id"].(string); ok {
							toolUseIDs[id] = true
						}
					}
				}
			}
		}
		if role == "tool" {
			if id, ok := msg["toolCallId"].(string); ok {
				toolResultIDs[id] = true
			}
		}
	}

	// Second pass: filter messages.
	for _, msg := range parsed {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)

		// Skip whitespace-only messages (unless they have tool calls).
		if strings.TrimSpace(content) == "" {
			hasToolCalls := false
			if tc, ok := msg["toolCalls"]; ok && tc != nil {
				hasToolCalls = true
			}
			if role == "tool" {
				hasToolCalls = true // tool results are always kept
			}
			if !hasToolCalls {
				continue
			}
		}

		// Skip thinking-only messages.
		if role == "assistant" && content == "" {
			meta, _ := msg["metadata"].(map[string]interface{})
			if meta != nil {
				if _, hasThinking := meta["thinkingContent"]; hasThinking {
					// Thinking-only without content — skip.
					if tc, ok := msg["toolCalls"]; !ok || tc == nil {
						continue
					}
				}
			}
		}

		recovered = append(recovered, msg)
	}

	if len(recovered) == 0 {
		return json.Marshal([]interface{}{})
	}
	return json.Marshal(recovered)
}
