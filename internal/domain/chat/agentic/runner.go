package agentic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

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

// ElicitationSubmitter allows the runner to block on structured user input.
// The implementation (ElicitationHandler) lives in the adapter layer; the runner
// receives it via RunInput so there is no circular dependency.
type ElicitationSubmitter interface {
	Submit(ctx context.Context, serverName, requestID string, params ElicitationParams) ElicitationResult
}

// RunInput carries everything needed to start an agentic run.
type RunInput struct {
	RunID           uuid.UUID
	SessionID       uuid.UUID
	AgentID         uuid.UUID
	UserMessage     string
	SystemPrompt    string
	TenantID        string
	PermissionRules *PermissionRules

	// CurrentDepth is the recursion depth of this run. Root agent = 0.
	CurrentDepth int
	// RemainingBudgetUSD is the budget left for this run and any sub-agents.
	// Zero means no budget limit (inherit from config.MaxBudgetUSD).
	RemainingBudgetUSD float64
	// ParentEventCh, when set, receives forwarded events from sub-agent runs.
	// This allows the parent SSE stream to include sub-agent activity.
	ParentEventCh chan<- RunEvent
	// SubtaskID is the identity of this sub-agent for mailbox messaging.
	// Empty for the root agent.
	SubtaskID string
	// ParentSessionID is the root session ID used as the mailbox key.
	// Sub-agents use this to share a mailbox with siblings.
	ParentSessionID uuid.UUID

	// Elicitation, when set, handles ask_user tool calls by blocking until the
	// user submits a response via POST /elicitation/{requestId}/respond.
	Elicitation ElicitationSubmitter
}

// Runner orchestrates the agentic loop: LLM → tool_calls → execution → tool_results → LLM.
type Runner struct {
	chatModel       ai.ChatModel
	skillClient     *SkillRuntimeClient
	prompt          *PromptBuilder
	tools           *ToolSchemaBuilder
	mcpClient       MCPClientService
	ctxManager      *ContextManager
	memory          *MemoryBridge
	persister       MessagePersister
	history         HistoryLoader
	toolExec        *StreamingToolExecutor
	subtaskExec     *SubtaskExecutor
	agentMailbox    *AgentMailbox
	managementExec  *ManagementExecutor
	denialTracker   *DenialTracker
	turnEndHandlers []TurnEndHandler
	runEndHandlers  []RunEndHandler
	toolSummary     *ToolUseSummaryGenerator
	memoryExtractor *SessionMemoryExtractor
	cacheSafeSnap   *CacheSafeParamsSnapshot
	progress        *RunProgressTracker
	config          RunConfig
	runID           uuid.UUID
}

// NewRunner creates a Runner with the given dependencies.
// ctxManager, memory, hookExecutor, and subtaskExec may be nil (features are skipped).
func NewRunner(
	chatModel ai.ChatModel,
	skillClient *SkillRuntimeClient,
	prompt *PromptBuilder,
	tools *ToolSchemaBuilder,
	ctxManager *ContextManager,
	memory *MemoryBridge,
	persister MessagePersister,
	history HistoryLoader,
	hookExecutor *HookExecutor,
	config RunConfig,
) *Runner {
	var dt *DenialTracker
	if config.DenialEscalationThreshold > 0 {
		dt = NewDenialTracker(config.DenialEscalationThreshold)
	}
	return &Runner{
		chatModel:     chatModel,
		skillClient:   skillClient,
		prompt:        prompt,
		tools:         tools,
		ctxManager:    ctxManager,
		memory:        memory,
		persister:     persister,
		history:       history,
		toolExec:      NewStreamingToolExecutor(skillClient, hookExecutor, config),
		progress:      NewRunProgressTracker(10),
		config:        config,
		denialTracker: dt,
	}
}

// WithManagementExecutor attaches a ManagementExecutor to the Runner.
func (r *Runner) WithManagementExecutor(exec *ManagementExecutor) *Runner {
	r.managementExec = exec
	return r
}

// WithMCPClient attaches an MCP client so each run can build a tenant-scoped
// MCP bridge for tool schema loading and tool execution routing.
func (r *Runner) WithMCPClient(client MCPClientService) *Runner {
	r.mcpClient = client
	return r
}

// Progress returns the Runner's progress tracker for external monitoring.
func (r *Runner) Progress() *RunProgressTracker {
	return r.progress
}

// WithSubtaskExecutor attaches a SubtaskExecutor to the Runner.
func (r *Runner) WithSubtaskExecutor(exec *SubtaskExecutor) *Runner {
	r.subtaskExec = exec
	return r
}

// WithMailbox attaches a Mailbox to the Runner for inter-agent messaging.
func (r *Runner) WithAgentMailbox(m *AgentMailbox) *Runner {
	r.agentMailbox = m
	return r
}

// WithTurnEndHandlers registers handlers executed at the end of each turn.
func (r *Runner) WithTurnEndHandlers(handlers ...TurnEndHandler) *Runner {
	r.turnEndHandlers = append(r.turnEndHandlers, handlers...)
	return r
}

// WithRunEndHandlers registers handlers executed at the end of the run.
func (r *Runner) WithRunEndHandlers(handlers ...RunEndHandler) *Runner {
	r.runEndHandlers = append(r.runEndHandlers, handlers...)
	return r
}

// WithToolUseSummaryGenerator attaches a generator for cosmetic tool batch summaries.
func (r *Runner) WithToolUseSummaryGenerator(gen *ToolUseSummaryGenerator) *Runner {
	r.toolSummary = gen
	return r
}

// WithSessionMemoryExtractor attaches the background session memory extractor.
func (r *Runner) WithSessionMemoryExtractor(extractor *SessionMemoryExtractor) *Runner {
	r.memoryExtractor = extractor
	return r
}

// WithCacheSafeParamsSnapshot attaches the snapshot used by post-turn background forks.
func (r *Runner) WithCacheSafeParamsSnapshot(snap *CacheSafeParamsSnapshot) *Runner {
	r.cacheSafeSnap = snap
	return r
}

