package agentic

// API-round message grouping for multi-turn conversations.
//
// Inspired by Claude Code's groupMessagesByApiRound — groups messages
// at API-round boundaries so each group represents one LLM request-response
// cycle. A new group starts when a new assistant message ID appears.
// Tool-use blocks and their results stay in the same group as long as
// they share the same assistant message ID. This enables per-round
// operations like compaction, summarization, and replay.

// GroupableMessage is a message with a type and optional assistant ID.
type GroupableMessage struct {
	ID          string // unique message ID
	Type        string // "assistant", "user", "tool_result", "system", etc.
	AssistantID string // ID of the assistant message (same across streaming chunks)
}

// GroupMessagesByAPIRound groups messages at API-round boundaries.
// A boundary fires when a new assistant message begins (different
// AssistantID from the prior assistant). This keeps tool_use and
// tool_result pairs within the same group.
func GroupMessagesByAPIRound(messages []GroupableMessage) [][]GroupableMessage {
	if len(messages) == 0 {
		return nil
	}

	var groups [][]GroupableMessage
	var current []GroupableMessage
	var lastAssistantID string
	seenAssistant := false

	for _, msg := range messages {
		if msg.Type == "assistant" &&
			seenAssistant &&
			msg.AssistantID != lastAssistantID &&
			len(current) > 0 {
			groups = append(groups, current)
			current = []GroupableMessage{msg}
		} else {
			current = append(current, msg)
		}

		if msg.Type == "assistant" {
			lastAssistantID = msg.AssistantID
			seenAssistant = true
		}
	}

	if len(current) > 0 {
		groups = append(groups, current)
	}

	return groups
}

// GroupRoundSummary describes a single API round.
type GroupRoundSummary struct {
	AssistantID string
	Messages    int
	HasToolUse  bool
	HasText     bool
}

// SummarizeRounds provides a summary of each group.
func SummarizeRounds(groups [][]GroupableMessage) []GroupRoundSummary {
	summaries := make([]GroupRoundSummary, 0, len(groups))
	for _, group := range groups {
		s := GroupRoundSummary{Messages: len(group)}
		for _, msg := range group {
			if msg.Type == "assistant" && s.AssistantID == "" {
				s.AssistantID = msg.AssistantID
			}
			if msg.Type == "tool_use" || msg.Type == "tool_call" {
				s.HasToolUse = true
			}
			if msg.Type == "assistant" {
				s.HasText = true
			}
		}
		summaries = append(summaries, s)
	}
	return summaries
}
