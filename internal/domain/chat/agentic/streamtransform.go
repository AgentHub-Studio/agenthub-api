package agentic

import (
	"fmt"
	"strings"
	"sync"
)

// Stateful streamed message transformer with tool categorization.
//
// Inspired by Claude Code's streamlinedTransform — transforms streaming
// messages by accumulating tool-use statistics, resetting on text boundaries,
// and generating summary messages. Produces a "distilled" output that keeps
// text intact while summarizing tool calls with cumulative counts.

// StreamTransformCategory classifies a tool call.
type StreamTransformCategory string

const (
	StreamCatSearch  StreamTransformCategory = "searches"
	StreamCatRead    StreamTransformCategory = "reads"
	StreamCatWrite   StreamTransformCategory = "writes"
	StreamCatCommand StreamTransformCategory = "commands"
	StreamCatOther   StreamTransformCategory = "other"
)

// StreamTransformCounts tracks tool usage per category.
type StreamTransformCounts struct {
	Searches int
	Reads    int
	Writes   int
	Commands int
	Other    int
}

// Total returns the sum of all categories.
func (c StreamTransformCounts) Total() int {
	return c.Searches + c.Reads + c.Writes + c.Commands + c.Other
}

// StreamTransformOutputType is the kind of transformed output.
type StreamTransformOutputType string

const (
	StreamTransformText        StreamTransformOutputType = "text"
	StreamTransformToolSummary StreamTransformOutputType = "tool_summary"
	StreamTransformResult      StreamTransformOutputType = "result"
	StreamTransformSkipped     StreamTransformOutputType = "skipped"
)

// StreamTransformOutput is the result of transforming one message.
type StreamTransformOutput struct {
	Type        StreamTransformOutputType
	Text        string
	ToolSummary string
	OriginalID  string
}

// ToolCategorizer maps a tool name to a category.
type ToolCategorizer func(toolName string) StreamTransformCategory

// StreamTransformMessage is a raw input message to transform.
type StreamTransformMessage struct {
	ID        string
	Type      string // "assistant", "result", "system", "user", etc.
	Text      string // extracted text content
	ToolNames []string // tool names used in this message
}

// StreamTransformer accumulates tool counts across messages and
// produces summarized output. Text resets the accumulator.
type StreamTransformer struct {
	mu         sync.Mutex
	counts     StreamTransformCounts
	categorize ToolCategorizer
}

// DefaultToolCategorizer returns a categorizer using built-in rules.
func DefaultToolCategorizer(searchTools, readTools, writeTools, commandTools []string) ToolCategorizer {
	searchSet := toSet(searchTools)
	readSet := toSet(readTools)
	writeSet := toSet(writeTools)
	commandSet := toSet(commandTools)

	return func(toolName string) StreamTransformCategory {
		if matchesPrefix(toolName, searchSet) {
			return StreamCatSearch
		}
		if matchesPrefix(toolName, readSet) {
			return StreamCatRead
		}
		if matchesPrefix(toolName, writeSet) {
			return StreamCatWrite
		}
		if matchesPrefix(toolName, commandSet) {
			return StreamCatCommand
		}
		return StreamCatOther
	}
}

func toSet(ss []string) map[string]struct{} {
	m := make(map[string]struct{}, len(ss))
	for _, s := range ss {
		m[s] = struct{}{}
	}
	return m
}

func matchesPrefix(name string, set map[string]struct{}) bool {
	for prefix := range set {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// NewStreamTransformer creates a transformer with the given categorizer.
// If categorizer is nil, all tools are categorized as "other".
func NewStreamTransformer(categorize ToolCategorizer) *StreamTransformer {
	if categorize == nil {
		categorize = func(string) StreamTransformCategory { return StreamCatOther }
	}
	return &StreamTransformer{categorize: categorize}
}

// Transform processes one message and returns the transformed output.
func (t *StreamTransformer) Transform(msg StreamTransformMessage) StreamTransformOutput {
	t.mu.Lock()
	defer t.mu.Unlock()

	switch msg.Type {
	case "assistant":
		// Accumulate tool uses
		for _, name := range msg.ToolNames {
			t.accumulate(name)
		}

		text := strings.TrimSpace(msg.Text)
		if text != "" {
			// Text message: emit text, reset counts
			t.counts = StreamTransformCounts{}
			return StreamTransformOutput{
				Type:       StreamTransformText,
				Text:       text,
				OriginalID: msg.ID,
			}
		}

		// Tool-only message: emit summary
		summary := FormatToolSummary(t.counts)
		if summary == "" {
			return StreamTransformOutput{Type: StreamTransformSkipped, OriginalID: msg.ID}
		}
		return StreamTransformOutput{
			Type:        StreamTransformToolSummary,
			ToolSummary: summary,
			OriginalID:  msg.ID,
		}

	case "result":
		return StreamTransformOutput{Type: StreamTransformResult, OriginalID: msg.ID}

	default:
		return StreamTransformOutput{Type: StreamTransformSkipped, OriginalID: msg.ID}
	}
}

// Counts returns the current accumulated counts.
func (t *StreamTransformer) Counts() StreamTransformCounts {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.counts
}

// Reset clears the accumulated counts.
func (t *StreamTransformer) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts = StreamTransformCounts{}
}

func (t *StreamTransformer) accumulate(toolName string) {
	cat := t.categorize(toolName)
	switch cat {
	case StreamCatSearch:
		t.counts.Searches++
	case StreamCatRead:
		t.counts.Reads++
	case StreamCatWrite:
		t.counts.Writes++
	case StreamCatCommand:
		t.counts.Commands++
	default:
		t.counts.Other++
	}
}

// FormatToolSummary generates a human-readable summary of tool counts.
func FormatToolSummary(c StreamTransformCounts) string {
	var parts []string
	if c.Searches > 0 {
		parts = append(parts, fmt.Sprintf("searched %d %s", c.Searches, plural(c.Searches, "pattern", "patterns")))
	}
	if c.Reads > 0 {
		parts = append(parts, fmt.Sprintf("read %d %s", c.Reads, plural(c.Reads, "file", "files")))
	}
	if c.Writes > 0 {
		parts = append(parts, fmt.Sprintf("wrote %d %s", c.Writes, plural(c.Writes, "file", "files")))
	}
	if c.Commands > 0 {
		parts = append(parts, fmt.Sprintf("ran %d %s", c.Commands, plural(c.Commands, "command", "commands")))
	}
	if c.Other > 0 {
		parts = append(parts, fmt.Sprintf("%d other %s", c.Other, plural(c.Other, "tool", "tools")))
	}
	if len(parts) == 0 {
		return ""
	}
	result := strings.Join(parts, ", ")
	// Capitalize first letter
	return strings.ToUpper(result[:1]) + result[1:]
}

func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return singular
	}
	return pluralForm
}