// Run starts the agentic loop in a goroutine and returns a channel of events.
// The channel is closed when the run completes or an error occurs.
func (r *Runner) Run(ctx context.Context, in RunInput) <-chan RunEvent {
	r.runID = in.RunID
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
	// Track whether a clean EventRunComplete was emitted. If the loop exits
	// via an error path (emitError + return) without emitting run_complete,
	// the deferred guard emits a minimal one so the client always knows the
	// run finished — inspired by Claude Code's guarantee that every run
	// ends with a terminal event.
	runCompleted := false
	var totalTokens, totalOutputTokens, latestInputTokens int
	var cumulativeCacheReadTokens, cumulativeCacheCreationTokens int
	var totalCost float64
	var turnIndex int

	defer func() {
		if !runCompleted {
			ch <- NewRunEvent(EventRunComplete, RunCompleteData{
				TotalTurns:                    turnIndex,
				TotalTokens:                   totalTokens,
				TotalCost:                     totalCost,
				LatestInputTokens:             latestInputTokens,
				CumulativeOutputTokens:        totalOutputTokens,
				CumulativeCacheReadTokens:     cumulativeCacheReadTokens,
				CumulativeCacheCreationTokens: cumulativeCacheCreationTokens,
			})
		}
	}()

	// Snapshot immutable gates once at run start. These pre-computed flags
	// prevent re-evaluating conditions on every loop iteration.
	gates := BuildRunGates(r.config, in.CurrentDepth, r.subtaskExec != nil)

	// 1. Recall memories (non-fatal on failure).
	memories := ""
	if r.memory != nil {
		var err error
		memories, err = r.memory.Recall(ctx, in.AgentID, in.UserMessage)
		if err != nil {
			emitError(ch, "memory_recall", err)
		}
	}

	// 2. Build tool schemas with deferred loading (depth limits for sub-agent availability).
	// When the total tool count exceeds DeferredToolThreshold, tools marked ShouldDefer
	// are separated — only their names go into the system prompt, and the LLM must call
	// tool_search to load their full schemas on demand.
	// Inspired by Claude Code's isDeferredTool + ToolSearchTool pattern.
	toolBuilder := r.tools.Clone()
	if toolBuilder == nil {
		emitError(ch, "tool_schema_build", fmt.Errorf("tool builder not configured"))
		return
	}

	if r.mcpClient != nil {
		bridge := NewMCPToolBridge(r.mcpClient, in.TenantID)
		toolBuilder.WithMCPBridge(bridge)
		if r.toolExec != nil {
			r.toolExec.WithMCPBridge(bridge)
		}
	} else if r.toolExec != nil {
		r.toolExec.WithMCPBridge(nil)
	}

	toolBuilder.WithDepthLimits(in.CurrentDepth, r.config.MaxDepth)
	toolResult, err := toolBuilder.BuildWithDeferred(ctx, in.AgentID)
	if err != nil {
		emitError(ch, "tool_schema_build", err)
		return
	}
	for _, w := range toolResult.Warnings {
		emitWarning(ch, "mcp_load_failed", w)
	}
	aiTools := convertLLMToolsToAI(toolResult.Loaded)
	toolNames := make([]string, len(toolResult.Loaded))
	for i, t := range toolResult.Loaded {
		toolNames[i] = t.Name
	}
	slog.Info("agentic: tools loaded for LLM", "count", len(aiTools), "tools", toolNames, "agentID", in.AgentID)
	readOnlyIndex := BuildReadOnlyIndex(toolResult.All)
	destructiveIndex := BuildDestructiveIndex(toolResult.All)
	contextModeIndex := BuildContextModeIndex(toolResult.All)
	interruptBehaviorIndex := BuildInterruptBehaviorIndex(toolResult.All)
	searchOrReadIndex := BuildSearchOrReadIndex(toolResult.All)
	deferredTools := toolResult.Deferred

	// Build allowed-tool index: only tools in toolResult.All (loaded + deferred) may be
	// executed in this run. This prevents the LLM from calling tools it "remembers" from
	// prior turns that are no longer bound to the agent (P-SK6 security fix).
	allowedToolsIndex := make(map[string]bool, len(toolResult.All))
	for _, t := range toolResult.All {
		allowedToolsIndex[t.Name] = true
	}
	_ = contextModeIndex       // TODO: use for fork-mode skill execution via SubtaskExecutor
	_ = interruptBehaviorIndex // TODO: pass to SSE handler for graceful stop
	_ = searchOrReadIndex      // TODO: pass to SSE handler for result auto-collapse

	// Merge per-tool result limits from DB into the config map.
	dbToolLimits := BuildMaxResultIndex(toolResult.All)
	if len(dbToolLimits) > 0 {
		if r.config.ToolResultLimits == nil {
			r.config.ToolResultLimits = dbToolLimits
		} else {
			for name, limit := range dbToolLimits {
				if _, exists := r.config.ToolResultLimits[name]; !exists {
					r.config.ToolResultLimits[name] = limit
				}
			}
		}
	}

	// 3. Build system prompt (after tools, so deferred tool names can be injected).
	systemPrompt, err := r.prompt.Build(ctx, PromptInput{
		AgentID:           in.AgentID,
		SessionID:         in.SessionID,
		SystemPrompt:      in.SystemPrompt,
		Memories:          memories,
		CoordinatorMode:   gates.CoordinatorMode,
		DeferredToolNames: toolResult.DeferredToolNames(),
		UserOnlySkills:    toolResult.UserOnlySkills,
		UnavailableTools:  toolResult.Warnings,
	})
	if err != nil {
		emitError(ch, "prompt_build", err)
		return
	}

	// 4. Load conversation history.
	messages, lastResponseID, err := r.loadHistory(ctx, in.SessionID)
	if err != nil {
		emitError(ch, "load_history", err)
		return
	}

	// Track the index of the last message sent to the LLM, used to slice
	// messages when response chaining is active (previous_response_id).
	lastSentIndex := 0

	// 5. Append user message.
	messages = append(messages, ai.Message{
		Role:    ai.RoleUser,
		Content: in.UserMessage,
	})

	// Persist user message.
	userMsg := chat.ChatMessage{
		SessionID:   in.SessionID,
		Role:        "user",
		Content:     in.UserMessage,
		MessageType: chat.MessageTypeText,
		RunID:       &r.runID,
	}
	if r.runID == uuid.Nil {
		userMsg.RunID = nil
	}
	if _, err := r.persister.CreateMessage(ctx, userMsg); err != nil {
		emitError(ch, "persist_user_msg", err)
		return
	}

	// 5. Agentic loop.
	// Token accounting follows Claude Code's cumulative vs incremental pattern:
	// - latestInputTokens: REPLACED each turn (most recent prompt tokens only)
	// - cumulativeOutputTokens: ACCUMULATED across all turns (sum of completions)
	// - cumulativeCacheRead/Creation: ACCUMULATED across all turns
	compactFailures := 0
	maxTokensRecoveryCount := 0
	const maxCompactFailures = 3
	const maxMaxTokensRecoveries = 3
	effectiveMaxTokens := r.config.MaxTokensPerCall // may increase on "length" recovery
	budgetTracker := &BudgetTracker{}
	// taskBudgetRemaining tracks how much of the output token budget has been
	// "consumed" by compacted-away context. After compaction the LLM can no
	// longer count tokens from the removed history, so we decrement remaining
	// and pass it in subsequent requests. Inspired by Claude Code's
	// taskBudgetRemaining tracking across compaction boundaries.
	taskBudgetRemaining := 0 // 0 means "not tracking" (no compaction has occurred yet)

	// Effective budget: prefer explicit remaining budget (from parent), fall back to config.
	effectiveBudget := r.config.MaxBudgetUSD
	if in.RemainingBudgetUSD > 0 {
		effectiveBudget = in.RemainingBudgetUSD
		gates.HasBudgetLimit = true // override: parent passed an explicit budget
	}

	for turnIndex < r.config.MaxIterations {
		slog.Info("agentic: loop iteration start", "turn", turnIndex, "maxIterations", r.config.MaxIterations, "ctxErr", ctx.Err())
		if err := ctx.Err(); err != nil {
			slog.Error("agentic: context cancelled at loop start", "turn", turnIndex, "error", err)
			emitError(ch, "context_cancelled", err)
			return
		}

		// Drain mailbox messages for sub-agents before each LLM call.
		if r.agentMailbox != nil && in.SubtaskID != "" && in.ParentSessionID != uuid.Nil {
			mailboxContent := DrainAgentMailbox(r.agentMailbox, in.ParentSessionID, in.SubtaskID)
			if mailboxContent != "" {
				messages = append(messages, ai.Message{
					Role:    ai.RoleUser,
					Content: mailboxContent,
				})
			}
		}

		// Inject escalation hints from denial tracker.
		if r.denialTracker != nil {
			if hints := r.denialTracker.EscalationHints(); len(hints) > 0 {
				hintMsg := "[SYSTEM] The following tools have been repeatedly denied by permission rules:\n"
				for _, h := range hints {
					hintMsg += "- " + h + "\n"
				}
				messages = append(messages, ai.Message{
					Role:    ai.RoleUser,
					Content: hintMsg,
				})
			}
		}

		// Per-turn token budget (may be escalated).
		turnBudget := r.config.EffectiveTurnBudget(turnIndex)
		maxTokensForCall := r.config.MaxTokensPerCall
		if turnBudget > 0 && (maxTokensForCall == 0 || turnBudget < maxTokensForCall) {
			maxTokensForCall = turnBudget
		}

		// Apply per-turn budget on top of the recovery-adjusted max tokens.
		// Recovery (finish_reason="length") increases effectiveMaxTokens above MaxTokensPerCall;
		// in that case we honour the recovery value and don't cap it back down.
		callMaxTokens := effectiveMaxTokens
		if maxTokensForCall > 0 && (callMaxTokens == 0 || maxTokensForCall < callMaxTokens) &&
			effectiveMaxTokens <= r.config.MaxTokensPerCall {
			callMaxTokens = maxTokensForCall
		}

		opts := ai.ChatOptions{
			Model:        r.config.Model,
			MaxTokens:    callMaxTokens,
			Temperature:  r.config.Temperature,
			Tools:        aiTools,
			Stream:       true,
			SystemMsg:    systemPrompt,
			Thinking:     resolveThinkingConfig(r.config),
			CacheControl: gates.CacheControl,
			Effort:       gates.ResolvedEffort,
		}
		cacheSafeParams := NewCacheSafeParams(
			systemPrompt,
			aiTools,
			r.chatModel.GetProviderName(),
			opts.Model,
			gates.CacheControl,
		)

		// Determine query source for this turn.
		turnSource := SourceMainLoop
		if in.CurrentDepth > 0 {
			turnSource = SourceSubtask
		}

		// Response chaining: set previous_response_id and slice messages to
		// only send new items when the provider supports server-side history.
		opts.PreviousResponseID = lastResponseID
		messagesToSend := messages
		if lastResponseID != "" && lastSentIndex > 0 && lastSentIndex < len(messages) {
			messagesToSend = messages[lastSentIndex:]
		}

		// 5a. Call LLM with streaming (with retry + model fallback).
		fallbackResult, err := retryStreamWithFallbackSource(ctx, r.chatModel, messagesToSend, opts, r.config, turnSource,
			func(from, to string, fallbackErr error) {
				ch <- NewRunEvent(EventModelFallback, ModelFallbackData{
					FromModel: from,
					ToModel:   to,
					Reason:    fallbackErr.Error(),
				})
			},
		)
		if err != nil {
			// Recovery: context overflow (input + max_tokens > limit).
			// Reduce max_tokens and retry without compaction.
			// Inspired by Claude Code's withRetry.ts adjustedMaxTokens logic.
			if isContextOverflow(err) && maxTokensRecoveryCount < maxMaxTokensRecoveries {
				adjusted := computeAdjustedMaxTokens(err.Error())
				if adjusted > 0 {
					maxTokensRecoveryCount++
					slog.Warn("context overflow: reducing max_tokens to fit",
						"adjusted", adjusted,
						"previous", effectiveMaxTokens,
						"attempt", maxTokensRecoveryCount,
					)
					effectiveMaxTokens = adjusted
					continue // retry the turn with reduced max_tokens
				}
			}

			// Recovery: if prompt is too long, compact context and retry.
			if isPromptTooLong(err) && r.ctxManager != nil && compactFailures < maxCompactFailures {
				slog.Warn("prompt too long, attempting reactive compaction",
					"turn", turnIndex,
					"compactFailures", compactFailures,
				)
				chatMsgs := aiMessagesToChatMessages(messages)
				systemTokens := EstimateStringTokens(systemPrompt)
				compactResult, compactErr := r.ctxManager.ReactiveCompact(ctx, chatMsgs, systemTokens, r.config, nil)
				if compactErr != nil {
					compactFailures++
					slog.Error("reactive compaction failed during prompt_too_long recovery",
						"error", compactErr, "failures", compactFailures)
					if compactFailures >= maxCompactFailures {
						emitError(ch, "compact_circuit_breaker", fmt.Errorf("compaction failed %d times consecutively", compactFailures))
						return
					}
					emitError(ch, "llm_call", err)
					return
				}
				// Verify compaction actually reduced tokens; escalate if needed.
				compactResult, compactErr = r.ctxManager.VerifyCompaction(ctx, compactResult, systemTokens, r.config, nil)
				if compactErr != nil {
					compactFailures++
					emitError(ch, "llm_call", err)
					return
				}
				messages = chatMessagesToAI(compactResult.Messages)
				// Compaction invalidates the server-side response chain.
				lastResponseID = ""
				lastSentIndex = 0
				ch <- NewRunEvent(EventContextCompacted, CompactData{
					OriginalMessages: compactResult.OriginalCount,
					CompactedTo:      compactResult.CompactedCount,
				})
				compactFailures = 0
				continue // retry the turn with compacted context
			}

			// Recovery: invalid previous_response_id — clear chain and retry with full history.
			if isInvalidResponseID(err) && lastResponseID != "" {
				slog.Warn("previous_response_id rejected, falling back to full history",
					"responseID", lastResponseID)
				lastResponseID = ""
				lastSentIndex = 0
				continue // retry the turn with full history
			}

			emitError(ch, "llm_call", err)
			return
		}

		// Track which model was actually used for cost estimation.
		effectiveModel := fallbackResult.ModelUsed
		cacheSafeParams = NewCacheSafeParams(
			systemPrompt,
			aiTools,
			r.chatModel.GetProviderName(),
			effectiveModel,
			gates.CacheControl,
		)

		// 5b. Consume stream, accumulate response.
		assistantContent, toolCalls, finishReason, usage, streamResponseID, streamErr := r.consumeStream(ctx, ch, fallbackResult.Stream)
		if streamErr != nil {
			emitError(ch, "stream_consume", streamErr)
			return
		}

		// Update response chaining state: record the index before appending
		// the assistant message so the next iteration can slice correctly.
		lastSentIndex = len(messages)
		if streamResponseID != "" {
			lastResponseID = streamResponseID
		}

		// Clear response chain when the model was swapped (different provider
		// won't recognise the previous response_id).
		if fallbackResult.WasFallback {
			lastResponseID = ""
			lastSentIndex = 0
		}

		// Guard against empty LLM response (no text, no tool calls).
		// Some providers return empty content on edge cases; treat as no-op stop.
		if assistantContent == "" && len(toolCalls) == 0 && finishReason == "stop" {
			assistantContent = "(no content)"
		}

		totalTokens += usage.TotalTokens
		totalOutputTokens += usage.CompletionTokens
		// Cumulative vs incremental: input tokens are REPLACED (latest snapshot),
		// output/cache tokens are ACCUMULATED (running sum).
		latestInputTokens = usage.PromptTokens
		cumulativeCacheReadTokens += usage.CacheReadTokens
		cumulativeCacheCreationTokens += usage.CacheCreationTokens

		// Check per-turn budget after consuming the stream.
		if turnBudget > 0 && usage.TotalTokens > turnBudget {
			emitError(ch, "turn_budget_exceeded",
				fmt.Errorf("turn %d used %d tokens, exceeding budget of %d", turnIndex, usage.TotalTokens, turnBudget))
		}

		// Accumulate cost using the effective model (may be a fallback).
		turnCost := EstimateCostUSD(effectiveModel, usage)
		totalCost += turnCost

		// Update progress tracker.
		r.progress.SetTurnIndex(turnIndex)
		r.progress.RecordLLMCall(usage.TotalTokens, turnCost, r.config.Model)

		// Budget check.
		if gates.HasBudgetLimit && totalCost > effectiveBudget {
			emitError(ch, "budget_exceeded", fmt.Errorf("run cost $%.4f exceeded budget $%.4f", totalCost, effectiveBudget))
			return
		}

		// 5c. Build and persist assistant message.
		assistantMsg := r.buildAssistantMessage(in.SessionID, assistantContent, toolCalls, finishReason, usage, turnIndex, streamResponseID)
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

		// Helper to build turn-end payload for hooks.
		buildTurnEndPayload := func() TurnEndPayload {
			var tcInfos []ToolCallInfo
			for _, tc := range toolCalls {
				tcInfos = append(tcInfos, ToolCallInfo{ID: tc.ID, Name: tc.Function.Name})
			}
			usageJSON, _ := json.Marshal(tokenUsageWithCost(usage, turnCost, effectiveModel))
			return TurnEndPayload{
				Event:            HookTurnEnd,
				AgentID:          in.AgentID.String(),
				SessionID:        in.SessionID.String(),
				TurnIndex:        turnIndex,
				AssistantContent: assistantContent,
				ToolCalls:        tcInfos,
				TokenUsage:       usageJSON,
			}
		}

		// 5d. Check finish reason.
		switch finishReason {
		case "stop":
			// Token budget continuation: if a budget is set and the LLM stopped
			// before reaching it, inject a nudge message to keep working.
			if gates.HasOutputTokenBudget {
				decision := budgetTracker.CheckTokenBudget(r.config.OutputTokenBudget, totalOutputTokens)
				if decision.Action == "continue" {
					ch <- NewRunEvent(EventTurnComplete, TurnCompleteData{
						TurnIndex:  turnIndex,
						TokenUsage: tokenUsageWithCost(usage, turnCost, effectiveModel),
						Source:     SourceBudgetNudge,
					})
					// Inject nudge as user message to keep the LLM working.
					messages = append(messages, ai.Message{
						Role:    ai.RoleUser,
						Content: decision.NudgeMessage,
					})
					turnIndex++
					continue
				}
			}

			// Turn-end hooks (before emitting turn_complete).
			r.executeTurnEndHooks(ctx, ch, buildTurnEndPayload())
			stopHookResult := r.handlePostTurnLifecycle(ctx, ch, StopHookContext{
				Messages:         append([]ai.Message(nil), messages...),
				SystemPrompt:     systemPrompt,
				QuerySource:      turnSource,
				AgentID:          in.AgentID,
				SessionID:        in.SessionID,
				TenantID:         in.TenantID,
				CacheSafeParams:  cacheSafeParams,
				CurrentDepth:     in.CurrentDepth,
				TurnIndex:        turnIndex,
				CurrentTokens:    EstimateTokens(aiMessagesToChatMessages(messages)) + EstimateStringTokens(systemPrompt),
				TurnHadToolCalls: false,
			})
			if stopHookResult.PreventContinuation {
				emitError(ch, "stop_hook_veto", errors.New(stopHookResult.StopReason))
				return
			}
			for _, blockingErr := range stopHookResult.BlockingErrors {
				messages = append(messages, ai.Message{Role: ai.RoleUser, Content: blockingErr})
			}

			ch <- NewRunEvent(EventTurnComplete, TurnCompleteData{
				TurnIndex:   turnIndex,
				TokenUsage:  tokenUsageWithCost(usage, turnCost, effectiveModel),
				BudgetUsed:  usage.TotalTokens,
				BudgetLimit: turnBudget,
				Model:       effectiveModel,
				Source:      turnSource,
			})
			ch <- NewRunEvent(EventRunProgress, r.progress.Snapshot())
			runCompleted = true
			ch <- NewRunEvent(EventRunComplete, RunCompleteData{
				TotalTurns:                    turnIndex + 1,
				TotalTokens:                   totalTokens,
				TotalCost:                     totalCost,
				LatestInputTokens:             latestInputTokens,
				CumulativeOutputTokens:        totalOutputTokens,
				CumulativeCacheReadTokens:     cumulativeCacheReadTokens,
				CumulativeCacheCreationTokens: cumulativeCacheCreationTokens,
			})

			// Run-end hooks (after run_complete).
			r.executeRunEndHooks(ctx, ch, RunEndPayload{
				Event:       HookRunEnd,
				AgentID:     in.AgentID.String(),
				SessionID:   in.SessionID.String(),
				TotalTurns:  turnIndex + 1,
				TotalTokens: totalTokens,
				TotalCost:   totalCost,
			})
			return

		case "tool_calls":
			// 5e. Apply permission rules and execute tool calls.
			tcNames := make([]string, len(toolCalls))
			for ti, tc := range toolCalls {
				tcNames[ti] = tc.Function.Name
			}
			slog.Info("agentic: LLM requested tool_calls", "turn", turnIndex, "tools", tcNames)
			toolResults := r.executeWithPermissions(ctx, ch, toolCalls, in, totalCost, readOnlyIndex, destructiveIndex, deferredTools, allowedToolsIndex)
			slog.Info("agentic: tool execution completed", "turn", turnIndex, "resultCount", len(toolResults), "ctxErr", ctx.Err())

			// Check if denial tracking indicates a stuck loop.
			if r.denialTracker != nil && len(r.denialTracker.EscalationHints()) > 0 {
				totalDenials := r.denialTracker.TotalDenials()
				emitError(ch, "denial_escalation", fmt.Errorf(
					"too many tool denials (%d total) — LLM appears stuck in a permission loop",
					totalDenials))
				return
			}

			// Persist and append tool results to history.
			turnResultChars := 0
			for i, result := range toolResults {
				tcID := toolCalls[i].ID
				toolName := toolCalls[i].Function.Name

				toolLimit := resolveToolResultLimit(toolName, r.config.ToolResultLimits, r.config.MaxToolResultChars)
				result = truncateToolResult(result, toolLimit)

				// Enforce per-turn aggregate budget.
				if gates.HasAggregateResultLimit && turnResultChars+len(result.Output) > r.config.MaxToolResultsPerTurnChars {
					budgetMsg := "[tool result omitted: per-turn budget exceeded]"
					result.Output = json.RawMessage(budgetMsg)
				}
				turnResultChars += len(result.Output)

				resultContent := FormatToolResult(result)
				toolMsg := chat.ChatMessage{
					SessionID:   in.SessionID,
					Role:        "tool",
					Content:     resultContent,
					MessageType: chat.MessageTypeToolResult,
					ToolCallID:  &tcID,
					TurnIndex:   turnIndex,
					RunID:       &r.runID,
				}
				if r.runID == uuid.Nil {
					toolMsg.RunID = nil
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

				// Emit tool result event (skip if executor already emitted to avoid duplicates).
				if !result.EmittedToStream {
					ch <- NewRunEvent(EventToolResult, ToolResultData{
						ID:         tcID,
						Name:       toolName,
						Output:     result.Output,
						DurationMs: result.LatencyMs,
						Error:      result.Error,
					})
				}

				// Track tool execution in progress.
				r.progress.RecordToolCall(toolName)
			}

			if summary := r.buildToolUseSummary(ctx, in.AgentID, toolCalls, toolResults, assistantContent); summary != "" {
				ch <- NewRunEvent(EventToolUseSummary, ToolUseSummaryData{
					TurnIndex: turnIndex,
					Summary:   summary,
				})
			}

			// Turn-end hooks (after tool results, before incrementing turnIndex).
			r.executeTurnEndHooks(ctx, ch, buildTurnEndPayload())
			stopHookResult := r.handlePostTurnLifecycle(ctx, ch, StopHookContext{
				Messages:         append([]ai.Message(nil), messages...),
				SystemPrompt:     systemPrompt,
				QuerySource:      turnSource,
				AgentID:          in.AgentID,
				SessionID:        in.SessionID,
				TenantID:         in.TenantID,
				CacheSafeParams:  cacheSafeParams,
				CurrentDepth:     in.CurrentDepth,
				TurnIndex:        turnIndex,
				CurrentTokens:    EstimateTokens(aiMessagesToChatMessages(messages)) + EstimateStringTokens(systemPrompt),
				TurnHadToolCalls: true,
			})
			if stopHookResult.PreventContinuation {
				emitError(ch, "stop_hook_veto", errors.New(stopHookResult.StopReason))
				return
			}
			for _, blockingErr := range stopHookResult.BlockingErrors {
				messages = append(messages, ai.Message{Role: ai.RoleUser, Content: blockingErr})
			}

			slog.Info("agentic: tool results persisted, emitting turn_complete", "turn", turnIndex, "ctxErr", ctx.Err())
			ch <- NewRunEvent(EventTurnComplete, TurnCompleteData{
				TurnIndex:   turnIndex,
				TokenUsage:  tokenUsageWithCost(usage, turnCost, effectiveModel),
				BudgetUsed:  usage.TotalTokens,
				BudgetLimit: turnBudget,
				Model:       effectiveModel,
				Source:      turnSource,
			})

			// Emit consolidated progress.
			ch <- NewRunEvent(EventRunProgress, r.progress.Snapshot())

			// Check context compaction using progressive stages.
			if r.ctxManager != nil && compactFailures < maxCompactFailures {
				systemTokens := EstimateStringTokens(systemPrompt)
				chatMsgs := aiMessagesToChatMessages(messages)

				// Capture pre-compact context size for task budget tracking.
				// After compaction the LLM can no longer see the compacted history,
				// so we must decrement the budget remaining accordingly.
				preCompactTokens := EstimateTokens(chatMsgs) + systemTokens

				result, err := r.ctxManager.ReactiveCompact(ctx, chatMsgs, systemTokens, r.config, nil)
				if err != nil {
					compactFailures++
					slog.Warn("reactive compaction failed",
						"error", err, "failures", compactFailures)
				} else if result.Stage != "" {
					// Verify compaction and escalate if needed.
					result, err = r.ctxManager.VerifyCompaction(ctx, result, systemTokens, r.config, nil)
					if err != nil {
						compactFailures++
						slog.Warn("post-compact verification failed",
							"error", err, "failures", compactFailures)
					}
					// Apply compacted messages back.
					messages = chatMessagesToAI(result.Messages)
					// Compaction invalidates the server-side response chain.
					lastResponseID = ""
					lastSentIndex = 0
					compactFailures = 0
					ch <- NewRunEvent(EventContextCompacted, CompactData{
						OriginalMessages: result.OriginalCount,
						CompactedTo:      result.CompactedCount,
					})

					// Update task budget remaining across compaction boundary.
					// The compacted-away history is no longer visible to the LLM,
					// so decrement remaining by what was consumed pre-compact.
					// Inspired by Claude Code's taskBudgetRemaining tracking.
					if gates.HasOutputTokenBudget {
						budget := r.config.OutputTokenBudget
						if taskBudgetRemaining == 0 {
							taskBudgetRemaining = budget
						}
						taskBudgetRemaining -= preCompactTokens
						if taskBudgetRemaining < 0 {
							taskBudgetRemaining = 0
						}
						slog.Debug("task budget updated after compaction",
							"preCompactTokens", preCompactTokens,
							"taskBudgetRemaining", taskBudgetRemaining,
							"totalBudget", budget,
						)
					}

					// Post-compact cleanup: reset transient state that may be stale
					// after context changes. Inspired by Claude Code's postCompactCleanup.
					maxTokensRecoveryCount = 0
					effectiveMaxTokens = r.config.MaxTokensPerCall

					// Clear prompt section cache so stable sections (tools, KBs) are
					// recomputed. After compaction the old cached values may reference
					// context that was summarized away.
					// Inspired by Claude Code's clearSystemPromptSections on /compact.
					r.prompt.ClearCache()

					// Rebuild system prompt to re-inject tool descriptions, memories,
					// deferred tool names, and KB context that were summarized away.
					// Inspired by CC's buildPostCompactMessages() which re-injects
					// deferred_tools_delta, invoked_skills, and agent_listing after compact.
					if result.Stage == StageFullSummarization {
						freshMemories := ""
						if r.memory != nil {
							if m, err := r.memory.Recall(ctx, in.AgentID, in.UserMessage); err == nil {
								freshMemories = m
							}
						}
						if rebuilt, err := r.prompt.Build(ctx, PromptInput{
							AgentID:           in.AgentID,
							SessionID:         in.SessionID,
							SystemPrompt:      in.SystemPrompt,
							Memories:          freshMemories,
							CoordinatorMode:   gates.CoordinatorMode,
							DeferredToolNames: toolResult.DeferredToolNames(),
							UserOnlySkills:    toolResult.UserOnlySkills,
							UnavailableTools:  toolResult.Warnings,
						}); err == nil {
							systemPrompt = rebuilt
						}
					}
				}
			}

			turnIndex++
			slog.Info("agentic: advancing to next turn after tool_calls", "nextTurn", turnIndex, "ctxErr", ctx.Err())
			continue

		case "length":
			// Recovery: retry with increased max_tokens up to 3 times.
			// The model hit the output token limit; increase it and continue.
			if maxTokensRecoveryCount < maxMaxTokensRecoveries {
				maxTokensRecoveryCount++
				newMax := effectiveMaxTokens * 2
				if newMax > 32768 {
					newMax = 32768
				}
				slog.Warn("max_tokens hit, retrying with increased limit",
					"attempt", maxTokensRecoveryCount,
					"oldMaxTokens", effectiveMaxTokens,
					"newMaxTokens", newMax,
				)
				effectiveMaxTokens = newMax
				// Don't increment turnIndex — retry the same turn.
				continue
			}
			emitError(ch, "max_tokens", fmt.Errorf("LLM response truncated after %d recovery attempts (max_tokens reached)", maxTokensRecoveryCount))
			return

		default:
			// Unknown finish reason, treat as stop.
			r.executeTurnEndHooks(ctx, ch, buildTurnEndPayload())
			stopHookResult := r.handlePostTurnLifecycle(ctx, ch, StopHookContext{
				Messages:         append([]ai.Message(nil), messages...),
				SystemPrompt:     systemPrompt,
				QuerySource:      turnSource,
				AgentID:          in.AgentID,
				SessionID:        in.SessionID,
				TenantID:         in.TenantID,
				CacheSafeParams:  cacheSafeParams,
				CurrentDepth:     in.CurrentDepth,
				TurnIndex:        turnIndex,
				CurrentTokens:    EstimateTokens(aiMessagesToChatMessages(messages)) + EstimateStringTokens(systemPrompt),
				TurnHadToolCalls: false,
			})
			if stopHookResult.PreventContinuation {
				emitError(ch, "stop_hook_veto", errors.New(stopHookResult.StopReason))
				return
			}
			for _, blockingErr := range stopHookResult.BlockingErrors {
				messages = append(messages, ai.Message{Role: ai.RoleUser, Content: blockingErr})
			}

			ch <- NewRunEvent(EventTurnComplete, TurnCompleteData{
				TurnIndex:   turnIndex,
				TokenUsage:  tokenUsageWithCost(usage, turnCost, effectiveModel),
				BudgetUsed:  usage.TotalTokens,
				BudgetLimit: turnBudget,
				Model:       effectiveModel,
				Source:      turnSource,
			})
			ch <- NewRunEvent(EventRunProgress, r.progress.Snapshot())
			runCompleted = true
			ch <- NewRunEvent(EventRunComplete, RunCompleteData{
				TotalTurns:                    turnIndex + 1,
				TotalTokens:                   totalTokens,
				TotalCost:                     totalCost,
				LatestInputTokens:             latestInputTokens,
				CumulativeOutputTokens:        totalOutputTokens,
				CumulativeCacheReadTokens:     cumulativeCacheReadTokens,
				CumulativeCacheCreationTokens: cumulativeCacheCreationTokens,
			})

			// Run-end hooks.
			r.executeRunEndHooks(ctx, ch, RunEndPayload{
				Event:       HookRunEnd,
				AgentID:     in.AgentID.String(),
				SessionID:   in.SessionID.String(),
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
	content string, toolCalls []ai.ToolCall, finishReason string, usage ai.Usage, responseID string, err error,
) {
	// Track tool calls being built incrementally. Some providers emit
	// repeated deltas for the same tool call ID while arguments stream in.
	toolCallsByID := map[string]*ai.ToolCall{}
	toolCallOrder := make([]string, 0)
	var lastToolCall *ai.ToolCall

	for chunk := range stream {
		if ctx.Err() != nil {
			return "", nil, "", ai.Usage{}, "", ctx.Err()
		}

		if chunk.Error != nil {
			return "", nil, "", ai.Usage{}, "", chunk.Error
		}

		// Accumulate usage from stream (providers may send partial usage across chunks).
		if chunk.Usage != nil {
			if chunk.Usage.PromptTokens > 0 {
				usage.PromptTokens = chunk.Usage.PromptTokens
			}
			if chunk.Usage.CompletionTokens > 0 {
				usage.CompletionTokens = chunk.Usage.CompletionTokens
			}
			if chunk.Usage.CacheReadTokens > 0 {
				usage.CacheReadTokens = chunk.Usage.CacheReadTokens
			}
			if chunk.Usage.CacheCreationTokens > 0 {
				usage.CacheCreationTokens = chunk.Usage.CacheCreationTokens
			}
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
		}

		if chunk.ThinkingDelta != "" {
			ch <- NewRunEvent(EventThinkingDelta, ThinkingDeltaData{Content: chunk.ThinkingDelta})
		}

		if chunk.Delta != "" {
			content += chunk.Delta
			ch <- NewRunEvent(EventTextDelta, TextDeltaData{Content: chunk.Delta})
			// Guard against models that generate runaway recursive JSON instead
			// of proper tool calls (observed with some local 20B models).
			// If the text buffer exceeds 50 KB and contains a deeply nested
			// JSON-in-JSON pattern, abort the stream to avoid OOM and 100 KB+
			// garbage being sent to the client.
			if len(content) > 50_000 && strings.Count(content, `"arguments":{`) > 5 {
				return content, nil, "stop", usage, responseID, fmt.Errorf("runaway recursive output detected (>50KB with nested JSON pattern) — model may be generating malformed tool calls as text")
			}
		}

		if chunk.ToolCallDelta != nil {
			tc := chunk.ToolCallDelta
			if tc.ID != "" {
				existing, ok := toolCallsByID[tc.ID]
				if !ok {
					existing = &ai.ToolCall{
						ID:   tc.ID,
						Type: tc.Type,
						Function: ai.ToolFunction{
							Name:      tc.Function.Name,
							Arguments: tc.Function.Arguments,
						},
					}
					toolCallsByID[tc.ID] = existing
					toolCallOrder = append(toolCallOrder, tc.ID)
				} else {
					if existing.Type == "" {
						existing.Type = tc.Type
					}
					if existing.Function.Name == "" {
						existing.Function.Name = tc.Function.Name
					}
					existing.Function.Arguments += tc.Function.Arguments
				}
				lastToolCall = existing
			} else if lastToolCall != nil {
				// Providers without stable IDs append args to the latest tool call.
				if lastToolCall.Function.Name == "" {
					lastToolCall.Function.Name = tc.Function.Name
				}
				lastToolCall.Function.Arguments += tc.Function.Arguments
			}
		}

		if chunk.ResponseID != "" {
			responseID = chunk.ResponseID
		}

		if chunk.FinishReason != "" {
			finishReason = chunk.FinishReason
		}
	}

	// Collect tool calls in first-seen order.
	for _, id := range toolCallOrder {
		if tc, ok := toolCallsByID[id]; ok {
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

	return content, toolCalls, finishReason, usage, responseID, nil
}

// loadHistory loads messages from the database and converts to ai.Message format.
// Applies time-based tool result eviction when the session has been idle longer
// than the cache TTL (inspired by Claude Code's microCompact.ts cold-cache trigger).
// Returns the messages and the response_id from the last assistant message's metadata
// (for response chaining with providers that support it).
func (r *Runner) loadHistory(ctx context.Context, sessionID uuid.UUID) ([]ai.Message, string, error) {
	if r.history == nil {
		return nil, "", nil
	}

	chatMsgs, err := r.history.FindAllMessages(ctx, sessionID)
	if err != nil {
		return nil, "", fmt.Errorf("runner: load history: %w", err)
	}

	// Time-based tool result eviction: if the session has been idle longer
	// than the cache TTL, clear old tool results before they waste tokens
	// on the now-cold cache miss. Fire before the request, not after.
	if len(chatMsgs) > 0 {
		lastMsg := chatMsgs[len(chatMsgs)-1]
		if !lastMsg.CreatedAt.IsZero() {
			idleTime := time.Since(lastMsg.CreatedAt)
			cfg := DefaultTimeBasedEvictionConfig()
			chatMsgs = EvictStaleToolResults(chatMsgs, idleTime, cfg)
		}
	}

	// Extract response_id from the last assistant message's metadata.
	// This enables response chaining when resuming a session.
	var lastResponseID string
	for i := len(chatMsgs) - 1; i >= 0; i-- {
		if chatMsgs[i].Role == "assistant" && len(chatMsgs[i].Metadata) > 0 {
			var meta map[string]string
			if json.Unmarshal(chatMsgs[i].Metadata, &meta) == nil {
				lastResponseID = meta["response_id"]
			}
			break
		}
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

	messages = filterUnresolvedToolUses(messages)
	messages = SanitizeMessages(messages)
	return messages, lastResponseID, nil
}

// filterUnresolvedToolUses removes assistant messages that contain tool_calls
// without matching tool_result messages. This can happen when a run crashes
// between emitting a tool_use and receiving its tool_result. The API rejects
// orphaned tool_use blocks. Inspired by Claude Code's filterUnresolvedToolUses.
func filterUnresolvedToolUses(messages []ai.Message) []ai.Message {
	// Collect all tool_result IDs.
	toolResultIDs := make(map[string]bool)
	for _, m := range messages {
		if m.Role == ai.RoleTool && m.ToolCallID != "" {
			toolResultIDs[m.ToolCallID] = true
		}
	}

	// Find unresolved tool_use IDs.
	unresolvedIDs := make(map[string]bool)
	for _, m := range messages {
		if m.Role == ai.RoleAssistant {
			for _, tc := range m.ToolCalls {
				if !toolResultIDs[tc.ID] {
					unresolvedIDs[tc.ID] = true
				}
			}
		}
	}

	if len(unresolvedIDs) == 0 {
		return messages
	}

	// Filter: remove assistant messages with ALL tool_calls unresolved,
	// and remove orphaned tool messages referencing unknown calls.
	var filtered []ai.Message
	for _, m := range messages {
		if m.Role == ai.RoleAssistant && len(m.ToolCalls) > 0 {
			allUnresolved := true
			for _, tc := range m.ToolCalls {
				if !unresolvedIDs[tc.ID] {
					allUnresolved = false
					break
				}
			}
			if allUnresolved {
				// Skip this orphaned assistant message entirely.
				continue
			}
		}
		filtered = append(filtered, m)
	}

	return filtered
}

// SanitizeMessages removes degenerate messages that could cause API errors or
// waste tokens. Inspired by Claude Code's filterWhitespaceOnlyAssistantMessages
// and filterOrphanedThinkingOnlyMessages.
//
// Filters applied:
// 1. Remove assistant messages with empty/whitespace-only content and no tool_calls.
// 2. Remove consecutive duplicate user messages (can occur after compaction).
func SanitizeMessages(messages []ai.Message) []ai.Message {
	if len(messages) == 0 {
		return messages
	}

	var result []ai.Message
	for _, m := range messages {
		// Filter 1: whitespace-only assistant messages without tool calls.
		if m.Role == ai.RoleAssistant && len(m.ToolCalls) == 0 && strings.TrimSpace(m.Content) == "" {
			continue
		}

		// Filter 2: consecutive user messages — collapse to the last one in each run.
		// This handles orphaned user messages from failed LLM calls: when the runner
		// persists the user message before calling the LLM and the LLM call fails,
		// no assistant response is saved. On the next run the history has consecutive
		// user messages which most LLM providers (e.g. Ollama) reject with HTTP 400.
		// We keep the most recent user message in each consecutive run so that when
		// the runner appends the new user message it lands after an assistant turn.
		if m.Role == ai.RoleUser && len(result) > 0 && result[len(result)-1].Role == ai.RoleUser {
			result[len(result)-1] = m // replace previous with current (keep last)
			continue
		}

		result = append(result, m)
	}

	return result
}

// buildAssistantMessage creates a ChatMessage for persistence.
func (r *Runner) buildAssistantMessage(
	sessionID uuid.UUID,
	content string,
	toolCalls []ai.ToolCall,
	finishReason string,
	usage ai.Usage,
	turnIndex int,
	responseID string,
) chat.ChatMessage {
	msg := chat.ChatMessage{
		SessionID:    sessionID,
		Role:         "assistant",
		Content:      content,
		MessageType:  chat.MessageTypeText,
		FinishReason: &finishReason,
		TurnIndex:    turnIndex,
		RunID:        &r.runID,
	}

	if r.runID == uuid.Nil {
		msg.RunID = nil
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

	// Persist the provider's response ID so that subsequent runs can resume
	// the response chain without resending the full conversation history.
	if responseID != "" {
		meta := map[string]string{"response_id": responseID}
		if raw, err := json.Marshal(meta); err == nil {
			msg.Metadata = raw
		}
	}

	return msg
}

// executeAgentHubManage handles administrative operations for Auto-Reflection.
// This is the core mechanism that allows an agent to manage the platform itself.
func (r *Runner) executeAgentHubManage(ctx context.Context, ch chan<- RunEvent, tc ai.ToolCall, in RunInput) ToolExecResult {
	start := time.Now()
	input := json.RawMessage(tc.Function.Arguments)

	ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
		ID: tc.ID, Name: tc.Function.Name, Input: input,
	})
	ch <- NewRunEvent(EventToolProgress, ToolProgressData{
		ID: tc.ID, Name: tc.Function.Name, State: ToolStateExecuting,
	})

	// Forward the request to the administrative skill or internal API.
	if r.managementExec != nil {
		var args struct {
			Operation string          `json:"operation"`
			Resource  string          `json:"resource"`
			ID        string          `json:"id"`
			Query     string          `json:"query"`
			Payload   json.RawMessage `json:"payload"`
		}
		if err := json.Unmarshal(input, &args); err == nil {
			execResult := r.managementExec.Execute(ctx, args.Operation, args.Resource, args.ID, args.Query, args.Payload)
			execResult.LatencyMs = time.Since(start).Milliseconds()

			ch <- NewRunEvent(EventToolProgress, ToolProgressData{ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted})
			ch <- NewRunEvent(EventToolResult, ToolResultData{
				ID:         tc.ID,
				Name:       tc.Function.Name,
				Output:     execResult.Output,
				DurationMs: execResult.LatencyMs,
				Error:      execResult.Error,
			})
			execResult.EmittedToStream = true
			return execResult
		}
	}

	// Fallback to skill-runtime if no local executor is attached (legacy/remote).
	execResult, err := r.skillClient.Execute(ctx, "agenthub-admin", input, in.TenantID, in.AgentID.String(), in.SessionID.String())

	latency := time.Since(start).Milliseconds()
	if err != nil {
		errMsg := err.Error()
		res := ToolExecResult{Error: &errMsg, LatencyMs: latency, EmittedToStream: true}
		ch <- NewRunEvent(EventToolProgress, ToolProgressData{ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted})
		ch <- NewRunEvent(EventToolResult, ToolResultData{ID: tc.ID, Name: tc.Function.Name, Output: nil, Error: &errMsg, DurationMs: latency})
		return res
	}

	execResult.LatencyMs = latency
	execResult.EmittedToStream = true
	ch <- NewRunEvent(EventToolProgress, ToolProgressData{ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted})
	ch <- NewRunEvent(EventToolResult, ToolResultData{ID: tc.ID, Name: tc.Function.Name, Output: execResult.Output, DurationMs: latency})
	return *execResult
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

// FormatToolResult produces a string representation of a tool execution result.
// Empty results get a descriptive message instead of "{}" because some models
// interpret empty tool_result content as a stop signal.
// Inspired by Claude Code's toolResultStorage.ts empty result injection.
func FormatToolResult(r ToolExecResult) string {
	if r.Error != nil {
		return fmt.Sprintf("Error: %s", *r.Error)
	}
	if len(r.Output) > 0 {
		s := string(r.Output)
		if strings.TrimSpace(s) != "" && s != "{}" && s != "null" {
			return s
		}
	}
	// Inject descriptive message for empty/trivial results.
	if r.ToolName != "" {
		return fmt.Sprintf("(%s completed with no output)", r.ToolName)
	}
	return "(tool completed with no output)"
}

// tokenUsageWithCost creates a TokenUsage with cost and model information.
func tokenUsageWithCost(usage ai.Usage, cost float64, model string) TokenUsage {
	return TokenUsage{
		PromptTokens:        usage.PromptTokens,
		CompletionTokens:    usage.CompletionTokens,
		TotalTokens:         usage.TotalTokens,
		CacheReadTokens:     usage.CacheReadTokens,
		CacheCreationTokens: usage.CacheCreationTokens,
		CostUSD:             cost,
		Model:               model,
	}
}

// resolveToolResultLimit returns the effective max result chars for a tool,
// preferring the per-tool override, then the global config default.
func resolveToolResultLimit(toolName string, toolLimits map[string]int, globalMax int) int {
	if limit, ok := toolLimits[toolName]; ok && limit > 0 {
		return limit
	}
	return globalMax
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

// executeWithPermissions evaluates permission rules for each tool call, executes
// permitted ones via StreamingToolExecutor, and returns results in the same order
// as the input toolCalls. Denied/confirm tools get error results without execution.
// Agent tool calls are routed to the SubtaskExecutor for sub-agent spawning.
func (r *Runner) executeWithPermissions(ctx context.Context, ch chan<- RunEvent, toolCalls []ai.ToolCall, in RunInput, totalCost float64, readOnlyIndex map[string]bool, destructiveIndex map[string]bool, deferredTools []LLMTool, allowedToolsIndex map[string]bool) []ToolExecResult {
	results := make([]ToolExecResult, len(toolCalls))

	// Partition tool calls into categories.
	var regularTools []ai.ToolCall
	regularIdx := map[int]int{} // original index → regular index
	var agentTools []ai.ToolCall
	agentIdx := map[int]int{} // original index → agent index

	for i, tc := range toolCalls {
		// Reject tool calls for tools not in the allowed set for this run.
		// This prevents the LLM from calling tools it remembers from conversation
		// history that are no longer bound to the agent (P-SK6).
		if len(allowedToolsIndex) > 0 && !allowedToolsIndex[tc.Function.Name] {
			errMsg := fmt.Sprintf("Tool '%s' is not available for this agent.", tc.Function.Name)
			results[i] = ToolExecResult{Error: &errMsg, ToolName: tc.Function.Name}
			ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Input: json.RawMessage(tc.Function.Arguments),
			})
			ch <- NewRunEvent(EventToolResult, ToolResultData{
				ID:    tc.ID,
				Name:  tc.Function.Name,
				Error: &errMsg,
			})
			continue
		}

		// Check permissions first.
		if in.PermissionRules != nil {
			decision := EvaluatePermission(in.PermissionRules, tc.Function.Name, tc.Function.Arguments)
			switch decision {
			case PermissionDeny:
				errMsg := FormatDeniedError(tc.Function.Name)
				results[i] = ToolExecResult{Error: &errMsg}
				// Track denial and emit event.
				denialCount := 1
				escalated := false
				if r.denialTracker != nil {
					escalated = r.denialTracker.RecordDenial(tc.Function.Name, tc.Function.Arguments)
					if rec := r.denialTracker.GetRecord(tc.Function.Name); rec != nil {
						denialCount = rec.Count
					}
				}
				ch <- NewRunEvent(EventToolDenied, ToolDeniedData{
					ID:          tc.ID,
					Name:        tc.Function.Name,
					Reason:      errMsg,
					DenialCount: denialCount,
					Escalated:   escalated,
				})
				continue

			case PermissionConfirm:
				errMsg := fmt.Sprintf("Tool '%s' requires confirmation but running in automated mode.", tc.Function.Name)
				results[i] = ToolExecResult{Error: &errMsg}

				// Track as denial too — confirm in automated mode is effectively a deny.
				if r.denialTracker != nil {
					r.denialTracker.RecordDenial(tc.Function.Name, tc.Function.Arguments)
				}
				ch <- NewRunEvent(EventToolDenied, ToolDeniedData{
					ID:     tc.ID,
					Name:   tc.Function.Name,
					Reason: errMsg,
				})
				continue
			}
		}

		// Auto-require confirmation for destructive tools (delete, drop, overwrite)
		// even when permission rules would allow them. This is a safety net inspired
		// by Claude Code's isDestructive per-tool flag (Tool.ts).
		if destructiveIndex[tc.Function.Name] {
			errMsg := fmt.Sprintf(
				"Tool '%s' is flagged as destructive (irreversible operation). "+
					"Automated execution is blocked — this operation requires explicit user confirmation.",
				tc.Function.Name)
			results[i] = ToolExecResult{Error: &errMsg, ToolName: tc.Function.Name}
			denialCount := 0
			if r.denialTracker != nil {
				denialCount = r.denialTracker.TotalDenials()
			}
			ch <- NewRunEvent(EventToolDenied, ToolDeniedData{
				ID:          tc.ID,
				Name:        tc.Function.Name,
				Reason:      "destructive operation requires confirmation",
				DenialCount: denialCount,
			})
			continue
		}

		// Route ask_user calls to the ElicitationHandler — block until the user
		// submits a response via POST /elicitation/{requestId}/respond.
		if tc.Function.Name == "ask_user" && in.Elicitation != nil {
			slog.Info("agentic: ask_user intercepted — blocking for user input", "toolCallID", tc.ID, "args", tc.Function.Arguments)
			var params struct {
				Message         string            `json:"message"`
				Schema          json.RawMessage   `json:"schema"`
				InputSchema     json.RawMessage   `json:"inputSchema"`
				RequestedSchema json.RawMessage   `json:"requestedSchema"`
				Questions       []AskUserQuestion `json:"questions"`
			}
			_ = json.Unmarshal(json.RawMessage(tc.Function.Arguments), &params)
			requestedSchema := params.Schema
			if len(requestedSchema) == 0 {
				requestedSchema = params.InputSchema
			}
			if len(requestedSchema) == 0 {
				requestedSchema = params.RequestedSchema
			}
			elicParams := ElicitationParams{
				Mode:            ElicitationModeForm,
				Message:         params.Message,
				RequestedSchema: requestedSchema,
				Questions:       params.Questions,
			}
			ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
				ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(tc.Function.Arguments),
			})
			result := in.Elicitation.Submit(ctx, "", tc.ID, elicParams)
			slog.Info("agentic: ask_user Submit returned", "toolCallID", tc.ID, "action", result.Action, "contentKeys", mapKeys(result.Content), "ctxErr", ctx.Err())
			var output json.RawMessage
			if result.Action == ElicitationCancel || result.Action == ElicitationDecline {
				msg := fmt.Sprintf(`{"action": %q}`, result.Action)
				output = json.RawMessage(msg)
			} else {
				out, _ := json.Marshal(result.Content)
				output = out
			}
			results[i] = ToolExecResult{Output: output}
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted,
			})
			continue
		}

		// Builtin: agenthub_manage — handles administrative operations via reflection.
		if tc.Function.Name == "agenthub_manage" {
			execResult := r.executeAgentHubManage(ctx, ch, tc, in)
			results[i] = execResult
			continue
		}

		// Route tool_search calls locally — resolve deferred tool schemas without
		// hitting the skill-runtime. Inspired by Claude Code's ToolSearchTool.
		if IsToolSearchCall(tc.Function.Name) && len(deferredTools) > 0 {
			result := ExecuteToolSearch(json.RawMessage(tc.Function.Arguments), deferredTools)
			results[i] = result
			ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
				ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(tc.Function.Arguments),
			})
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted,
			})
			continue
		}

		// Route agent tool calls to SubtaskExecutor.
		if IsAgentToolCall(tc) && r.subtaskExec != nil {
			agentIdx[i] = len(agentTools)
			agentTools = append(agentTools, tc)
			continue
		}

		// Route send_message tool calls to the mailbox handler.
		if IsSendMessageToolCall(tc.Function.Name) && r.agentMailbox != nil {
			sessionID := in.ParentSessionID
			if sessionID == uuid.Nil {
				sessionID = in.SessionID
			}
			results[i] = HandleSendMessage(r.agentMailbox, sessionID, in.SubtaskID, json.RawMessage(tc.Function.Arguments), ch)
			continue
		}

		regularIdx[i] = len(regularTools)
		regularTools = append(regularTools, tc)
	}

	// Execute regular tools.
	if len(regularTools) > 0 {
		execResults := r.toolExec.ExecuteAll(ctx, ch, regularTools, in, readOnlyIndex)
		for origIdx, regIdx := range regularIdx {
			if regIdx < len(execResults) {
				results[origIdx] = execResults[regIdx]
				// Reset denial counter on successful execution.
				if r.denialTracker != nil && execResults[regIdx].Error == nil {
					r.denialTracker.RecordAllow(regularTools[regIdx].Function.Name)
				}
			}
		}
	}

	// Execute agent tool calls (sub-agents).
	if len(agentTools) > 0 {
		agentResults := r.subtaskExec.ExecuteParallel(ctx, ch, agentTools, in, r.config, totalCost)
		for origIdx, agtIdx := range agentIdx {
			if agtIdx < len(agentResults) {
				results[origIdx] = agentResults[agtIdx]
			}
		}
	}

	return results
}

