package agentic

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// parseTextToolCalls extracts tool calls emitted as raw JSON text by LLMs
// that fail to use the provider's structured tool_calls channel.
//
// Observed with openrouter/openai/gpt-oss-120b, which occasionally returns
// payloads like `{"tool":"agenthub_get_agent","call":{"agent_id":"..."}}`
// in the assistant content instead of a proper tool_calls array. Without
// this fallback the tool never executes and the user sees raw JSON.
//
// Returns the parsed tool calls and a bool indicating whether the content
// was fully consumed (i.e. the entire trimmed string was valid tool-call
// JSON). The caller should only replace content/toolCalls when consumed
// is true — partial matches are left as text to avoid silently dropping
// explanatory prose that happens to contain a JSON fragment.
//
// Supported shapes (single object or array):
//   - {"tool":"<name>","call":{...}}             — gpt-oss variant A
//   - {"tool":"<name>","action":{...}}           — gpt-oss variant B
//   - {"tool_name":"<name>","arguments":{...}}   — some local models
//   - {"name":"<name>","arguments":{...}}        — OpenAI-ish
//   - {"name":"<name>","parameters":{...}}       — Gemini-ish
func parseTextToolCalls(content string) (calls []ai.ToolCall, consumed bool) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, false
	}
	if trimmed[0] != '{' && trimmed[0] != '[' {
		return nil, false
	}

	// Try array first.
	if trimmed[0] == '[' {
		var arr []json.RawMessage
		if err := json.Unmarshal([]byte(trimmed), &arr); err != nil {
			return nil, false
		}
		out := make([]ai.ToolCall, 0, len(arr))
		for i, raw := range arr {
			tc, ok := decodeOneToolCall(raw, i)
			if !ok {
				return nil, false
			}
			out = append(out, tc)
		}
		if len(out) == 0 {
			return nil, false
		}
		return out, true
	}

	tc, ok := decodeOneToolCall(json.RawMessage(trimmed), 0)
	if !ok {
		return nil, false
	}
	return []ai.ToolCall{tc}, true
}

// decodeOneToolCall attempts to decode a single JSON object into an ai.ToolCall.
// It probes all known field aliases for name and arguments. The index is used
// to generate a stable synthetic ID when the payload lacks one.
func decodeOneToolCall(raw json.RawMessage, index int) (ai.ToolCall, bool) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return ai.ToolCall{}, false
	}

	name, ok := stringField(m, "tool", "tool_name", "name", "function")
	if !ok || name == "" {
		return ai.ToolCall{}, false
	}

	// Require an explicit args-like field to distinguish tool calls from prose
	// that happens to carry a "name" key (e.g. `{"name":"Alice","age":30}`).
	// A call with no arguments must still declare it explicitly, typically as
	// "call":null or "arguments":{}.
	argsRaw, ok := rawField(m, "call", "action", "arguments", "parameters", "input", "args")
	if !ok {
		return ai.ToolCall{}, false
	}
	// Normalize null → {}.
	if strings.TrimSpace(string(argsRaw)) == "null" {
		argsRaw = json.RawMessage(`{}`)
	}

	// Arguments must be a JSON object or array — reject bare scalars to keep
	// the fallback tight (prose like `{"name":"Alice"}` must not be coerced
	// into a tool call).
	t := strings.TrimSpace(string(argsRaw))
	if t == "" || (t[0] != '{' && t[0] != '[') {
		return ai.ToolCall{}, false
	}

	id := stringFieldOrEmpty(m, "id", "call_id")
	if id == "" {
		id = fmt.Sprintf("call_txt_%d_%s", index, name)
	}

	return ai.ToolCall{
		ID:   id,
		Type: "function",
		Function: ai.ToolFunction{
			Name:      name,
			Arguments: string(argsRaw),
		},
	}, true
}

func stringField(m map[string]json.RawMessage, keys ...string) (string, bool) {
	for _, k := range keys {
		raw, ok := m[k]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil && s != "" {
			return s, true
		}
	}
	return "", false
}

func stringFieldOrEmpty(m map[string]json.RawMessage, keys ...string) string {
	s, _ := stringField(m, keys...)
	return s
}

func rawField(m map[string]json.RawMessage, keys ...string) (json.RawMessage, bool) {
	for _, k := range keys {
		if raw, ok := m[k]; ok {
			return raw, true
		}
	}
	return nil, false
}
