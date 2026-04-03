package agentic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// MessagePersister is the subset of chat.Repository used by the Runner to persist messages.
type MessagePersister interface {
	CreateMessage(ctx context.Context, m chat.ChatMessage) (chat.ChatMessage, error)
}

// HistoryLoader loads the conversation history for a session.
type HistoryLoader interface {
	FindAllMessages(ctx context.Context, sessionID uuid.UUID) ([]chat.ChatMessage, error)
}

// RunInput carries everything needed to start an agentic run.
type RunInput struct {
	SessionID    uuid.UUID
	AgentID      uuid.UUID
	UserMessage  string
	SystemPrompt string
	TenantID     string
}

// Runner orchestrates the agentic loop: LLM → tool_calls → execution → tool_results → LLM.
type Runner struct {
	chatModel    ai.ChatModel
	skillClient  *SkillRuntimeClient
	prompt       *PromptBuilder
	tools        *ToolSchemaBuilder
	ctxManager   *ContextManager
	memory       *MemoryBridge
	persister    MessagePersister
	history      HistoryLoader
	config       RunConfig
}

// NewRunner creates a Runner with the given dependencies.
// ctxManager and memory may be nil (features are skipped).
func NewRunner(
	chatModel ai.ChatModel,
	skillClient *SkillRuntimeClient,
	prompt *PromptBuilder,
	tools *ToolSchemaBuilder,
	ctxManager *ContextManager,
	memory *MemoryBridge,
	persister MessagePersister,
	history HistoryLoader,
	config RunConfig,
) *Runner {
	return &Runner{
		chatModel:   chatModel,
		skillClient: skillClient,
		prompt:      prompt,
		tools:       tools,
		ctxManager:  ctxManager,
		memory:      memory,
		persister:   persister,
		history:     history,
		config:      config,
	}
}

// Run starts the agentic loop in a goroutine and returns a channel of events.
// The channel is closed when the run completes or an error occurs.
func (r *Runner) Run(ctx context.Context, in RunInput) <-chan RunEvent {
	ch := make(chan RunEvent, r.config.StreamBufferSize)

	go func() {
		defer close(ch)

		runCtx := ctx
		if r.config.TotalTimeout > 0 {
			var cancel context.CancelFunc
			runCtx, cancel = context.WithTimeout(ctx, r.config.TotalTimeout)
			defer cancel()
		}

		r.runLoop(runCtx, ch, in)
	}()

	return ch
}