// executeTurnEndHooks runs all registered turn-end handlers.
func (r *Runner) executeTurnEndHooks(ctx context.Context, ch chan<- RunEvent, payload TurnEndPayload) {
	if r.toolExec != nil && r.toolExec.hookExecutor != nil {
		r.toolExec.hookExecutor.ExecuteTurnEnd(ctx, payload, r.turnEndHandlers)
	} else {
		// No hook executor — run in-memory handlers directly.
		for _, h := range r.turnEndHandlers {
			if err := h.HandleTurnEnd(ctx, payload); err != nil {
				emitError(ch, "turn_end_handler", err)
			}
		}
	}
}

// executeRunEndHooks runs all registered run-end handlers.
func (r *Runner) executeRunEndHooks(ctx context.Context, ch chan<- RunEvent, payload RunEndPayload) {
	if r.toolExec != nil && r.toolExec.hookExecutor != nil {
		r.toolExec.hookExecutor.ExecuteRunEnd(ctx, payload, r.runEndHandlers)
	} else {
		for _, h := range r.runEndHandlers {
			if err := h.HandleRunEnd(ctx, payload); err != nil {
				emitError(ch, "run_end_handler", err)
			}
		}
	}
}

func (r *Runner) handlePostTurnLifecycle(ctx context.Context, ch chan<- RunEvent, hookCtx StopHookContext) StopHookResult {
	var hookExecutor *HookExecutor
	if r.toolExec != nil {
		hookExecutor = r.toolExec.hookExecutor
	}
	if r.memoryExtractor == nil && r.cacheSafeSnap == nil && hookExecutor == nil {
		return StopHookResult{}
	}
	return NewStopHooksOrchestrator(
		hookExecutor,
		r.memoryExtractor,
		r.cacheSafeSnap,
		ch,
	).HandleStopHooks(ctx, hookCtx)
}

