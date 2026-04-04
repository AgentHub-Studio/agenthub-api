package agentic

import (
	"fmt"
	"sort"
)

// Context window optimization suggestions.
//
// Inspired by Claude Code's contextSuggestions.ts — analyzes
// context usage patterns and generates actionable suggestions
// to reduce token consumption. Detects large tool results,
// read bloat, memory bloat, near-capacity, and auto-compact
// status.

// CtxSeverity indicates the urgency of a context suggestion.
type CtxSeverity string

const (
	CtxSeverityInfo    CtxSeverity = "info"
	CtxSeverityWarning CtxSeverity = "warning"
)

// ContextSuggestion is an actionable recommendation for
// reducing context window usage.
type ContextSuggestion struct {
	Severity      CtxSeverity `json:"severity"`
	Title         string             `json:"title"`
	Detail        string             `json:"detail"`
	SavingsTokens int                `json:"savingsTokens,omitempty"`
}

// ToolUsageStats tracks token usage for a specific tool type.
type ToolUsageStats struct {
	Name         string `json:"name"`
	CallTokens   int    `json:"callTokens"`
	ResultTokens int    `json:"resultTokens"`
}

// MemoryFileStats tracks token usage for a memory file.
type MemoryFileStats struct {
	Path   string `json:"path"`
	Tokens int    `json:"tokens"`
}

// MessageBreakdown provides per-tool-type token usage.
type MessageBreakdown struct {
	ToolCallsByType []ToolUsageStats `json:"toolCallsByType"`
}

// ContextAnalysisData is the input for context suggestion generation.
type ContextAnalysisData struct {
	Percentage           int               `json:"percentage"`
	RawMaxTokens         int               `json:"rawMaxTokens"`
	IsAutoCompactEnabled bool              `json:"isAutoCompactEnabled"`
	MessageBreakdown     *MessageBreakdown `json:"messageBreakdown,omitempty"`
	MemoryFiles          []MemoryFileStats `json:"memoryFiles,omitempty"`
}

// Thresholds for triggering context suggestions.
const (
	largeToolResultPercent = 15
	largeToolResultTokens  = 10_000
	readBloatPercent       = 5
	nearCapacityPercent    = 80
	memoryHighPercent      = 5
	memoryHighTokens       = 5_000
)

// Well-known tool names for tool-specific advice.
const (
	toolNameBash     = "Bash"
	toolNameRead     = "Read"
	toolNameGrep     = "Grep"
	toolNameWebFetch = "WebFetch"
)

// GenerateContextSuggestions analyzes context usage and returns
// prioritized suggestions sorted by severity (warnings first)
// then by potential savings descending.
func GenerateContextSuggestions(data ContextAnalysisData) []ContextSuggestion {
	var suggestions []ContextSuggestion

	checkNearCapacity(data, &suggestions)
	checkLargeToolResults(data, &suggestions)
	checkReadResultBloat(data, &suggestions)
	checkMemoryBloat(data, &suggestions)
	checkAutoCompactDisabled(data, &suggestions)

	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].Severity != suggestions[j].Severity {
			return suggestions[i].Severity == CtxSeverityWarning
		}
		return suggestions[i].SavingsTokens > suggestions[j].SavingsTokens
	})

	return suggestions
}

func checkNearCapacity(data ContextAnalysisData, out *[]ContextSuggestion) {
	if data.Percentage < nearCapacityPercent {
		return
	}
	detail := "Autocompact will trigger soon, which discards older messages. Use /compact now to control what gets kept."
	if !data.IsAutoCompactEnabled {
		detail = "Autocompact is disabled. Use /compact to free space, or enable autocompact in settings."
	}
	*out = append(*out, ContextSuggestion{
		Severity: CtxSeverityWarning,
		Title:    fmt.Sprintf("Context is %d%% full", data.Percentage),
		Detail:   detail,
	})
}

func checkLargeToolResults(data ContextAnalysisData, out *[]ContextSuggestion) {
	if data.MessageBreakdown == nil || data.RawMaxTokens == 0 {
		return
	}
	for _, tool := range data.MessageBreakdown.ToolCallsByType {
		total := tool.CallTokens + tool.ResultTokens
		pct := float64(total) / float64(data.RawMaxTokens) * 100
		if pct < largeToolResultPercent || total < largeToolResultTokens {
			continue
		}
		if s := largeToolSuggestion(tool.Name, total, pct); s != nil {
			*out = append(*out, *s)
		}
	}
}