func (r *Runner) runLoop(ctx context.Context, ch chan<- RunEvent, in RunInput) {
	// 1. Build system prompt.
	memories := ""
	if r.memory != nil {
		var err error
		memories, err = r.memory.Recall(ctx, in.AgentID, in.UserMessage)
		if err != nil {
			emitError(ch, "memory_recall", err)
			// Non-fatal: continue without memories.
		}
	}

	systemPrompt, err := r.prompt.Build(ctx, PromptInput{
		AgentID:      in.AgentID,
		SessionID:    in.SessionID,
		SystemPrompt: in.SystemPrompt,
		Memories:     memories,
	})
	if err != nil {
		emitError(ch, "prompt_build", err)
		return
	}

	// 2. Build tool schemas.
	llmTools, err := r.tools.Build(ctx, in.AgentID)
	if err != nil {
		emitError(ch, "tool_schema_build", err)
		return
	}
	aiTools := convertLLMToolsToAI(llmTools)

	// 3. Load conversation history.
	messages, err := r.loadHistory(ctx, in.SessionID)
	if err != nil {
		emitError(ch, "load_history", err)
		return
	}

	// 4. Append user message.
	messages = append(messages, ai.Message{
		Role:    ai.RoleUser,
		Content: in.UserMessage,
	})

	// Persist user message.
	if _, err := r.persister.CreateMessage(ctx, chat.ChatMessage{
		SessionID:   in.SessionID,
		Role:        "user",
		Content:     in.UserMessage,
		MessageType: chat.MessageTypeText,
	}); err != nil {
		emitError(ch, "persist_user_msg", err)
		return
	}

	// 5. Agentic loop.
	totalTokens := 0
	totalCost := 0.0
	turnIndex := 0

	for turnIndex < r.config.MaxIterations {
		if err := ctx.Err(); err != nil {
			emitError(ch, "context_cancelled", err)
			return
		}

		opts := ai.ChatOptions{
			Model:       r.config.Model,
			MaxTokens:   r.config.MaxTokensPerCall,
			Temperature: r.config.Temperature,
			Tools:       aiTools,
			Stream:      true,
			SystemMsg:   systemPrompt,
		}

		// 5a. Call LLM with streaming (with retry for transient errors).
		stream, err := retryStream(ctx, r.chatModel, messages, opts, r.config.RetryMaxAttempts)
		if err != nil {
			emitError(ch, "llm_call", err)
			return
		}

		// 5b. Consume stream, accumulate response.
		assistantContent, toolCalls, finishReason, usage, streamErr := r.consumeStream(ctx, ch, stream)
		if streamErr != nil {
			emitError(ch, "stream_consume", streamErr)
			return
		}

		totalTokens += usage.TotalTokens

		// Accumulate cost.
		turnCost := EstimateCostUSD(r.config.Model, usage)
		totalCost += turnCost

		// Budget check.
		if r.config.MaxBudgetUSD > 0 && totalCost > r.config.MaxBudgetUSD {
			emitError(ch, "budget_exceeded", fmt.Errorf("run cost $%.4f exceeded budget $%.4f", totalCost, r.config.MaxBudgetUSD))
			return
		}

		// 5c. Build and persist assistant message.
		assistantMsg := r.buildAssistantMessage(in.SessionID, assistantContent, toolCalls, finishReason, usage, turnIndex)
		if _, err := r.persister.CreateMessage(ctx, assistantMsg); err != nil {
			emitError(ch, "persist_assistant_msg", err)
			return
		}

		// Append to in-memory history.
		aiAssistant := ai.Message{
			Role:      ai.RoleAssistant,
			Content:   assistantContent,
			ToolCalls: append([]ai.ToolCall{}, toolCalls...),
		}
		messages = append(messages, aiAssistant)

		// 5d. Check finish reason.
		switch finishReason {
		case "stop":
			ch <- NewRunEvent(EventTurnComplete, TurnCompleteData{
				TurnIndex:  turnIndex,
				TokenUsage: tokenUsageWithCost(usage, turnCost),
			})
			ch <- NewRunEvent(EventRunComplete, RunCompleteData{
				TotalTurns:  turnIndex + 1,
				TotalTokens: totalTokens,
				TotalCost:   totalCost,
			})

			// Maybe store memories.
			if r.memory != nil {
				turnMsgs := r.collectTurnMessages(assistantContent, toolCalls)
				_, _ = r.memory.MaybeStore(ctx, in.AgentID, turnIndex, turnMsgs)
			}
			return

		case "tool_calls":
			// 5e. Execute tool calls.
			toolResults := r.executeToolCalls(ctx, ch, toolCalls, in)

			// Persist and append tool results to history.
			for i, result := range toolResults {
				tcID := toolCalls[i].ID
				toolName := toolCalls[i].Function.Name

				resultContent := formatToolResult(result)
				toolMsg := chat.ChatMessage{
					SessionID:   in.SessionID,
					Role:        "tool",
					Content:     resultContent,
					MessageType: chat.MessageTypeToolResult,
					ToolCallID:  &tcID,
					TurnIndex:   turnIndex,
				}
				if _, err := r.persister.CreateMessage(ctx, toolMsg); err != nil {
					emitError(ch, "persist_tool_result", err)
					return
				}

				messages = append(messages, ai.Message{
					Role:       ai.RoleTool,
					Content:    resultContent,
					ToolCallID: tcID,
				})

				// Emit tool result event.
				ch <- NewRunEvent(EventToolResult, ToolResultData{
					ID:         tcID,
					Name:       toolName,
					Output:     result.Output,
					DurationMs: result.LatencyMs,
					Error:      result.Error,
				})
			}

			ch <- NewRunEvent(EventTurnComplete, TurnCompleteData{
				TurnIndex:  turnIndex,
				TokenUsage: tokenUsageWithCost(usage, turnCost),
			})

			// Check context compaction (heuristic based on message count × avg tokens).
			if r.ctxManager != nil {
				systemTokens := EstimateStringTokens(systemPrompt)
				chatMsgs := aiMessagesToChatMessages(messages)
				if ShouldCompact(chatMsgs, systemTokens, r.config) {
					ch <- NewRunEvent(EventContextCompacted, CompactData{
						OriginalMessages: len(messages),
						CompactedTo:      len(messages), // no-op until Summarizer is wired
					})
				}
			}

			turnIndex++
			continue

		case "length":
			emitError(ch, "max_tokens", fmt.Errorf("LLM response truncated (max_tokens reached)"))
			return

		default:
			// Unknown finish reason, treat as stop.
			ch <- NewRunEvent(EventTurnComplete, TurnCompleteData{
				TurnIndex:  turnIndex,
				TokenUsage: tokenUsageWithCost(usage, turnCost),
			})
			ch <- NewRunEvent(EventRunComplete, RunCompleteData{
				TotalTurns:  turnIndex + 1,
				TotalTokens: totalTokens,
				TotalCost:   totalCost,
			})
			return
		}
	}

	// Safety brake: max iterations reached.
	emitError(ch, "max_iterations", fmt.Errorf("agentic loop exceeded maximum iterations (%d)", r.config.MaxIterations))
}