func (r *Runner) buildToolUseSummary(
	ctx context.Context,
	agentID uuid.UUID,
	toolCalls []ai.ToolCall,
	toolResults []ToolExecResult,
	lastAssistantText string,
) string {
	if r.toolSummary == nil || len(toolCalls) == 0 || len(toolResults) == 0 {
		return ""
	}

	infos := make([]ToolSummaryInfo, 0, len(toolCalls))
	for i, tc := range toolCalls {
		info := ToolSummaryInfo{
			Name:  tc.Function.Name,
			Input: json.RawMessage(tc.Function.Arguments),
		}
		if i < len(toolResults) {
			info.Output = toolResults[i].Output
			info.Error = toolResults[i].Error
		}
		infos = append(infos, info)
	}

	return r.toolSummary.Generate(ctx, agentID, infos, lastAssistantText)
}

func emitError(ch chan<- RunEvent, code string, err error) {
	ch <- NewRunEvent(EventError, ErrorData{
		Message: err.Error(),
		Code:    code,
	})
}

func emitWarning(ch chan<- RunEvent, code string, message string) {
	ch <- NewRunEvent(EventWarning, WarningData{
		Message: message,
		Code:    code,
	})
}

func mapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// chatMessagesToAI converts chat.ChatMessage slice back to ai.Message slice.
func chatMessagesToAI(msgs []chat.ChatMessage) []ai.Message {
	result := make([]ai.Message, len(msgs))
	for i, m := range msgs {
		result[i] = ai.Message{
			Role:       m.Role,
			Content:    m.Content,
			ToolCallID: derefString(m.ToolCallID),
		}
		if len(m.ToolCalls) > 0 {
			var tcs []ai.ToolCall
			if err := json.Unmarshal(m.ToolCalls, &tcs); err == nil {
				result[i].ToolCalls = tcs
			}
		}
	}
	return result
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

// --- Effort level support ---

// ModelSupportsEffort returns true if the model supports the effort parameter.
// Currently supported by Claude Opus 4.6 and Sonnet 4.6.
// Inspired by Claude Code's modelSupportsEffort in effort.ts.
func ModelSupportsEffort(model string) bool {
	effortPrefixes := []string{
		"claude-opus-4-6",
		"claude-sonnet-4-6",
	}
	lower := strings.ToLower(model)
	for _, prefix := range effortPrefixes {
		if len(lower) >= len(prefix) && lower[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// ModelSupportsMaxEffort returns true if the model supports "max" effort.
// Per API docs, "max" is only valid for Opus 4.6. Other models return an error.
func ModelSupportsMaxEffort(model string) bool {
	return strings.Contains(strings.ToLower(model), "opus-4-6")
}

// ResolveEffortLevel determines the effective effort level based on config and model.
// Returns nil if no effort parameter should be sent (API defaults to "high").
// Clamps "max" to "high" for models that don't support it.
// Inspired by Claude Code's resolveAppliedEffort in effort.ts.
func ResolveEffortLevel(cfg RunConfig) *ai.EffortLevel {
	if cfg.Effort == nil {
		return nil
	}
	if !ModelSupportsEffort(cfg.Model) {
		return nil
	}

	effort := *cfg.Effort

	// API rejects "max" on non-Opus-4.6 models — downgrade to "high".
	if effort == ai.EffortMax && !ModelSupportsMaxEffort(cfg.Model) {
		high := ai.EffortHigh
		return &high
	}

	return &effort
}

// --- Thinking support ---

// modelSupportsThinking returns true if the model supports extended thinking.
func modelSupportsThinking(model string) bool {
	thinkingPrefixes := []string{
		"claude-opus-4",
		"claude-sonnet-4",
		"claude-haiku-4",
	}
	for _, prefix := range thinkingPrefixes {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// modelSupportsAdaptiveThinking returns true if the model supports adaptive thinking
// (where the model decides when and how much to think).
func modelSupportsAdaptiveThinking(model string) bool {
	adaptivePrefixes := []string{
		"claude-opus-4-6",
		"claude-sonnet-4-6",
	}
	for _, prefix := range adaptivePrefixes {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// resolveThinkingConfig determines the effective ThinkingConfig based on the
// RunConfig settings and model capabilities. Returns nil if thinking is disabled
// or the model doesn't support it.
func resolveThinkingConfig(cfg RunConfig) *ai.ThinkingConfig {
	if cfg.Thinking == nil || cfg.Thinking.Type == ai.ThinkingDisabled {
		return nil
	}
	if !modelSupportsThinking(cfg.Model) {
		return nil
	}

	// Adaptive mode: let the model decide.
	if cfg.Thinking.Type == ai.ThinkingAdaptive {
		if modelSupportsAdaptiveThinking(cfg.Model) {
			return &ai.ThinkingConfig{Type: ai.ThinkingAdaptive}
		}
		// Fallback to enabled with default budget for models that support
		// thinking but not adaptive.
		return &ai.ThinkingConfig{
			Type:         ai.ThinkingEnabled,
			BudgetTokens: defaultThinkingBudget(cfg.MaxTokensPerCall),
		}
	}

	// Enabled mode: use explicit budget.
	budget := cfg.Thinking.BudgetTokens
	if budget <= 0 {
		budget = defaultThinkingBudget(cfg.MaxTokensPerCall)
	}
	// Budget must be less than max_tokens.
	if budget >= cfg.MaxTokensPerCall {
		budget = cfg.MaxTokensPerCall - 1
	}
	return &ai.ThinkingConfig{
		Type:         ai.ThinkingEnabled,
		BudgetTokens: budget,
	}
}

// defaultThinkingBudget returns a sensible default thinking budget
// based on the max output tokens (approximately 80% of max_tokens).
func defaultThinkingBudget(maxTokens int) int {
	budget := maxTokens * 4 / 5
	if budget < 1024 {
		budget = 1024
	}
	return budget
}