func largeToolSuggestion(name string, tokens int, pct float64) *ContextSuggestion {
	tokenStr := FormatTokens(int64(tokens))
	pctStr := fmt.Sprintf("%.0f%%", pct)

	switch name {
	case toolNameBash:
		return &ContextSuggestion{
			Severity:      CtxSeverityWarning,
			Title:         fmt.Sprintf("Bash results using %s tokens (%s)", tokenStr, pctStr),
			Detail:        "Pipe output through head, tail, or grep to reduce result size. Avoid cat on large files — use Read with offset/limit instead.",
			SavingsTokens: tokens / 2,
		}
	case toolNameRead:
		return &ContextSuggestion{
			Severity:      CtxSeverityInfo,
			Title:         fmt.Sprintf("Read results using %s tokens (%s)", tokenStr, pctStr),
			Detail:        "Use offset and limit parameters to read only the sections you need. Avoid re-reading entire files when you only need a few lines.",
			SavingsTokens: tokens * 3 / 10,
		}
	case toolNameGrep:
		return &ContextSuggestion{
			Severity:      CtxSeverityInfo,
			Title:         fmt.Sprintf("Grep results using %s tokens (%s)", tokenStr, pctStr),
			Detail:        "Add more specific patterns or use the glob parameter to narrow file types. Consider Glob for file discovery instead of Grep.",
			SavingsTokens: tokens * 3 / 10,
		}
	case toolNameWebFetch:
		return &ContextSuggestion{
			Severity:      CtxSeverityInfo,
			Title:         fmt.Sprintf("WebFetch results using %s tokens (%s)", tokenStr, pctStr),
			Detail:        "Web page content can be very large. Consider extracting only the specific information needed.",
			SavingsTokens: tokens * 4 / 10,
		}
	default:
		if pct >= 20 {
			return &ContextSuggestion{
				Severity:      CtxSeverityInfo,
				Title:         fmt.Sprintf("%s using %s tokens (%s)", name, tokenStr, pctStr),
				Detail:        "This tool is consuming a significant portion of context.",
				SavingsTokens: tokens / 5,
			}
		}
		return nil
	}
}

func checkReadResultBloat(data ContextAnalysisData, out *[]ContextSuggestion) {
	if data.MessageBreakdown == nil || data.RawMaxTokens == 0 {
		return
	}
	var readTool *ToolUsageStats
	for i := range data.MessageBreakdown.ToolCallsByType {
		if data.MessageBreakdown.ToolCallsByType[i].Name == toolNameRead {
			readTool = &data.MessageBreakdown.ToolCallsByType[i]
			break
		}
	}
	if readTool == nil {
		return
	}

	total := readTool.CallTokens + readTool.ResultTokens
	totalPct := float64(total) / float64(data.RawMaxTokens) * 100
	if totalPct >= largeToolResultPercent && total >= largeToolResultTokens {
		return // already covered by checkLargeToolResults
	}

	readPct := float64(readTool.ResultTokens) / float64(data.RawMaxTokens) * 100
	if readPct >= readBloatPercent && readTool.ResultTokens >= largeToolResultTokens {
		*out = append(*out, ContextSuggestion{
			Severity:      CtxSeverityInfo,
			Title:         fmt.Sprintf("File reads using %s tokens (%.0f%%)", FormatTokens(int64(readTool.ResultTokens)), readPct),
			Detail:        "If you are re-reading files, consider referencing earlier reads. Use offset/limit for large files.",
			SavingsTokens: readTool.ResultTokens * 3 / 10,
		})
	}
}

func checkMemoryBloat(data ContextAnalysisData, out *[]ContextSuggestion) {
	if data.RawMaxTokens == 0 {
		return
	}
	totalTokens := 0
	for _, f := range data.MemoryFiles {
		totalTokens += f.Tokens
	}
	pct := float64(totalTokens) / float64(data.RawMaxTokens) * 100
	if pct < memoryHighPercent || totalTokens < memoryHighTokens {
		return
	}

	// Top 3 largest memory files.
	sorted := make([]MemoryFileStats, len(data.MemoryFiles))
	copy(sorted, data.MemoryFiles)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Tokens > sorted[j].Tokens
	})
	top := sorted
	if len(top) > 3 {
		top = top[:3]
	}
	parts := make([]string, len(top))
	for i, f := range top {
		parts[i] = fmt.Sprintf("%s (%s)", f.Path, FormatTokens(int64(f.Tokens)))
	}

	*out = append(*out, ContextSuggestion{
		Severity:      CtxSeverityInfo,
		Title:         fmt.Sprintf("Memory files using %s tokens (%.0f%%)", FormatTokens(int64(totalTokens)), pct),
		Detail:        fmt.Sprintf("Largest: %s. Review and prune stale entries.", joinComma(parts)),
		SavingsTokens: totalTokens * 3 / 10,
	})
}

func checkAutoCompactDisabled(data ContextAnalysisData, out *[]ContextSuggestion) {
	if data.IsAutoCompactEnabled {
		return
	}
	if data.Percentage < 50 || data.Percentage >= nearCapacityPercent {
		return
	}
	*out = append(*out, ContextSuggestion{
		Severity: CtxSeverityInfo,
		Title:    "Autocompact is disabled",
		Detail:   "Without autocompact, you will hit context limits and lose the conversation. Enable it in settings or use /compact manually.",
	})
}

// joinComma joins strings with ", ".
func joinComma(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}