// consumeStream reads all chunks from the stream channel and accumulates the response.
func (r *Runner) consumeStream(ctx context.Context, ch chan<- RunEvent, stream <-chan ai.StreamChunk) (
	content string, toolCalls []ai.ToolCall, finishReason string, usage ai.Usage, err error,
) {
	// Track tool calls being built incrementally.
	toolCallMap := map[int]*ai.ToolCall{}
	toolCallIndex := 0

	for chunk := range stream {
		if ctx.Err() != nil {
			return "", nil, "", ai.Usage{}, ctx.Err()
		}

		if chunk.Error != nil {
			return "", nil, "", ai.Usage{}, chunk.Error
		}

		// Accumulate usage from stream (providers may send partial usage across chunks).
		if chunk.Usage != nil {
			if chunk.Usage.PromptTokens > 0 {
				usage.PromptTokens = chunk.Usage.PromptTokens
			}
			if chunk.Usage.CompletionTokens > 0 {
				usage.CompletionTokens = chunk.Usage.CompletionTokens
			}
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		}

		if chunk.Delta != "" {
			content += chunk.Delta
			ch <- NewRunEvent(EventTextDelta, TextDeltaData{Content: chunk.Delta})
		}

		if chunk.ToolCallDelta != nil {
			tc := chunk.ToolCallDelta
			if tc.ID != "" {
				// New tool call starting.
				toolCallMap[toolCallIndex] = &ai.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: ai.ToolFunction{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
				toolCallIndex++
			} else if toolCallIndex > 0 {
				// Appending to current tool call's arguments.
				existing := toolCallMap[toolCallIndex-1]
				existing.Function.Arguments += tc.Function.Arguments
			}
		}

		if chunk.FinishReason != "" {
			finishReason = chunk.FinishReason
		}
	}

	// Collect tool calls from map.
	for i := 0; i < toolCallIndex; i++ {
		if tc, ok := toolCallMap[i]; ok {
			toolCalls = append(toolCalls, *tc)
		}
	}

	// Map "tool_calls" finish reason if we got tool calls.
	if len(toolCalls) > 0 && finishReason == "" {
		finishReason = "tool_calls"
	}
	if finishReason == "" {
		finishReason = "stop"
	}

	return content, toolCalls, finishReason, usage, nil
}

// executeToolCalls runs the tool calls either in parallel (read-only) or serially.
func (r *Runner) executeToolCalls(ctx context.Context, ch chan<- RunEvent, toolCalls []ai.ToolCall, in RunInput) []ToolExecResult {
	results := make([]ToolExecResult, len(toolCalls))

	// Emit tool_call_start events.
	for _, tc := range toolCalls {
		ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		})
	}

	// Build ToolCall structs for parallel execution.
	calls := make([]ToolCall, len(toolCalls))
	for i, tc := range toolCalls {
		calls[i] = ToolCall{
			ID:        tc.ID,
			Slug:      tc.Function.Name,
			Input:     json.RawMessage(tc.Function.Arguments),
			TenantID:  in.TenantID,
			AgentID:   in.AgentID.String(),
			SessionID: in.SessionID.String(),
		}
	}

	// Execute all in parallel using the skill client's semaphore.
	toolCtx := ctx
	if r.config.ToolTimeout > 0 {
		var cancel context.CancelFunc
		toolCtx, cancel = context.WithTimeout(ctx, r.config.ToolTimeout)
		defer cancel()
	}

	execResults := r.skillClient.ExecuteParallel(toolCtx, calls, r.config.ConcurrentReadTools)
	for i, res := range execResults {
		results[i] = truncateToolResult(res, r.config.MaxToolResultChars)
	}

	return results
}

