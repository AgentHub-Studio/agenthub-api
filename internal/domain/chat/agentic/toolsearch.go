package agentic

import (
	"encoding/json"
	"fmt"
	"strings"
)

// toolSearchName is the builtin tool name for on-demand schema loading.
const toolSearchName = "tool_search"

// IsToolSearchCall returns true if the tool call targets the tool_search builtin.
func IsToolSearchCall(name string) bool {
	return name == toolSearchName
}

// toolSearchInput represents the parsed input for a tool_search call.
type toolSearchInput struct {
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

// ExecuteToolSearch resolves a tool_search call locally by matching deferred tools
// by name or keyword. Returns a ToolExecResult with the matching tool schemas as JSON.
//
// Matching strategy (in priority order):
// 1. Exact name match (e.g., query="agenthub_create_agent")
// 2. Prefix "select:" for explicit multi-tool selection (e.g., "select:tool1,tool2")
// 3. Keyword search against tool name, description, and search_hint
//
// Inspired by Claude Code's ToolSearchTool which fetches deferred tool schemas on demand.
func ExecuteToolSearch(input json.RawMessage, deferred []LLMTool) ToolExecResult {
	var parsed toolSearchInput
	if err := json.Unmarshal(input, &parsed); err != nil {
		errMsg := fmt.Sprintf("Invalid input for tool_search: %s", err.Error())
		return ToolExecResult{Error: &errMsg, ToolName: toolSearchName}
	}

	if parsed.Query == "" {
		errMsg := "The required parameter 'query' is missing for tool_search."
		return ToolExecResult{Error: &errMsg, ToolName: toolSearchName}
	}

	maxResults := parsed.MaxResults
	if maxResults <= 0 {
		maxResults = 5
	}

	var matches []LLMTool

	// Strategy 1: "select:name1,name2" explicit selection.
	if strings.HasPrefix(parsed.Query, "select:") {
		names := strings.Split(strings.TrimPrefix(parsed.Query, "select:"), ",")
		nameSet := make(map[string]bool, len(names))
		for _, n := range names {
			nameSet[strings.TrimSpace(n)] = true
		}
		for _, t := range deferred {
			if nameSet[t.Name] {
				matches = append(matches, t)
			}
		}
	} else {
		// Strategy 2: Exact name match.
		for _, t := range deferred {
			if t.Name == parsed.Query {
				matches = append(matches, t)
				break
			}
		}

		// Strategy 3: Keyword search (if no exact match).
		if len(matches) == 0 {
			query := strings.ToLower(parsed.Query)
			keywords := strings.Fields(query)

			type scored struct {
				tool  LLMTool
				score int
			}
			var candidates []scored

			for _, t := range deferred {
				score := 0
				searchable := strings.ToLower(t.Name + " " + t.Description + " " + t.SearchHint)
				for _, kw := range keywords {
					if strings.Contains(searchable, kw) {
						score++
					}
				}
				if score > 0 {
					candidates = append(candidates, scored{tool: t, score: score})
				}
			}

			// Sort by score descending (simple insertion sort for small N).
			for i := 1; i < len(candidates); i++ {
				for j := i; j > 0 && candidates[j].score > candidates[j-1].score; j-- {
					candidates[j], candidates[j-1] = candidates[j-1], candidates[j]
				}
			}

			for i, c := range candidates {
				if i >= maxResults {
					break
				}
				matches = append(matches, c.tool)
			}
		}
	}

	if len(matches) == 0 {
		output, _ := json.Marshal(map[string]any{
			"message": fmt.Sprintf("No deferred tools found matching '%s'. Available deferred tools: %s",
				parsed.Query, deferredNames(deferred)),
			"tools": []any{},
		})
		return ToolExecResult{Output: output, ToolName: toolSearchName}
	}

	// Build response with full schemas.
	type toolDef struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		InputSchema json.RawMessage `json:"input_schema"`
	}
	defs := make([]toolDef, len(matches))
	for i, m := range matches {
		defs[i] = toolDef{
			Name:        m.Name,
			Description: m.Description,
			InputSchema: m.InputSchema,
		}
	}

	output, _ := json.Marshal(map[string]any{
		"message": fmt.Sprintf("Found %d tool(s). You can now call them directly.", len(defs)),
		"tools":   defs,
	})
	return ToolExecResult{Output: output, ToolName: toolSearchName}
}

// deferredNames returns a comma-separated list of deferred tool names.
func deferredNames(deferred []LLMTool) string {
	names := make([]string, len(deferred))
	for i, t := range deferred {
		names[i] = t.Name
	}
	return strings.Join(names, ", ")
}