// loadHistory loads messages from the database and converts to ai.Message format.
func (r *Runner) loadHistory(ctx context.Context, sessionID uuid.UUID) ([]ai.Message, error) {
	if r.history == nil {
		return nil, nil
	}

	chatMsgs, err := r.history.FindAllMessages(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("runner: load history: %w", err)
	}

	var messages []ai.Message
	for _, m := range chatMsgs {
		// Skip system and compact_summary messages — handled by PromptBuilder.
		if m.MessageType == chat.MessageTypeSystem || m.MessageType == chat.MessageTypeCompactSummary {
			continue
		}

		aiMsg := ai.Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: derefString(m.ToolCallID),
		}

		// Parse tool_calls from assistant messages.
		if len(m.ToolCalls) > 0 {
			var tcs []ai.ToolCall
			if err := json.Unmarshal(m.ToolCalls, &tcs); err == nil {
				aiMsg.ToolCalls = tcs
			}
		}

		messages = append(messages, aiMsg)
	}

	return messages, nil
}

// buildAssistantMessage creates a ChatMessage for persistence.
func (r *Runner) buildAssistantMessage(
	sessionID uuid.UUID,
	content string,
	toolCalls []ai.ToolCall,
	finishReason string,
	usage ai.Usage,
	turnIndex int,
) chat.ChatMessage {
	msg := chat.ChatMessage{
		SessionID:    sessionID,
		Role:         "assistant",
		Content:      content,
		MessageType:  chat.MessageTypeText,
		FinishReason: &finishReason,
		TurnIndex:    turnIndex,
	}

	if len(toolCalls) > 0 {
		msg.MessageType = chat.MessageTypeToolUse
		if raw, err := json.Marshal(toolCalls); err == nil {
			msg.ToolCalls = raw
		}
	}

	if usage.TotalTokens > 0 {
		if raw, err := json.Marshal(usage); err == nil {
			msg.TokenUsage = raw
		}
	}

	return msg
}

// collectTurnMessages creates TurnMessage entries for the MemoryBridge.
func (r *Runner) collectTurnMessages(content string, toolCalls []ai.ToolCall) []TurnMessage {
	var msgs []TurnMessage
	if content != "" {
		msgs = append(msgs, TurnMessage{Role: "assistant", Content: content})
	}
	for _, tc := range toolCalls {
		msgs = append(msgs, TurnMessage{
			Role:    "assistant",
			Content: fmt.Sprintf("[tool_call: %s(%s)]", tc.Function.Name, tc.Function.Arguments),
		})
	}
	return msgs
}

// convertLLMToolsToAI converts the internal LLMTool format to the ai.Tool format.
func convertLLMToolsToAI(tools []LLMTool) []ai.Tool {
	result := make([]ai.Tool, len(tools))
	for i, t := range tools {
		var params map[string]any
		if len(t.InputSchema) > 0 {
			_ = json.Unmarshal(t.InputSchema, &params)
		}
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		result[i] = ai.Tool{
			Type: "function",
			Function: ai.ToolSchema{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  params,
			},
		}
	}
	return result
}

// formatToolResult produces a string representation of a tool execution result.
func formatToolResult(r ToolExecResult) string {
	if r.Error != nil {
		return fmt.Sprintf("Error: %s", *r.Error)
	}
	if len(r.Output) > 0 {
		return string(r.Output)
	}
	return "{}"
}

// tokenUsageWithCost creates a TokenUsage with cost information.
func tokenUsageWithCost(usage ai.Usage, cost float64) TokenUsage {
	return TokenUsage{
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		TotalTokens:      usage.TotalTokens,
		CostUSD:          cost,
	}
}

// truncateToolResult truncates the output if it exceeds maxChars.
// Returns the result unmodified if maxChars is 0 or output is within limit.
func truncateToolResult(result ToolExecResult, maxChars int) ToolExecResult {
	if maxChars <= 0 || len(result.Output) <= maxChars {
		return result
	}
	originalLen := len(result.Output)
	note := fmt.Sprintf("\n[truncated from %d chars]", originalLen)
	result.Output = append(result.Output[:maxChars-len(note)], []byte(note)...)
	return result
}

func emitError(ch chan<- RunEvent, code string, err error) {
	ch <- NewRunEvent(EventError, ErrorData{
		Message: err.Error(),
		Code:    code,
	})
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// aiMessagesToChatMessages converts ai.Message slice to chat.ChatMessage slice
// for token estimation purposes. Only content and tool_calls are relevant.
func aiMessagesToChatMessages(msgs []ai.Message) []chat.ChatMessage {
	result := make([]chat.ChatMessage, len(msgs))
	for i, m := range msgs {
		result[i] = chat.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
		}
		if len(m.ToolCalls) > 0 {
			if raw, err := json.Marshal(m.ToolCalls); err == nil {
				result[i].ToolCalls = raw
			}
		}
	}
	return result
}
