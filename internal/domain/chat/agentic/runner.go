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
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic/processors"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledge"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// MessagePersister is the subset of chat.Repository used by the Runner to persist messages.
type MessagePersister interface {
	CreateMessage(ctx context.Context, m chat.ChatMessage) (chat.ChatMessage, error)
}

// RunMetadataPersister is the subset of chat.Repository used to persist run metrics.
// P-C325-2: populated via the runner's defer block on both success and failure.
type RunMetadataPersister interface {
	UpdateRunMetadata(ctx context.Context, id uuid.UUID, metadata json.RawMessage) error
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
	RequestContext  RequestContext

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

	// FrontendActions, when set, exposes CopilotKit-style frontend actions —
	// tools whose names match are intercepted at the routing point: the runner
	// emits an EventFrontendActionCall and blocks via Submit() until the
	// client posts the result back through POST /client-state. CopilotKit
	// Phase 1.
	FrontendActions FrontendActionsProvider

	// IsAdmin, when true, grants access to the agenthub_manage builtin tool.
	// P-C298-1: set from the caller's JWT "admin" realm role.
	IsAdmin bool

	// EnableManagement, when true, indicates the agent has opted in to management tools.
	// P-C184-2: must be combined with IsAdmin=true AND CurrentDepth==0 to include agenthub_manage.
	EnableManagement bool

	// DisableAskUser removes the ask_user builtin from the agent's tool set.
	// Read from agent.Config["disableAskUser"] by the chat service. Default false.
	DisableAskUser bool

	// DisableAgentDelegation removes the agent (sub-agent spawner) builtin.
	// Read from agent.Config["disableAgentDelegation"]. Default false.
	DisableAgentDelegation bool

	// UserMessageID, when non-nil, indicates the user message was already persisted
	// by the caller (chat.Service.RunSession). The runner skips its own persistence
	// to avoid duplicates. P-C178-2.
	UserMessageID *uuid.UUID

	// SkillIDsSnapshot, when non-empty, contains the skill IDs captured at session
	// creation. The toolBuilder uses these instead of the agent's current bindings
	// so the tool set stays consistent throughout the conversation. P-C115-1.
	SkillIDsSnapshot []uuid.UUID

	// MCPServerNamesSnapshot, when non-empty, contains the MCP server names bound to
	// the agent at session creation. Only tools from servers in this list are exposed
	// to the LLM. P-C253-1: agent-level MCP filtering.
	MCPServerNamesSnapshot []string
	// OutputProcessors contains ordered built-in processor names applied to
	// assistant text before it is persisted or streamed to callers.
	OutputProcessors []string

	// SearchOrReadTools marks tool results that clients should collapse by default.
	// It is populated only from the server-side tool schema for this run.
	SearchOrReadTools map[string]bool
	// InterruptBehaviors maps a tool name to its server-side interrupt behavior.
	// Only "block" is present; an absent entry defaults to immediate cancellation.
	InterruptBehaviors map[string]string

	// PermissionAudit, when set, records each permission decision to the audit log.
	// When nil, decisions are silently skipped (no-op).
	PermissionAudit PermissionAuditLogger
}

const toolResultPersistenceTimeout = 5 * time.Second

// contextForToolResultPersistence keeps a completed tool outcome durable when
// an interrupt cancelled the run context while a block-on-interrupt tool ran.
// It is bounded and carries the original context values, but never resumes the
// agentic loop after persistence.
func contextForToolResultPersistence(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx.Err() == nil {
		return ctx, func() {}
	}
	return context.WithTimeout(context.WithoutCancel(ctx), toolResultPersistenceTimeout)
}

// RunMetadata aggregates observability metrics collected during an agentic run.
// Persisted into chat_run.metadata at run completion (success or failure). P-C325-2.
type RunMetadata struct {
	TotalTurns        int     `json:"totalTurns"`
	TotalInputTokens  int     `json:"totalInputTokens"`
	TotalOutputTokens int     `json:"totalOutputTokens"`
	TotalCostUSD      float64 `json:"totalCostUSD"`
	ModelUsed         string  `json:"modelUsed"`
	ProviderUsed      string  `json:"providerUsed"`
	FinishReason      string  `json:"finishReason"`
	HadToolFailures   bool    `json:"hadToolFailures"`
	ToolCallCount     int     `json:"toolCallCount"`
	DurationMs        int64   `json:"durationMs"`
	ErrorMessage      *string `json:"errorMessage,omitempty"`
}

// Runner orchestrates the agentic loop: LLM → tool_calls → execution → tool_results → LLM.
type Runner struct {
	chatModel         ai.ChatModel
	skillClient       *SkillRuntimeClient
	prompt            *PromptBuilder
	promptCache       map[string]string // session-scoped prompt section cache (owned per Runner)
	tools             *ToolSchemaBuilder
	mcpClient         MCPClientService
	ctxManager        *ContextManager
	memory            *MemoryBridge
	persister         MessagePersister
	metadataPersister RunMetadataPersister // optional — nil for sub-runners
	history           HistoryLoader
	toolExec          *StreamingToolExecutor
	subtaskExec       *SubtaskExecutor
	agentMailbox      *AgentMailbox
	managementExec    *ManagementExecutor
	denialTracker     *DenialTracker
	turnEndHandlers   []TurnEndHandler
	runEndHandlers    []RunEndHandler
	toolSummary       *ToolUseSummaryGenerator
	memoryExtractor   *SessionMemoryExtractor
	cacheSafeSnap     *CacheSafeParamsSnapshot
	progress          *RunProgressTracker
	commands          *CommandRegistry
	config            RunConfig
	runID             uuid.UUID
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
		promptCache:   make(map[string]string),
		tools:         tools,
		ctxManager:    ctxManager,
		memory:        memory,
		persister:     persister,
		history:       history,
		toolExec:      NewStreamingToolExecutor(skillClient, hookExecutor, config),
		progress:      NewRunProgressTracker(10),
		commands:      NewCommandRegistry(),
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

// WithDocumentSearch wires the document search client into the tool executor.
// P-E1-2: enables the document_search builtin tool when an active knowledge base
// is linked to the agent.
func (r *Runner) WithDocumentSearch(client knowledge.DocumentSearchClient, kbIDs []uuid.UUID) *Runner {
	r.toolExec.WithDocumentSearch(client, kbIDs)
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

// WithMetadataPersister wires the repository used to persist run metrics after completion.
// P-C325-2: only the root runner (not sub-runners) should call this.
func (r *Runner) WithMetadataPersister(p RunMetadataPersister) *Runner {
	r.metadataPersister = p
	return r
}

// maxToolRetries is the maximum number of times a given tool may be called within
// a single run. After this limit the runner returns a synthetic error result to the
// LLM instead of executing the tool, preventing infinite tool-error retry loops.
// P-C57-3: padrão recorrente de LLM reattempting the same failing tool indefinitely.
const maxToolRetries = 3

// runState holds mutable per-run counters that are reset at the start of each Run.
type runState struct {
	// toolRetries maps tool name → number of times it has been invoked this run.
	toolRetries map[string]int
	// storedMemoryKeys is the set of normalised keys stored by memory_store this run.
	// BUG-MEM-STRESS1 fix: tracking per-key instead of a single bool allows the LLM
	// to store multiple distinct facts in a single run (bulk memorisation). Only the
	// exact same normalised key is blocked on a second call, not every subsequent call.
	storedMemoryKeys map[string]struct{}
	// collectedUserValues accumulates all values accepted by the user via ask_user
	// this run. BUG-ASK_USER-LOOP fix: when the LLM calls ask_user a second time for
	// fields already collected, the runner auto-answers with the cached values instead
	// of blocking for user input again. This prevents infinite ask_user loops observed
	// with models like gpt-oss-120b that do not follow the "_instruction" hint.
	collectedUserValues map[string]any
	// frontendActionsByName is the set of CopilotKit frontend action names declared by
	// the client for this session. Tool calls whose name matches are routed through
	// FrontendActionsProvider.Submit instead of local execution.
	frontendActionsByName map[string]bool
}

// newRunState initialises a fresh runState for a new run.
func newRunState() *runState {
	return &runState{
		toolRetries:           make(map[string]int),
		storedMemoryKeys:      make(map[string]struct{}),
		collectedUserValues:   make(map[string]any),
		frontendActionsByName: make(map[string]bool),
	}
}

// checkAndIncrementRetry returns an error when the tool has already been called
// maxToolRetries times, otherwise increments the counter and returns nil.
// P-C57-3: prevents the LLM from retrying a consistently-failing tool forever.
func (s *runState) checkAndIncrementRetry(toolName string) error {
	count := s.toolRetries[toolName]
	if count >= maxToolRetries {
		return fmt.Errorf("tool %q reached the maximum call limit (%d per run) — please proceed without it",
			toolName, maxToolRetries)
	}
	s.toolRetries[toolName] = count + 1
	return nil
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
	// P-C57-3: initialise per-run tool retry counters.
	rs := newRunState()

	// P-C325-2: track metrics for run metadata persistence.
	runStart := time.Now()
	runTiming := &RunTimingData{}
	var firstOutputAt time.Time
	markFirstOutput := func() {
		if firstOutputAt.IsZero() {
			firstOutputAt = time.Now()
		}
	}
	runTimingSnapshot := func() *RunTimingData {
		if !firstOutputAt.IsZero() {
			runTiming.FirstOutputMS = firstOutputAt.Sub(runStart).Milliseconds()
		}
		runTiming.TotalMS = time.Since(runStart).Milliseconds()
		return runTiming
	}
	var runHadToolFailures bool
	var runToolCallCount int
	var runFinishReason string

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

	// lastRunError tracks the most recent fatal error so the deferred cleanup
	// can persist it as an assistant message (P-P1: run errors were silently
	// dropped from chat history — user saw no response after a failed run).
	var lastRunError *ErrorData

	// localEmitError wraps emitError to record the error for deferred persistence.
	// P-C96-1: emit the friendly message to the SSE stream instead of the raw error,
	// so the frontend and the DB history receive consistent, user-facing messages.
	//
	// Bug 201: detecta context cancellation antes de qualquer code específico.
	// Quando o usuário cancela o run, o erro original é context.Canceled mas
	// o code emitido era específico da operação em curso (load_history,
	// llm_call, prompt_build etc), gerando mensagens confusas. Override para
	// "context_cancelled" garante mensagem correta "A solicitação foi cancelada".
	localEmitError := func(code string, err error) {
		if errors.Is(err, context.Canceled) || ctx.Err() != nil {
			code = "context_cancelled"
		}
		safeMsg := sanitizeSSEMessage(err.Error())
		lastRunError = &ErrorData{Message: safeMsg, Code: code}
		slog.Warn("agentic: run error", "code", code, "error", safeMsg)
		ch <- NewRunEvent(EventError, ErrorData{Message: safeMsg, Code: code})
	}

	defer func() {
		// Persist a system error message so the chat history always reflects what
		// happened — even when the run fails before producing any assistant output.
		// Use context.Background() because the request context may already be
		// cancelled by the time this deferred function executes.
		if lastRunError != nil {
			errContent := friendlyRunErrorMessage(lastRunError.Code, lastRunError.Message)
			msg := chat.ChatMessage{
				SessionID:   in.SessionID,
				Role:        "assistant",
				Content:     errContent,
				MessageType: chat.MessageTypeText,
				// RunID intentionally omitted: the run record may not exist in the DB
				// when the error occurs before run creation (e.g. LLM API key invalid),
				// which would cause a FK violation on chat_message_run_id_fkey.
			}
			// Use WithoutCancel to preserve tenant/values from ctx without inheriting its cancellation.
			persistCtx := context.WithoutCancel(ctx)
			if _, persistErr := r.persister.CreateMessage(persistCtx, msg); persistErr != nil {
				slog.Warn("runner: failed to persist run error message", "error", persistErr)
			}
		}
		if !runCompleted {
			ch <- NewRunEvent(EventRunComplete, RunCompleteData{
				TotalTurns:                    turnIndex,
				TotalTokens:                   totalTokens,
				TotalCost:                     totalCost,
				LatestInputTokens:             latestInputTokens,
				CumulativeOutputTokens:        totalOutputTokens,
				CumulativeCacheReadTokens:     cumulativeCacheReadTokens,
				CumulativeCacheCreationTokens: cumulativeCacheCreationTokens,
				Timing:                        runTimingSnapshot(),
			})
		}
		// P-C325-2: persist run metadata so observability tooling can query cost,
		// latency and failure data without parsing SSE event streams.
		if r.metadataPersister != nil && r.runID != uuid.Nil {
			finishReason := runFinishReason
			if finishReason == "" && lastRunError != nil {
				finishReason = "error"
			}
			// turnIndex is incremented only after tool_call turns; for a single
			// text-only turn that exits via "stop", turnIndex is still 0 but one
			// full turn did complete. Use turnIndex+1 when the run finished
			// normally (runCompleted=true) to accurately reflect the turn count.
			totalTurnsForMeta := turnIndex
			if runCompleted {
				totalTurnsForMeta = turnIndex + 1
			}
			meta := RunMetadata{
				TotalTurns:        totalTurnsForMeta,
				TotalInputTokens:  latestInputTokens,
				TotalOutputTokens: totalOutputTokens,
				TotalCostUSD:      totalCost,
				ModelUsed:         r.config.Model,
				ProviderUsed:      r.config.Provider,
				FinishReason:      finishReason,
				HadToolFailures:   runHadToolFailures,
				ToolCallCount:     runToolCallCount,
				DurationMs:        time.Since(runStart).Milliseconds(),
			}
			if lastRunError != nil {
				errMsg := lastRunError.Message
				meta.ErrorMessage = &errMsg
			}
			if metaJSON, err := json.Marshal(meta); err == nil {
				persistCtx := context.WithoutCancel(ctx)
				if err := r.metadataPersister.UpdateRunMetadata(persistCtx, r.runID, metaJSON); err != nil {
					slog.Warn("runner: failed to persist run metadata", "error", err, "runID", r.runID)
				}
			}
		}
	}()

	// Snapshot immutable gates once at run start. These pre-computed flags
	// prevent re-evaluating conditions on every loop iteration.
	gates := BuildRunGates(r.config, in.CurrentDepth, r.subtaskExec != nil)

	// 1. Recall memories (non-fatal on failure).
	//
	// A recall miss is expected and recoverable: the turn proceeds with an
	// empty memories string. Surface it as a warning (not an error) so the UI
	// does not light up red on every run when the embedding service is slow.
	memories := ""
	if r.memory != nil {
		var err error
		memories, err = r.memory.Recall(ctx, in.AgentID, in.UserMessage)
		if err != nil {
			slog.Warn("runner: memory recall failed, continuing without memories",
				"agentID", in.AgentID, "error", err)
			emitWarning(ch, "memory_recall", scrubInternalNetwork(err.Error()))
		}
	}

	// 1b. Intercept slash commands before touching the LLM.
	// CommandRegistry is pre-wired in NewRunner(); if the user message starts with '/'
	// and matches a registered command, we execute it locally, persist the response as
	// an assistant message, emit text_delta + run_complete, and return without ever
	// building tool schemas or calling the LLM.
	if r.commands != nil {
		if cmd, args, ok := r.commands.Parse(in.UserMessage); ok {
			cc := CommandContext{
				SessionID: in.SessionID.String(),
				AgentID:   in.AgentID.String(),
				TenantID:  in.TenantID,
			}
			result, cmdErr := r.commands.Execute(ctx, cmd, args, cc)
			if cmdErr != nil {
				localEmitError("slash_command", cmdErr)
				return
			}
			output, err := processAssistantOutput(ctx, result.Output, in.OutputProcessors)
			if err != nil {
				localEmitError("output_processor", err)
				return
			}
			// Persist user message so history is consistent.
			userCmdMsg := chat.ChatMessage{
				SessionID:   in.SessionID,
				Role:        "user",
				Content:     in.UserMessage,
				MessageType: chat.MessageTypeText,
				RunID:       &r.runID,
			}
			if r.runID == uuid.Nil {
				userCmdMsg.RunID = nil
			}
			if _, err := r.persister.CreateMessage(ctx, userCmdMsg); err != nil {
				localEmitError("persist_user_msg", err)
				return
			}
			// Persist assistant response.
			assistantCmdMsg := chat.ChatMessage{
				SessionID:   in.SessionID,
				Role:        "assistant",
				Content:     output,
				MessageType: chat.MessageTypeText,
				RunID:       &r.runID,
			}
			if r.runID == uuid.Nil {
				assistantCmdMsg.RunID = nil
			}
			if _, err := r.persister.CreateMessage(ctx, assistantCmdMsg); err != nil {
				localEmitError("persist_assistant_msg", err)
				return
			}
			// Emit the command output as a text stream.
			markFirstOutput()
			ch <- NewRunEvent(EventTextDelta, TextDeltaData{Content: output})
			runCompleted = true
			ch <- NewRunEvent(EventRunComplete, RunCompleteData{
				TotalTurns:  1,
				TotalTokens: 0,
				TotalCost:   0,
				Timing:      runTimingSnapshot(),
			})
			return
		}
	}

	// 2. Build tool schemas with deferred loading (depth limits for sub-agent availability).
	// When the total tool count exceeds DeferredToolThreshold, tools marked ShouldDefer
	// are separated — only their names go into the system prompt, and the LLM must call
	// tool_search to load their full schemas on demand.
	// Inspired by Claude Code's isDeferredTool + ToolSearchTool pattern.
	toolBuilder := r.tools.Clone()
	if toolBuilder == nil {
		localEmitError("tool_schema_build", fmt.Errorf("tool builder not configured"))
		return
	}

	if r.mcpClient != nil {
		bridge := NewMCPToolBridge(r.mcpClient, in.TenantID)
		// P-C253-1: filter MCP tools to only those from bound servers.
		// Use nil-check (not len>0) to support "has bindings but all disabled" (empty non-nil slice).
		if in.MCPServerNamesSnapshot != nil {
			bridge.WithAllowedServerNames(in.MCPServerNamesSnapshot)
		}
		toolBuilder.WithMCPBridge(bridge)
		if r.toolExec != nil {
			r.toolExec.WithMCPBridge(bridge)
		}
	} else if r.toolExec != nil {
		r.toolExec.WithMCPBridge(nil)
	}

	toolBuilder.WithDepthLimits(in.CurrentDepth, r.config.MaxDepth)
	toolBuilder.WithAdminScope(in.IsAdmin)                // P-C298-1: gate agenthub_manage on admin role
	toolBuilder.WithEnableManagement(in.EnableManagement) // P-C184-2: gate on agent opt-in flag
	toolBuilder.WithDisableAskUser(in.DisableAskUser)
	toolBuilder.WithDisableAgentDelegation(in.DisableAgentDelegation)
	toolBuilder.WithRequestRoles(in.RequestContext.UserRoles)
	// P-C115-1: use skill snapshot IDs when available to ensure consistent tool set.
	if len(in.SkillIDsSnapshot) > 0 {
		toolBuilder.WithSkillIDsSnapshot(in.SkillIDsSnapshot)
	}
	toolResult, err := toolBuilder.BuildWithDeferred(ctx, in.AgentID)
	if err != nil {
		// localEmitError auto-detecta context.Canceled (bug 201) e força
		// code "context_cancelled" para mensagem correta ao usuário.
		localEmitError("tool_schema_build", err)
		return
	}
	for _, w := range toolResult.Warnings {
		emitWarning(ch, "mcp_load_failed", w)
	}
	// Apply the pool-time permission prefilter before tools are advertised to the
	// model. Keep toolResult.All intact for the call-time permission gate: a model
	// that hallucinates a hidden tool still receives an explicit denial instead of
	// reaching its executor.
	advertisedLoadedTools := filterLLMToolsByPermission(toolResult.Loaded, in.PermissionRules)
	advertisedDeferredTools := filterLLMToolsByPermission(toolResult.Deferred, in.PermissionRules)
	advertisedTools := append(append([]LLMTool(nil), advertisedLoadedTools...), advertisedDeferredTools...)

	aiTools := convertLLMToolsToAI(advertisedLoadedTools)
	toolNames := make([]string, len(advertisedLoadedTools))
	for i, t := range advertisedLoadedTools {
		toolNames[i] = t.Name
	}

	// CopilotKit Phase 1: append client-declared frontend actions as LLM tools.
	// They are routed by name at the execution point — see the interception block
	// in executeWithPermissions for the Submit() handoff.
	if in.FrontendActions != nil {
		frontendActions := in.FrontendActions.GetActions(in.SessionID)
		for _, fa := range frontendActions {
			var params map[string]any
			if len(fa.Parameters) > 0 {
				_ = json.Unmarshal(fa.Parameters, &params)
			}
			if params == nil {
				params = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			aiTools = append(aiTools, ai.Tool{
				Type: "function",
				Function: ai.ToolSchema{
					Name:        fa.Name,
					Description: fa.Description,
					Parameters:  params,
				},
			})
			toolNames = append(toolNames, fa.Name)
			rs.frontendActionsByName[fa.Name] = true
		}
	}

	slog.Info("agentic: tools loaded for LLM", "count", len(aiTools), "tools", toolNames, "agentID", in.AgentID)
	readOnlyIndex := BuildReadOnlyIndex(toolResult.All)
	inputSchemaIndex := BuildInputSchemaIndex(toolResult.All)
	destructiveIndex := BuildDestructiveIndex(toolResult.All)
	contextModeTools := BuildContextModeToolIndex(toolResult.All)
	interruptBehaviorIndex := BuildInterruptBehaviorIndex(toolResult.All)
	searchOrReadIndex := BuildSearchOrReadIndex(toolResult.All)
	deferredTools := advertisedDeferredTools

	// Build allowed-tool index: only tools in toolResult.All (loaded + deferred) may be
	// executed in this run. This prevents the LLM from calling tools it "remembers" from
	// prior turns that are no longer bound to the agent (P-SK6 security fix).
	allowedToolsIndex := make(map[string]bool, len(toolResult.All))
	for _, t := range toolResult.All {
		allowedToolsIndex[t.Name] = true
	}
	// Frontend actions are allowed for this run too — they bypass the skill-binding
	// check because the client declared them, not the agent config.
	for name := range rs.frontendActionsByName {
		allowedToolsIndex[name] = true
	}
	in.InterruptBehaviors = interruptBehaviorIndex

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
	// BUG-SKILL-EMPTY: derive active skill slug set from the advertised tools so that
	// formatToolsSection can omit skills with no callable tool binding from the
	// "## Available Tools" section. The permission prefilter must apply to both
	// the JSON schemas and this prompt section to avoid advertising denied tools.
	activeSkillSlugs := make(map[string]bool, len(advertisedTools))
	for _, t := range advertisedTools {
		if !t.Builtin && !t.DisableModelInvocation {
			skillSlug := t.Name
			if t.SkillSlug != "" {
				skillSlug = t.SkillSlug
			}
			activeSkillSlugs[skillSlug] = true
		}
	}
	systemPrompt, err := r.prompt.Build(ctx, PromptInput{
		AgentID:           in.AgentID,
		SessionID:         in.SessionID,
		SystemPrompt:      in.SystemPrompt,
		Memories:          memories,
		CoordinatorMode:   gates.CoordinatorMode,
		DeferredToolNames: llmToolNames(advertisedDeferredTools),
		UserOnlySkills:    toolResult.UserOnlySkills,
		ActiveSkillSlugs:  activeSkillSlugs,
		RequestContext:    in.RequestContext,
	})
	if err != nil {
		localEmitError("prompt_build", err)
		return
	}
	// CopilotKit Phase 2: append client-declared readables as <app_state>.
	// The block stays fresh because we refetch the snapshot every time the
	// prompt is (re)built — and the prompt is rebuilt after compaction.
	if in.FrontendActions != nil {
		systemPrompt += FormatAppStateBlock(in.FrontendActions.GetReadables(in.SessionID))
	}

	// 4. Load conversation history. The service persists the current user turn
	// before starting the runner; omit that exact row here and append it once
	// below after any runtime notes have been injected.
	messages, lastResponseID, err := r.loadHistory(ctx, in.SessionID, in.UserMessageID)
	if err != nil {
		localEmitError("load_history", err)
		return
	}

	// Track the index of the last message sent to the LLM, used to slice
	// messages when response chaining is active (previous_response_id).
	lastSentIndex := 0

	// 4b. Fire session_start hooks on the first run of a session.
	// Detected by empty history: no prior messages means this is the opening turn.
	// Sub-agents (CurrentDepth > 0) are not first-run sessions — skip them.
	// Prompt hook inject texts are prepended as system notes so the LLM sees them
	// in the first turn context.
	if len(messages) == 0 && in.CurrentDepth == 0 && r.toolExec != nil && r.toolExec.hookExecutor != nil {
		sessionStartResults := r.toolExec.hookExecutor.Execute(ctx, HookPayload{
			Event:     HookSessionStart,
			AgentID:   in.AgentID.String(),
			SessionID: in.SessionID.String(),
		})
		for _, res := range sessionStartResults {
			if res.Inject != "" {
				note := "[SYSTEM NOTE from session_start hook]\n" + res.Inject
				noteMsg := chat.ChatMessage{
					SessionID:   in.SessionID,
					Role:        "user",
					Content:     note,
					MessageType: chat.MessageTypeText,
					RunID:       &r.runID,
				}
				if r.runID == uuid.Nil {
					noteMsg.RunID = nil
				}
				if _, err := r.persister.CreateMessage(ctx, noteMsg); err != nil {
					slog.Warn("runner: failed to persist session_start hook note", "error", err)
				}
				messages = append(messages, ai.Message{Role: ai.RoleUser, Content: note})
			}
		}
	}

	// 4c. BUG-MCP-SILENT-FAIL fix: inject MCP failure warnings into the LLM context so
	// the model can proactively inform the user that bound MCP tools are unavailable,
	// rather than silently responding as if those tools never existed.
	// These are non-fatal — the run continues with whatever tools did load.
	if len(toolResult.Warnings) > 0 {
		var sb strings.Builder
		sb.WriteString("[SYSTEM NOTE: The following MCP servers bound to this agent could not be reached. Their tools are unavailable for this session.]\n")
		for _, w := range toolResult.Warnings {
			sb.WriteString("- ")
			sb.WriteString(w)
			sb.WriteString("\n")
		}
		mcpNote := sb.String()
		messages = append(messages, ai.Message{Role: ai.RoleUser, Content: mcpNote})
	}

	// 5. Append user message.
	messages = append(messages, ai.Message{
		Role:    ai.RoleUser,
		Content: in.UserMessage,
	})

	// Persist user message — only when not already persisted by the caller.
	// P-C178-2: chat.Service.RunSession persists the message upfront so it is
	// never lost if the run fails to initialise. When UserMessageID is set, the
	// message is already in the DB; skip to avoid a duplicate.
	if in.UserMessageID == nil {
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
			localEmitError("persist_user_msg", err)
			return
		}
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
	// emptyResponseRetries tracks how many consecutive turns returned neither text
	// nor tool calls. One retry is allowed (with a nudge); on the second empty
	// response we fall through and persist "(no content)" as-is.
	emptyResponseRetries := 0
	// duplicateToolCallStreak tracks consecutive turns that called the exact same
	// tool with the same arguments. When this exceeds the threshold the runner
	// injects a nudge instead of executing the duplicate call, to break loops.
	duplicateToolCallStreak := 0
	lastToolCallSignature := ""
	const maxDuplicateToolCallStreak = 3
	// lastTurnHadToolErrors tracks whether the immediately-preceding tool-execution
	// turn returned at least one error result. When true, any ask_user call in the
	// next turn is auto-answered with a synthetic "config error" response instead of
	// blocking for user input (P-G1 fix: prevents the LLM from delegating tool config
	// failures to the user via ask_user).
	lastTurnHadToolErrors := false
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
			ToolChoice:   resolveToolChoice(r.config.ToolMode, aiTools, r.chatModel.GetProviderName()),
		}
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
		// P-C102-1: wrap in a per-call timeout so a stalled provider never blocks forever.
		var callCtx context.Context
		var cancelCall context.CancelFunc
		if r.config.LLMCallTimeout > 0 {
			callCtx, cancelCall = context.WithTimeout(ctx, r.config.LLMCallTimeout)
		} else {
			callCtx, cancelCall = context.WithCancel(ctx) // no-op cancel for uniform cleanup
		}
		fallbackResult, err := retryStreamWithFallbackSource(callCtx, r.chatModel, messagesToSend, opts, r.config, turnSource,
			func(from, to string, fallbackErr error) {
				ch <- NewRunEvent(EventModelFallback, ModelFallbackData{
					FromModel: from,
					ToModel:   to,
					Reason:    fallbackErr.Error(),
				})
			},
		)
		if err != nil {
			cancelCall() // P-C102-1: release per-call timeout context on error
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
						localEmitError("compact_circuit_breaker", fmt.Errorf("compaction failed %d times consecutively", compactFailures))
						return
					}
					localEmitError("llm_call", err)
					return
				}
				// Verify compaction actually reduced tokens; escalate if needed.
				compactResult, compactErr = r.ctxManager.VerifyCompaction(ctx, compactResult, systemTokens, r.config, nil)
				if compactErr != nil {
					localEmitError("llm_call", err)
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

			localEmitError("llm_call", err)
			return
		}

		// Track which model was actually used for cost estimation.
		effectiveModel := fallbackResult.ModelUsed
		cacheSafeParams := NewCacheSafeParams(
			systemPrompt,
			aiTools,
			r.chatModel.GetProviderName(),
			effectiveModel,
			gates.CacheControl,
		)

		// 5b. Consume stream, accumulate response.
		// callCtx carries the per-call timeout so a stalled mid-stream provider
		// is also detected and aborted. cancelCall deferred until after consume.
		emitRawTextDeltas := len(in.OutputProcessors) == 0
		assistantContent, toolCalls, finishReason, usage, streamResponseID, streamErr := r.consumeStream(callCtx, ch, fallbackResult.Stream, emitRawTextDeltas, markFirstOutput)
		cancelCall() // P-C102-1: release per-call timeout context after streaming completes
		if streamErr != nil {
			localEmitError("stream_consume", streamErr)
			return
		}
		runTiming.StreamCompleteMS = time.Since(runStart).Milliseconds()
		if len(in.OutputProcessors) > 0 && assistantContent != "" {
			processedContent, err := processAssistantOutput(ctx, assistantContent, in.OutputProcessors)
			if err != nil {
				localEmitError("output_processor", err)
				return
			}
			assistantContent = processedContent
			markFirstOutput()
			ch <- NewRunEvent(EventTextDelta, TextDeltaData{Content: assistantContent})
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
		// Some providers (e.g. local Ollama models) return empty content on edge cases.
		// Allow one retry by injecting a nudge message; on the second empty response
		// fall through with "(no content)" so the turn is persisted and the run ends.
		if assistantContent == "" && len(toolCalls) == 0 && finishReason == "stop" {
			if emptyResponseRetries < 1 {
				emptyResponseRetries++
				slog.Warn("agentic: empty LLM response, retrying with nudge",
					"turn", turnIndex, "attempt", emptyResponseRetries)
				// Append a transient nudge — not persisted to the DB — to prompt the model
				// to produce a substantive response on the next iteration.
				messages = append(messages, ai.Message{
					Role:    ai.RoleUser,
					Content: "[SYSTEM] Your previous response was empty. Please respond to the user's request with text or by calling one of the available tools. If you need more information, ask the user a question.",
				})
				turnIndex++
				continue
			}
			// Second consecutive empty response — give up and store sentinel.
			slog.Warn("agentic: empty LLM response on retry, storing sentinel", "turn", turnIndex)
			assistantContent = "(no content)"
		} else {
			emptyResponseRetries = 0 // reset on any non-empty response
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
		if turnCost == 0 && gates.HasBudgetLimit && usage.TotalTokens > 0 {
			slog.Warn("agentic: cost estimation returned 0 for unknown model — budget limit may not enforce correctly",
				"model", effectiveModel, "tokens", usage.TotalTokens, "budget", effectiveBudget)
		}
		totalCost += turnCost

		// Update progress tracker.
		r.progress.SetTurnIndex(turnIndex)
		r.progress.RecordLLMCall(usage.TotalTokens, turnCost, r.config.Model)

		// Budget check.
		if gates.HasBudgetLimit && totalCost > effectiveBudget {
			localEmitError("budget_exceeded", fmt.Errorf("run cost $%.4f exceeded budget $%.4f", totalCost, effectiveBudget))
			return
		}

		// 5c. Build and persist assistant message.
		assistantMsg := r.buildAssistantMessage(in.SessionID, assistantContent, toolCalls, finishReason, usage, turnIndex, streamResponseID)
		persistStarted := time.Now()
		if _, err := r.persister.CreateMessage(ctx, assistantMsg); err != nil {
			emitError(ch, "persist_assistant_msg", err)
			return
		}
		runTiming.AssistantPersistMS += time.Since(persistStarted).Milliseconds()

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
		runFinishReason = finishReason // P-C325-2: track last finish reason for metadata
		switch finishReason {
		case "stop":
			// LLM produced a final response — reset the P-G1 error guard.
			lastTurnHadToolErrors = false
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
			postTurnLifecycleStarted := time.Now()
			// BUG-HOOK-TURNEND-INJECT: surface inject texts from turn-end hooks.
			if turnEndInjects := r.executeTurnEndHooks(ctx, ch, buildTurnEndPayload()); len(turnEndInjects) > 0 {
				injectNote := "[SYSTEM NOTE from hook]\n" + strings.Join(turnEndInjects, "\n---\n")
				hookNoteMsg := chat.ChatMessage{
					SessionID:   in.SessionID,
					Role:        "user",
					Content:     injectNote,
					MessageType: chat.MessageTypeText,
					RunID:       &r.runID,
				}
				if r.runID == uuid.Nil {
					hookNoteMsg.RunID = nil
				}
				if _, err := r.persister.CreateMessage(ctx, hookNoteMsg); err != nil {
					slog.Warn("runner: failed to persist turn-end hook inject note", "error", err)
				}
				messages = append(messages, ai.Message{Role: ai.RoleUser, Content: injectNote})
			}
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
			runTiming.PostTurnLifecycleMS = time.Since(postTurnLifecycleStarted).Milliseconds()

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
				Timing:                        runTimingSnapshot(),
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

			// Duplicate tool-call loop detection: if the LLM keeps calling the same
			// tool with identical arguments, it is stuck and unlikely to self-correct.
			// After maxDuplicateToolCallStreak identical consecutive calls, inject a
			// nudge message (without executing the duplicate) to break the cycle.
			callSig := computeToolCallSignature(toolCalls)
			if callSig == lastToolCallSignature {
				duplicateToolCallStreak++
			} else {
				duplicateToolCallStreak = 0
				lastToolCallSignature = callSig
			}
			// When the previous turn produced tool errors, allow at most one retry of
			// the same tool signature — re-calling an already-failed tool immediately
			// is a transient-error retry loop (P-C71-1 fix).
			effectiveStreakThreshold := maxDuplicateToolCallStreak
			if lastTurnHadToolErrors {
				effectiveStreakThreshold = 1
			}
			if duplicateToolCallStreak >= effectiveStreakThreshold {
				slog.Warn("agentic: duplicate tool-call loop detected, injecting break nudge",
					"turn", turnIndex, "streak", duplicateToolCallStreak, "signature", callSig,
					"prevTurnHadErrors", lastTurnHadToolErrors)
				// NOTE: the assistant message was already persisted and appended to messages
				// at lines 761-773 (the main flow above). Do NOT re-persist or re-append here —
				// that would produce duplicate DB rows and a doubled/tripled LLM context window.
				// Only persist and append the synthetic nudge tool_result.
				var nudgeContent string
				if lastTurnHadToolErrors {
					// P-F2-1: when the tool has been consistently failing, do NOT say
					// "data is already in your context" — that invites the LLM to fabricate.
					// Instead, instruct it to acknowledge the failure honestly.
					nudgeContent = fmt.Sprintf("[SYSTEM] You have called the same tool (%s) %d times and it has failed every time. Do NOT fabricate or invent any result. Tell the user that the operation could not be completed due to a recurring tool error, and do not claim otherwise.",
						tcNames[0], duplicateToolCallStreak+1)
				} else if tcNames[0] == "memory_store" {
					// BUG-MEM-STRESS1: for memory_store the generic "stop calling tools"
					// nudge causes the LLM to fabricate bulk success. Instead, tell it the
					// specific fact is stored and to advance to the next item in the list.
					nudgeContent = fmt.Sprintf("[SYSTEM] This fact is already stored in memory (%d identical calls). Do NOT call memory_store again for this exact fact. If the user gave you a list, call memory_store NOW with the NEXT fact from the list. If all facts have been stored, provide a summary of what was saved.",
						duplicateToolCallStreak+1)
				} else if tcNames[0] == "memory_store_bulk" {
					// BUG-MEM-STRESS1: bulk variant already stored all facts in the first call.
					nudgeContent = fmt.Sprintf("[SYSTEM] All facts were already stored by the first memory_store_bulk call (%d identical calls detected). Do NOT call memory_store_bulk again. Provide your final summary response to the user now.",
						duplicateToolCallStreak+1)
				} else {
					nudgeContent = fmt.Sprintf("[SYSTEM] You have called the same tool (%s) with identical arguments %d times. The data is already in your context. Please stop calling tools and provide a final summary response to the user now.",
						tcNames[0], duplicateToolCallStreak+1)
				}
				// P-C216-1: persist a nudge tool_result for EVERY tool_call in the batch.
				// When the LLM calls N tools in parallel and the repetition guard fires,
				// previously only toolCalls[0] got a result, leaving the other tool_call_ids
				// orphaned. OpenAI then rejects the entire session history with HTTP 400
				// ("tool_call_id did not have response messages"), permanently corrupting
				// the session. Fix: emit a nudge result for each tool_call in the turn.
				for _, tc := range toolCalls {
					tcID := tc.ID // capture loop variable
					nudgeResult := chat.ChatMessage{
						SessionID:   in.SessionID,
						Role:        "tool",
						Content:     nudgeContent,
						MessageType: chat.MessageTypeToolResult,
						ToolCallID:  &tcID,
						RunID:       &r.runID,
					}
					if _, err := r.persister.CreateMessage(ctx, nudgeResult); err != nil {
						emitError(ch, "persist_loop_nudge", err)
						return
					}
					messages = append(messages, ai.Message{
						Role:       ai.RoleTool,
						Content:    nudgeContent,
						ToolCallID: tc.ID,
					})
				}
				duplicateToolCallStreak = 0 // reset after nudge
				ch <- NewRunEvent(EventTurnComplete, TurnCompleteData{TurnIndex: turnIndex, Source: "loop_break"})
				turnIndex++
				continue
			}

			toolResults := r.executeWithPermissions(ctx, ch, toolCalls, in, totalCost, readOnlyIndex, inputSchemaIndex, destructiveIndex, deferredTools, allowedToolsIndex, contextModeTools, searchOrReadIndex, lastTurnHadToolErrors, rs)
			slog.Info("agentic: tool execution completed", "turn", turnIndex, "resultCount", len(toolResults), "ctxErr", ctx.Err())

			// Check if denial tracking indicates a stuck loop.
			if r.denialTracker != nil && len(r.denialTracker.EscalationHints()) > 0 {
				totalDenials := r.denialTracker.TotalDenials()
				emitError(ch, "denial_escalation", fmt.Errorf(
					"too many tool denials (%d total) — LLM appears stuck in a permission loop",
					totalDenials))
				return
			}

			// P-C325-2: count tool calls and detect failures for run metadata.
			// P-C336-1 (ACT-F3-13): aggregate sub-agent token/cost into parent run totals.
			runToolCallCount += len(toolResults)
			for _, r2 := range toolResults {
				if r2.Error != nil {
					runHadToolFailures = true
				}
				// Roll up sub-agent resource usage so RunComplete and RunMetadata
				// reflect the full cost of the run including nested agents.
				if r2.SubtaskTokens > 0 {
					totalTokens += r2.SubtaskTokens
				}
				if r2.SubtaskCostUSD > 0 {
					totalCost += r2.SubtaskCostUSD
				}
			}

			// Persist and append tool results to history.
			persistCtx, cancelPersist := contextForToolResultPersistence(ctx)
			defer cancelPersist()
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

				// P-C84-1: sanitize error before sending to LLM, same as for SSE (P-C65-2).
				sanitizedResult := result
				sanitizedResult.Error = sanitizeToolErrorPtr(result.Error)
				// SECRET-SCANNER: redact known credential patterns from tool output before
				// sending to the LLM. Prevents static authToken values from leaking into
				// the conversation when remote endpoints echo back Authorization headers.
				sanitizedResult.Output = redactSensitiveToolResultOutput(sanitizedResult.Output)
				resultContent := FormatToolResult(sanitizedResult)
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
				if _, err := r.persister.CreateMessage(persistCtx, toolMsg); err != nil {
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
						Output:     serializableJSONRawMessage(sanitizedResult.Output),
						DurationMs: result.LatencyMs,
						Error:      sanitizeToolErrorPtr(result.Error), // P-C65-2: scrub internal infra details
					})
				}

				// Track tool execution in progress.
				r.progress.RecordToolCall(toolName)
			}

			// Update P-G1 guard: track whether this turn had any tool errors.
			// The next turn's ask_user calls will be auto-answered if this is true.
			lastTurnHadToolErrors = false
			for _, res := range toolResults {
				if res.Error != nil {
					lastTurnHadToolErrors = true
					break
				}
			}

			// BUG-HOOK-PROMPT-INJECT fix: collect inject text from post-tool hooks
			// and inject as a [SYSTEM NOTE] user message before the next LLM call.
			// Injecting inside the tool result caused the LLM to retry the tool in
			// response to the annotation text. A separate user message keeps the
			// annotation in the conversation context without appearing as tool output.
			var hookInjectParts []string
			for _, res := range toolResults {
				if res.InjectText != "" {
					hookInjectParts = append(hookInjectParts, res.InjectText)
				}
			}
			if len(hookInjectParts) > 0 {
				injectNote := "[SYSTEM NOTE from hook]\n" + strings.Join(hookInjectParts, "\n---\n")
				hookNoteMsg := chat.ChatMessage{
					SessionID:   in.SessionID,
					Role:        "user",
					Content:     injectNote,
					MessageType: chat.MessageTypeText,
					RunID:       &r.runID,
				}
				if r.runID == uuid.Nil {
					hookNoteMsg.RunID = nil
				}
				if _, err := r.persister.CreateMessage(ctx, hookNoteMsg); err != nil {
					slog.Warn("runner: failed to persist hook inject note", "error", err)
				}
				messages = append(messages, ai.Message{
					Role:    ai.RoleUser,
					Content: injectNote,
				})
			}

			if summary := r.buildToolUseSummary(ctx, in.AgentID, toolCalls, toolResults, assistantContent); summary != "" {
				ch <- NewRunEvent(EventToolUseSummary, ToolUseSummaryData{
					TurnIndex: turnIndex,
					Summary:   summary,
				})
			}

			// Turn-end hooks (after tool results, before incrementing turnIndex).
			// BUG-HOOK-TURNEND-INJECT: surface inject texts; will be visible to LLM on next turn.
			if turnEndInjects := r.executeTurnEndHooks(ctx, ch, buildTurnEndPayload()); len(turnEndInjects) > 0 {
				injectNote := "[SYSTEM NOTE from hook]\n" + strings.Join(turnEndInjects, "\n---\n")
				hookNoteMsg := chat.ChatMessage{
					SessionID:   in.SessionID,
					Role:        "user",
					Content:     injectNote,
					MessageType: chat.MessageTypeText,
					RunID:       &r.runID,
				}
				if r.runID == uuid.Nil {
					hookNoteMsg.RunID = nil
				}
				if _, err := r.persister.CreateMessage(ctx, hookNoteMsg); err != nil {
					slog.Warn("runner: failed to persist turn-end hook inject note", "error", err)
				}
				messages = append(messages, ai.Message{Role: ai.RoleUser, Content: injectNote})
			}
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

					// Clear the session-scoped prompt cache so stable sections (tools, KBs)
					// are recomputed. After compaction the old cached values may reference
					// context that was summarized away.
					// Inspired by Claude Code's clearSystemPromptSections on /compact.
					r.promptCache = make(map[string]string)

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
							DeferredToolNames: llmToolNames(advertisedDeferredTools),
							UserOnlySkills:    toolResult.UserOnlySkills,
							ActiveSkillSlugs:  activeSkillSlugs,
							RequestContext:    in.RequestContext,
						}); err == nil {
							systemPrompt = rebuilt
							// CopilotKit Phase 2: re-append <app_state> after compaction
							// so the agent keeps seeing the latest client readables.
							if in.FrontendActions != nil {
								systemPrompt += FormatAppStateBlock(in.FrontendActions.GetReadables(in.SessionID))
							}
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
			// BUG-HOOK-TURNEND-INJECT: surface inject texts.
			postTurnLifecycleStarted := time.Now()
			if turnEndInjects := r.executeTurnEndHooks(ctx, ch, buildTurnEndPayload()); len(turnEndInjects) > 0 {
				injectNote := "[SYSTEM NOTE from hook]\n" + strings.Join(turnEndInjects, "\n---\n")
				hookNoteMsg := chat.ChatMessage{
					SessionID:   in.SessionID,
					Role:        "user",
					Content:     injectNote,
					MessageType: chat.MessageTypeText,
					RunID:       &r.runID,
				}
				if r.runID == uuid.Nil {
					hookNoteMsg.RunID = nil
				}
				if _, err := r.persister.CreateMessage(ctx, hookNoteMsg); err != nil {
					slog.Warn("runner: failed to persist turn-end hook inject note", "error", err)
				}
				messages = append(messages, ai.Message{Role: ai.RoleUser, Content: injectNote})
			}
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
			runTiming.PostTurnLifecycleMS = time.Since(postTurnLifecycleStarted).Milliseconds()

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
				Timing:                        runTimingSnapshot(),
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
	localEmitError("max_iterations", fmt.Errorf("agentic loop exceeded maximum iterations (%d)", r.config.MaxIterations))
}

// consumeStream reads all chunks from the stream channel and accumulates the response.
func (r *Runner) consumeStream(ctx context.Context, ch chan<- RunEvent, stream <-chan ai.StreamChunk, emitTextDeltas bool, markFirstOutput func()) (
	content string, toolCalls []ai.ToolCall, finishReason string, usage ai.Usage, responseID string, err error,
) {
	// Track tool calls being built incrementally. Some providers emit
	// repeated deltas for the same tool call ID while arguments stream in.
	toolCallsByID := map[string]*ai.ToolCall{}
	toolCallOrder := make([]string, 0)
	var lastToolCall *ai.ToolCall

	for {
		var chunk ai.StreamChunk
		var ok bool
		select {
		case <-ctx.Done():
			// P-C102-1: per-call timeout expired while waiting for a stream chunk.
			return "", nil, "", ai.Usage{}, "", ctx.Err()
		case chunk, ok = <-stream:
		}
		if !ok {
			break // stream closed normally
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
			markFirstOutput()
			ch <- NewRunEvent(EventThinkingDelta, ThinkingDeltaData{Content: chunk.ThinkingDelta})
		}

		if chunk.Delta != "" {
			content += chunk.Delta
			if emitTextDeltas {
				markFirstOutput()
				ch <- NewRunEvent(EventTextDelta, TextDeltaData{Content: chunk.Delta})
			}
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

	// Fallback: some models (e.g. openrouter/openai/gpt-oss-120b) emit tool
	// calls as raw JSON text instead of populating the provider's structured
	// tool_calls channel. When the assistant content is *only* such a payload
	// and no structured calls arrived, parse it here so the loop can execute
	// the requested tool instead of surfacing raw JSON to the user.
	if len(toolCalls) == 0 && content != "" {
		if parsed, consumed := parseTextToolCalls(content); consumed {
			// Log as structured event so observability pipelines can track
			// fallback frequency per agent/model and flag agents that should
			// migrate to modelConfig.toolMode="required" (issue #184).
			// Name of the first tool is included to help identify patterns
			// (e.g. a particular management skill that always triggers text).
			firstToolName := ""
			if len(parsed) > 0 {
				firstToolName = parsed[0].Function.Name
			}
			slog.Info("agentic: text-format tool call intercepted by fallback",
				"event", "text_toolcall_fallback",
				"toolName", firstToolName,
				"toolCount", len(parsed),
				"contentLen", len(content))
			toolCalls = parsed
			content = ""
			finishReason = "tool_calls"
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

func processAssistantOutput(ctx context.Context, content string, outputProcessors []string) (string, error) {
	if content == "" || len(outputProcessors) == 0 {
		return content, nil
	}
	pipeline, err := processors.BuildPipeline(nil, outputProcessors)
	if err != nil {
		return "", fmt.Errorf("runner: build output processors: %w", err)
	}
	processed, err := pipeline.RunOutput(ctx, content)
	if err != nil {
		return "", fmt.Errorf("runner: run output processors: %w", err)
	}
	return processed, nil
}

// loadHistory loads messages from the database and converts to ai.Message format.
// Applies time-based tool result eviction when the session has been idle longer
// than the cache TTL (inspired by Claude Code's microCompact.ts cold-cache trigger).
// Returns the messages and the response_id from the last assistant message's metadata
// (for response chaining with providers that support it).
// omitMessageID may identify the current user row that was pre-persisted by
// chat.Service. The Runner appends that turn at the correct position itself.
func (r *Runner) loadHistory(ctx context.Context, sessionID uuid.UUID, omitMessageID ...*uuid.UUID) ([]ai.Message, string, error) {
	if r.history == nil {
		return nil, "", nil
	}

	chatMsgs, err := r.history.FindAllMessages(ctx, sessionID)
	if err != nil {
		return nil, "", fmt.Errorf("runner: load history: %w", err)
	}

	// DX-01-M (ACT-F3-06): sliding-window cap — keep only the most recent N messages
	// to prevent enormous prompts in long-running sessions.
	if r.config.MaxHistoryMessages > 0 && len(chatMsgs) > r.config.MaxHistoryMessages {
		chatMsgs = chatMsgs[len(chatMsgs)-r.config.MaxHistoryMessages:]
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

	var omittedID uuid.UUID
	if len(omitMessageID) > 0 && omitMessageID[0] != nil {
		omittedID = *omitMessageID[0]
	}

	var messages []ai.Message
	for _, m := range chatMsgs {
		if omittedID != uuid.Nil && m.ID == omittedID {
			continue
		}
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

		// Filter 3: consecutive assistant messages — collapse to the last one.
		// P-C99-2: caused by concurrent runs both completing and persisting an
		// assistant reply. Most LLM providers reject adjacent assistant turns.
		// We keep the last assistant message in each run so the context stays valid.
		if m.Role == ai.RoleAssistant && len(result) > 0 && result[len(result)-1].Role == ai.RoleAssistant &&
			len(result[len(result)-1].ToolCalls) == 0 && len(m.ToolCalls) == 0 {
			result[len(result)-1] = m
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
			execResult := r.managementExec.Execute(ctx, args.Operation, args.Resource, args.ID, args.Query, args.Payload, in.AgentID)
			execResult.LatencyMs = time.Since(start).Milliseconds()

			ch <- NewRunEvent(EventToolProgress, ToolProgressData{ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted})
			ch <- NewRunEvent(EventToolResult, ToolResultData{
				ID:         tc.ID,
				Name:       tc.Function.Name,
				Output:     serializableJSONRawMessage(redactSensitiveToolResultOutput(execResult.Output)),
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
		rawMsg := err.Error()
		safeMsg := sanitizeToolError(rawMsg) // P-C65-2: scrub internal infra details before emitting
		res := ToolExecResult{Error: &safeMsg, LatencyMs: latency, EmittedToStream: true}
		ch <- NewRunEvent(EventToolProgress, ToolProgressData{ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted})
		ch <- NewRunEvent(EventToolResult, ToolResultData{ID: tc.ID, Name: tc.Function.Name, Output: nil, Error: &safeMsg, DurationMs: latency})
		return res
	}

	execResult.LatencyMs = latency
	execResult.EmittedToStream = true
	ch <- NewRunEvent(EventToolProgress, ToolProgressData{ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted})
	ch <- NewRunEvent(EventToolResult, ToolResultData{
		ID:         tc.ID,
		Name:       tc.Function.Name,
		Output:     serializableJSONRawMessage(redactSensitiveToolResultOutput(execResult.Output)),
		DurationMs: latency,
	})
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

// filterLLMToolsByPermission applies the pool-time permission decision to the
// schemas exposed to the model. Argument-specific deny rules remain visible:
// without a concrete tool call, their sample input is intentionally empty and
// the call-time check remains the authoritative enforcement point.
func filterLLMToolsByPermission(tools []LLMTool, rules *PermissionRules) []LLMTool {
	if rules == nil {
		return tools
	}

	prefilter, err := NewPermissionPreFilter(PrefilterConfig{Rules: rules}, nil)
	if err != nil {
		return nil
	}

	filtered := make([]LLMTool, 0, len(tools))
	for _, tool := range tools {
		source := ToolSourceSkill
		if tool.Builtin {
			source = ToolSourceBuiltin
		}
		if prefilter.Decide(ToolPoolEntry{Name: tool.Name, Source: source}) == PermissionDeny {
			continue
		}
		filtered = append(filtered, tool)
	}
	return filtered
}

func llmToolNames(tools []LLMTool) []string {
	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	return names
}

// FormatToolResult produces a string representation of a tool execution result.
// Empty results get a descriptive message instead of "{}" because some models
// interpret empty tool_result content as a stop signal.
// Inspired by Claude Code's toolResultStorage.ts empty result injection.
func FormatToolResult(r ToolExecResult) string {
	if r.Error != nil {
		// P-F3-1 (BUG-F3): append a direct instruction so the LLM does not call ask_user
		// after a server-side failure. The P-G1 guard in executeWithPermissions already
		// intercepts ask_user on the next turn, but this hint prevents the extra round-trip
		// by steering the model at the moment the error is delivered.
		// BUG-ASK_USER-2: also explicitly forbid outputting JSON tool call formats in plain
		// text — some models leak tool_call arguments into their text response when confused.
		return fmt.Sprintf("Error: %s\n\n[SYSTEM] If this failure is server-side (configuration, network, or internal error), do NOT call ask_user and do NOT output JSON tool call formats in your text response — respond directly to the user in plain natural language about what went wrong.", *r.Error)
	}
	if len(r.Output) > 0 {
		// P-C176-1: strip status_code / statusCode from HTTP tool output before
		// sending to LLM. HTTP status codes are transport metadata; the LLM should
		// reason about the response body, not the protocol-level code.
		sanitised := stripStatusCodeFromToolOutput(r.Output)
		s := string(sanitised)
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
// When truncated, appends a structured marker so the LLM knows the data is
// incomplete and can inform the user. P-C125-1, P-C150-2 (ACT-F2-27).
func truncateToolResult(result ToolExecResult, maxChars int) ToolExecResult {
	if maxChars <= 0 || len(result.Output) <= maxChars {
		return result
	}
	originalLen := len(result.Output)
	note := fmt.Sprintf("\n[TRUNCATED: showing first %d of %d chars — full result available on request]", maxChars, originalLen)
	shown := maxChars - len(note)
	if shown < 0 {
		shown = 0
	}
	result.Output = append(result.Output[:shown], []byte(note)...)
	return result
}

func forkModeToolCall(tc ai.ToolCall, tool LLMTool) ai.ToolCall {
	args, _ := json.Marshal(SubtaskInput{
		Prompt: buildForkModeSkillPrompt(tc, tool),
		Tools:  tool.AllowedTools,
	})
	return ai.ToolCall{
		ID:   tc.ID,
		Type: tc.Type,
		Function: ai.ToolFunction{
			Name:      agentToolName,
			Arguments: string(args),
		},
	}
}

func buildForkModeSkillPrompt(tc ai.ToolCall, tool LLMTool) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Run the fork-mode skill %q in an isolated sub-agent.\n\n", tool.Name)
	sb.WriteString("Original tool input:\n")
	input := strings.TrimSpace(tc.Function.Arguments)
	if input == "" {
		input = "{}"
	}
	sb.WriteString(input)
	sb.WriteString("\n\n")
	if tool.Description != "" {
		sb.WriteString("Skill description:\n")
		sb.WriteString(tool.Description)
		sb.WriteString("\n\n")
	}
	if len(tool.AllowedTools) > 0 {
		sb.WriteString("Allowed tools for this skill:\n")
		sb.WriteString(strings.Join(tool.AllowedTools, ", "))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// executeWithPermissions evaluates permission rules for each tool call, executes
// permitted ones via StreamingToolExecutor, and returns results in the same order
// as the input toolCalls. Denied/confirm tools get error results without execution.
// Agent tool calls are routed to the SubtaskExecutor for sub-agent spawning.
// prevTurnHadToolErrors indicates whether the immediately-preceding tool turn
// returned at least one error. When true, any ask_user call is auto-answered
// with a synthetic config-error response instead of blocking for user input (P-G1).
func (r *Runner) executeWithPermissions(ctx context.Context, ch chan<- RunEvent, toolCalls []ai.ToolCall, in RunInput, totalCost float64, readOnlyIndex map[string]bool, inputSchemaIndex map[string]json.RawMessage, destructiveIndex map[string]bool, deferredTools []LLMTool, allowedToolsIndex map[string]bool, contextModeTools map[string]LLMTool, searchOrReadIndex map[string]bool, prevTurnHadToolErrors bool, rs *runState) []ToolExecResult {
	results := make([]ToolExecResult, len(toolCalls))

	// Partition tool calls into categories.
	var regularTools []ai.ToolCall
	regularIdx := map[int]int{} // original index → regular index
	var agentTools []ai.ToolCall
	agentIdx := map[int]int{} // original index → agent index

	for i, tc := range toolCalls {
		// P-C57-3: enforce per-run retry limit before any other checks so that
		// exhausted tools never reach permission evaluation or execution.
		// BUG-MEM-STRESS1 fix: memory_store is exempt — it has per-key dedup in
		// storedMemoryKeys, so the global retry counter would prematurely block
		// legitimate bulk memorisation (e.g. "store these 22 facts").
		if tc.Function.Name == "memory_store" || tc.Function.Name == "memory_store_bulk" {
			// count is still tracked for observability but never blocks execution.
			rs.toolRetries[tc.Function.Name]++
		} else if retryErr := rs.checkAndIncrementRetry(tc.Function.Name); retryErr != nil {
			errMsg := retryErr.Error()
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

		// Reject tool calls for tools not in the allowed set for this run.
		// This prevents the LLM from calling tools it remembers from conversation
		// history that are no longer bound to the agent (P-SK6).
		if len(allowedToolsIndex) > 0 && !allowedToolsIndex[tc.Function.Name] {
			// BUG-SUB-AGENT-1 fix: include an explicit directive so the LLM does not
			// keep retrying the same blocked tool on subsequent turns. Without this,
			// models may call the same unavailable tool 3× (maxToolRetries) before
			// giving a direct answer, wasting latency and tokens.
			errMsg := fmt.Sprintf("Tool '%s' is not available for this agent. Do NOT call it again — provide a direct answer instead.", tc.Function.Name)
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

		// CopilotKit Phase 1: client-declared frontend actions. Emit the call event,
		// block until the client posts the result back via POST /client-state, then
		// surface the result as a normal tool_result. No permission evaluation —
		// the action is owned and authorised by the client app itself.
		if rs.frontendActionsByName[tc.Function.Name] && in.FrontendActions != nil {
			ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
				ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(tc.Function.Arguments),
			})
			ch <- NewRunEvent(EventFrontendActionCall, FrontendActionCallData{
				ID:        tc.ID,
				Name:      tc.Function.Name,
				Arguments: json.RawMessage(tc.Function.Arguments),
			})
			// Mirror ask_user: use a long-lived context (24h) so the run-level
			// processing timeout does not expire while the user is interacting.
			actionCtx, actionCancel := context.WithTimeout(context.Background(), 24*time.Hour)
			actionResult := in.FrontendActions.Submit(actionCtx, in.SessionID, tc.ID, tc.Function.Name, json.RawMessage(tc.Function.Arguments))
			actionCancel()
			if actionResult.Status == "error" {
				errMsg := actionResult.Error
				if errMsg == "" {
					errMsg = "frontend action failed"
				}
				results[i] = ToolExecResult{Error: &errMsg, ToolName: tc.Function.Name}
				ch <- NewRunEvent(EventToolResult, ToolResultData{
					ID: tc.ID, Name: tc.Function.Name, Error: &errMsg,
				})
				continue
			}
			output := actionResult.Result
			if len(output) == 0 {
				output = json.RawMessage(`null`)
			}
			output = redactSensitiveToolResultOutput(output)
			results[i] = ToolExecResult{Output: output, ToolName: tc.Function.Name}
			ch <- NewRunEvent(EventToolResult, ToolResultData{
				ID: tc.ID, Name: tc.Function.Name, Output: serializableJSONRawMessage(output),
			})
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted,
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
				r.logPermissionDecision(ctx, in, tc.Function.Name, tc.Function.Arguments, AuditDecisionDeny)
				ch <- NewRunEvent(EventToolDenied, ToolDeniedData{
					ID:          tc.ID,
					Name:        tc.Function.Name,
					Reason:      errMsg,
					DenialCount: denialCount,
					Escalated:   escalated,
				})
				continue

			case PermissionConfirm:
				// When an ElicitationSubmitter is wired, ask the user in real time.
				// When not wired (automated mode), escalate and treat as deny.
				if in.Elicitation != nil {
					approved := r.askConsentViaElicitation(ctx, in, tc.Function.Name, tc.Function.Arguments)
					if approved {
						r.logPermissionDecision(ctx, in, tc.Function.Name, tc.Function.Arguments, AuditDecisionConfirmApproved)
						// Fall through — tool is allowed.
					} else {
						errMsg := fmt.Sprintf("Tool '%s' was not approved by the user.", tc.Function.Name)
						results[i] = ToolExecResult{Error: &errMsg}
						r.logPermissionDecision(ctx, in, tc.Function.Name, tc.Function.Arguments, AuditDecisionConfirmDenied)
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
				} else {
					errMsg := fmt.Sprintf("Tool '%s' requires confirmation but running in automated mode.", tc.Function.Name)
					results[i] = ToolExecResult{Error: &errMsg}
					r.logPermissionDecision(ctx, in, tc.Function.Name, tc.Function.Arguments, AuditDecisionConfirmEscalated)
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
		}

		// Auto-require confirmation for destructive tools (delete, drop, overwrite)
		// even when permission rules would allow them. This is a safety net inspired
		// by Claude Code's isDestructive per-tool flag (Tool.ts).
		//
		// Meta-tools that multiplex CRUD operations via an "operation" argument
		// (e.g. agent-management) are flagged destructive because some of their
		// operations mutate state. For calls whose arguments explicitly request
		// a known read-only operation (list/get/show/...), fall through so the
		// LLM is not forced to keep retrying with narrower tools.
		if destructiveIndex[tc.Function.Name] && !isReadOnlyOperation(tc.Function.Arguments) {
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

		if contextTool, ok := contextModeTools[tc.Function.Name]; ok {
			if contextTool.ContextMode != "fork" {
				errMsg := fmt.Sprintf("Tool '%s' has unsupported contextMode %q.", tc.Function.Name, contextTool.ContextMode)
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
			if r.subtaskExec == nil {
				errMsg := fmt.Sprintf("Tool '%s' requires fork context but sub-agent execution is not available.", tc.Function.Name)
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
			agentIdx[i] = len(agentTools)
			agentTools = append(agentTools, forkModeToolCall(tc, contextTool))
			continue
		}

		// Route ask_user calls to the ElicitationHandler — block until the user
		// submits a response via POST /elicitation/{requestId}/respond.
		// P-G1 guard: if the previous turn had tool errors, auto-answer ask_user with
		// a synthetic "config error" response instead of blocking for user input.
		// This prevents the LLM from delegating backend configuration failures to the
		// user (e.g., asking for datasource_id when the tool is simply misconfigured).
		if tc.Function.Name == "ask_user" && prevTurnHadToolErrors {
			slog.Warn("agentic: ask_user intercepted after tool error turn — auto-answering to prevent P-G1 block", "toolCallID", tc.ID)
			ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
				ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(tc.Function.Arguments),
			})
			autoReply := `{"action":"cancel","reason":"The previous tool call failed due to a server-side configuration error. This is not information the user can provide — please explain the tool limitation directly to the user without asking for configuration details."}`
			results[i] = ToolExecResult{Output: json.RawMessage(autoReply), ToolName: tc.Function.Name}
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted,
			})
			continue
		}
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

			// BUG-ASK_USER-LOOP: auto-answer if all requested question IDs were already
			// collected via a prior accepted ask_user call this run. This prevents the
			// infinite-loop pattern where the LLM calls ask_user repeatedly even after
			// receiving the user's values (observed with gpt-oss-120b).
			if len(rs.collectedUserValues) > 0 && len(params.Questions) > 0 {
				autoValues := make(map[string]any)
				allCovered := true
				for _, q := range params.Questions {
					if val, ok := rs.collectedUserValues[q.ID]; ok {
						autoValues[q.ID] = val
					} else if q.Required == nil || *q.Required {
						allCovered = false
						break
					}
				}
				if allCovered && len(autoValues) > 0 {
					slog.Warn("agentic: ask_user auto-answered from run-state cache (BUG-ASK_USER-LOOP)", "toolCallID", tc.ID, "keys", mapKeys(autoValues))
					ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
						ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(tc.Function.Arguments),
					})
					wrapped := map[string]any{
						"values":       autoValues,
						"_instruction": "Values were already provided by the user earlier in this session. Proceed immediately with the pending task using these values.",
					}
					out, _ := json.Marshal(wrapped)
					results[i] = ToolExecResult{Output: out, ToolName: tc.Function.Name}
					ch <- NewRunEvent(EventToolProgress, ToolProgressData{
						ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted,
					})
					continue
				}
			}
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
			// P-C131-3 (ACT-F3-01): use a separate 24h timeout while waiting for user
			// input so that the run's 15-min processing timeout doesn't expire during
			// elicitation. The parent context is still checked for cancellation.
			elicitCtx, elicitCancel := context.WithTimeout(context.Background(), 24*time.Hour)
			result := in.Elicitation.Submit(elicitCtx, "", tc.ID, elicParams)
			elicitCancel()
			slog.Info("agentic: ask_user Submit returned", "toolCallID", tc.ID, "action", result.Action, "contentKeys", mapKeys(result.Content), "ctxErr", ctx.Err())
			var output json.RawMessage
			if result.Action == ElicitationCancel || result.Action == ElicitationDecline {
				// P-E2-1: Include explicit instruction so the LLM stops retrying.
				// Returning only {"action":"decline"} causes the LLM to re-ask repeatedly.
				msg := fmt.Sprintf(
					`{"action": %q, "message": "The user explicitly declined or cancelled this request. Do NOT ask again. Acknowledge that the action has been cancelled and offer no further prompts for this task."}`,
					result.Action,
				)
				output = json.RawMessage(msg)
			} else {
				// Accumulate accepted values in run-state so subsequent ask_user calls
				// for the same fields can be auto-answered (BUG-ASK_USER-LOOP fix).
				for k, v := range result.Content {
					rs.collectedUserValues[k] = v
				}
				// BUG-ASK_USER-LOOP: wrap the user-provided values with an explicit
				// _instruction field so the LLM knows to proceed immediately with the
				// pending task rather than calling ask_user again. Without this hint
				// models like gpt-oss-120b loop back to ask_user on the next turn.
				wrapped := map[string]any{
					"values":       result.Content,
					"_instruction": "User provided the requested values. Use them immediately to proceed with the pending task. Do NOT call ask_user again for the same information.",
				}
				out, _ := json.Marshal(wrapped)
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

		// Builtin: memory_store — persists a memory entry for long-term recall.
		if tc.Function.Name == "memory_store" {
			var args struct {
				Content  string `json:"content"`
				Category string `json:"category"`
			}
			_ = json.Unmarshal(json.RawMessage(tc.Function.Arguments), &args)
			ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
				ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(tc.Function.Arguments),
			})
			var memResult string
			if r.memory != nil {
				// BUG-MEM-STRESS1 fix: track per-key instead of a single boolean so
				// the LLM can store multiple distinct facts in one run (bulk memorisation).
				// Only block a second call for the EXACT same normalised key; different
				// content keys proceed normally.
				key := normalizeKey(args.Content)
				if _, alreadyStored := rs.storedMemoryKeys[key]; alreadyStored {
					memResult = "Memory already stored for this item — no action needed. Continue with the next fact."
				} else {
					memResult = r.memory.Store(ctx, in.AgentID, args.Content, args.Category)
					if strings.HasPrefix(memResult, "Memory stored successfully") {
						rs.storedMemoryKeys[key] = struct{}{}
					}
				}
			} else {
				memResult = "Memory storage is not available for this agent."
			}
			out, _ := json.Marshal(memResult)
			results[i] = ToolExecResult{Output: out, ToolName: tc.Function.Name}
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted,
			})
			// Post-tool hooks for memory_store builtin.
			// BUG-HOOK-BUILTIN: capture hookResults to populate InjectText so the runner
			// can emit [SYSTEM NOTE] messages. Previously the return value was discarded.
			if r.toolExec != nil && r.toolExec.hookExecutor != nil {
				hookResults := r.toolExec.hookExecutor.Execute(ctx, HookPayload{
					Event:      HookPostToolUse,
					AgentID:    in.AgentID.String(),
					SessionID:  in.SessionID.String(),
					ToolName:   tc.Function.Name,
					ToolInput:  json.RawMessage(tc.Function.Arguments),
					ToolOutput: results[i].Output,
				})
				for _, hr := range hookResults {
					if hr.Inject == "" {
						continue
					}
					if results[i].InjectText == "" {
						results[i].InjectText = hr.Inject
					} else {
						results[i].InjectText += "\n" + hr.Inject
					}
				}
			}
			continue
		}

		// Builtin: memory_store_bulk — persists multiple memory entries in one call.
		// BUG-MEM-STRESS1: batch alternative so the LLM does not need to loop.
		if tc.Function.Name == "memory_store_bulk" {
			var args struct {
				Facts []struct {
					Content  string `json:"content"`
					Category string `json:"category"`
				} `json:"facts"`
			}
			_ = json.Unmarshal(json.RawMessage(tc.Function.Arguments), &args)
			ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
				ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(tc.Function.Arguments),
			})
			var bulkResult string
			if r.memory != nil {
				// Per-run dedup: skip the actual store if ALL facts in this batch
				// are already in storedMemoryKeys from a prior call this run.
				// Without this, the LLM can call memory_store_bulk in a tight loop
				// (the duplicate-call detector can miss it when JSON formatting varies).
				allAlready := len(args.Facts) > 0
				for _, fact := range args.Facts {
					if _, done := rs.storedMemoryKeys[normalizeKey(fact.Content)]; !done {
						allAlready = false
						break
					}
				}
				if allAlready {
					bulkResult = fmt.Sprintf("All %d facts are already stored — no action needed. Provide your response to the user now.", len(args.Facts))
				} else {
					bulkResult = r.memory.StoreBulk(ctx, in.AgentID, args.Facts)
					for _, fact := range args.Facts {
						rs.storedMemoryKeys[normalizeKey(fact.Content)] = struct{}{}
					}
				}
			} else {
				bulkResult = "Memory storage is not available for this agent."
			}
			out, _ := json.Marshal(bulkResult)
			results[i] = ToolExecResult{Output: out, ToolName: tc.Function.Name}
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted,
			})
			// Post-tool hooks for memory_store_bulk builtin.
			// BUG-HOOK-BUILTIN: capture hookResults to populate InjectText.
			if r.toolExec != nil && r.toolExec.hookExecutor != nil {
				hookResults := r.toolExec.hookExecutor.Execute(ctx, HookPayload{
					Event:      HookPostToolUse,
					AgentID:    in.AgentID.String(),
					SessionID:  in.SessionID.String(),
					ToolName:   tc.Function.Name,
					ToolInput:  json.RawMessage(tc.Function.Arguments),
					ToolOutput: results[i].Output,
				})
				for _, hr := range hookResults {
					if hr.Inject == "" {
						continue
					}
					if results[i].InjectText == "" {
						results[i].InjectText = hr.Inject
					} else {
						results[i].InjectText += "\n" + hr.Inject
					}
				}
			}
			continue
		}

		// Builtin: memory_recall — explicit recall when auto-recall (via system
		// prompt injection) does not surface the relevant memory. Bug 287.
		if tc.Function.Name == "memory_recall" {
			var args struct {
				Query string `json:"query"`
			}
			_ = json.Unmarshal(json.RawMessage(tc.Function.Arguments), &args)
			ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
				ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(tc.Function.Arguments),
			})
			var recallResult string
			if r.memory != nil {
				if args.Query == "" {
					recallResult = "Empty query — provide what you want to recall."
				} else {
					rec, err := r.memory.Recall(ctx, in.AgentID, args.Query)
					if err != nil {
						// Bug 288: scrub internal URLs/IPs before exposing
						// the error to the LLM (and ultimately the client).
						recallResult = "Memory recall failed: " + scrubInternalNetwork(err.Error())
					} else if rec == "" {
						recallResult = "No relevant memories found for this query."
					} else {
						recallResult = rec
					}
				}
			} else {
				recallResult = "Memory recall is not available for this agent."
			}
			out, _ := json.Marshal(recallResult)
			results[i] = ToolExecResult{Output: out, ToolName: tc.Function.Name}
			ch <- NewRunEvent(EventToolProgress, ToolProgressData{
				ID: tc.ID, Name: tc.Function.Name, State: ToolStateCompleted,
			})
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

		// Canvas builtins — canvas_update, canvas_feedback, canvas_export_table.
		// Intercepted locally: emits EventCanvasUpdate or EventInputRequest without
		// hitting skill-runtime. Enables rich visual output in the chat canvas panel.
		if IsCanvasToolCall(tc.Function.Name) {
			ch <- NewRunEvent(EventToolCallStart, ToolCallStartData{
				ID: tc.ID, Name: tc.Function.Name, Input: json.RawMessage(tc.Function.Arguments),
			})
			var result ToolExecResult
			switch tc.Function.Name {
			case canvasUpdateName:
				result = HandleCanvasUpdate(tc.ID, json.RawMessage(tc.Function.Arguments), ch)
			case canvasFeedbackName:
				result = HandleCanvasFeedback(tc.ID, json.RawMessage(tc.Function.Arguments), ch)
			case canvasExportName:
				result = HandleCanvasExportTable(tc.ID, json.RawMessage(tc.Function.Arguments), ch)
			}
			results[i] = result
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
		executionInput := in
		executionInput.SearchOrReadTools = searchOrReadIndex
		execResults := r.toolExec.ExecuteAll(ctx, ch, regularTools, executionInput, readOnlyIndex, inputSchemaIndex)
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

// executeTurnEndHooks runs all registered turn-end handlers and returns inject
// texts from persisted hooks so the caller can emit [SYSTEM NOTE from hook]
// user messages before the next LLM turn.
// BUG-HOOK-TURNEND-INJECT: previously the inject field was discarded.
func (r *Runner) executeTurnEndHooks(ctx context.Context, ch chan<- RunEvent, payload TurnEndPayload) []string {
	if r.toolExec != nil && r.toolExec.hookExecutor != nil {
		return r.toolExec.hookExecutor.ExecuteTurnEnd(ctx, payload, r.turnEndHandlers)
	}
	// No hook executor — run in-memory handlers directly (no inject texts).
	for _, h := range r.turnEndHandlers {
		if err := h.HandleTurnEnd(ctx, payload); err != nil {
			emitError(ch, "turn_end_handler", err)
		}
	}
	return nil
}

// executeRunEndHooks runs all registered run-end handlers and persists any
// inject texts from prompt hooks as audit notes in the session history.
// There is no next LLM turn to inject into, but the notes remain visible
// in the conversation log for audit and observability purposes.
func (r *Runner) executeRunEndHooks(ctx context.Context, ch chan<- RunEvent, payload RunEndPayload) {
	if r.toolExec != nil && r.toolExec.hookExecutor != nil {
		injects := r.toolExec.hookExecutor.ExecuteRunEnd(ctx, payload, r.runEndHandlers)
		for _, inject := range injects {
			note := "[SYSTEM NOTE from run_end hook]\n" + inject
			sessionID, _ := uuid.Parse(payload.SessionID)
			noteMsg := chat.ChatMessage{
				SessionID:   sessionID,
				Role:        "user",
				Content:     note,
				MessageType: chat.MessageTypeText,
				RunID:       &r.runID,
			}
			if r.runID == uuid.Nil {
				noteMsg.RunID = nil
			}
			if _, err := r.persister.CreateMessage(ctx, noteMsg); err != nil {
				slog.Warn("runner: failed to persist run_end hook audit note", "error", err)
			}
		}
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

// computeToolCallSignature returns a stable string that uniquely identifies the
// set of tool calls in a turn (name + raw arguments). Used to detect identical
// consecutive calls that indicate a stuck tool-call loop.
func computeToolCallSignature(calls []ai.ToolCall) string {
	if len(calls) == 0 {
		return ""
	}
	parts := make([]string, len(calls))
	for i, c := range calls {
		parts[i] = c.Function.Name + ":" + c.Function.Arguments
	}
	return strings.Join(parts, "|")
}

func emitError(ch chan<- RunEvent, code string, err error) {
	// Bug 289: defensive scrub — every emitError reaches the SSE client,
	// so any error string carrying an internal URL/IP/svc is a network
	// disclosure. Apply scrubInternalNetwork as a safety net even when
	// the caller forgets.
	ch <- NewRunEvent(EventError, ErrorData{
		Message: sanitizeSSEMessage(err.Error()),
		Code:    code,
	})
}

func emitWarning(ch chan<- RunEvent, code string, message string) {
	ch <- NewRunEvent(EventWarning, WarningData{
		Message: sanitizeSSEMessage(message),
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

// friendlyRunErrorMessage maps internal error codes to user-facing messages.
// This is used when a run fails and the error needs to be persisted to chat history.
func friendlyRunErrorMessage(code, rawMsg string) string {
	switch code {
	case "llm_call", "stream_consume":
		// P-C252-1: distinguish timeout from unavailable — both produce errors but need
		// different user guidance (retry simpler request vs retry later).
		if strings.Contains(rawMsg, "deadline exceeded") || strings.Contains(rawMsg, "context deadline") || strings.Contains(rawMsg, "timeout") {
			return "O modelo de IA demorou muito para responder (timeout). Tente novamente com uma solicitação mais simples ou em alguns instantes."
		}
		if strings.Contains(rawMsg, "401") || strings.Contains(rawMsg, "authentication") || strings.Contains(rawMsg, "Unauthorized") {
			return "Não foi possível chamar o modelo de IA: credenciais inválidas ou expiradas. Verifique a chave de API nas configurações."
		}
		if strings.Contains(rawMsg, "402") || strings.Contains(rawMsg, "credit") || strings.Contains(rawMsg, "balance") {
			return "Não foi possível chamar o modelo de IA: saldo insuficiente. Verifique seu plano na plataforma do provedor."
		}
		if strings.Contains(rawMsg, "429") || strings.Contains(rawMsg, "rate limit") {
			return "O serviço de IA está temporariamente sobrecarregado (rate limit). Tente novamente em alguns instantes."
		}
		if strings.Contains(rawMsg, "503") || strings.Contains(rawMsg, "unavailable") {
			return "O serviço de IA está temporariamente indisponível. Tente novamente em alguns instantes."
		}
		if strings.Contains(rawMsg, "404") || strings.Contains(rawMsg, "does not exist") || strings.Contains(rawMsg, "model not found") {
			return "O modelo de IA configurado neste agente não existe. Verifique o nome do modelo nas configurações do agente."
		}
		if strings.Contains(rawMsg, "400") {
			return "O modelo de IA rejeitou a solicitação. Verifique a configuração do modelo nas configurações do agente."
		}
		return "O serviço de IA encontrou um erro inesperado. Tente novamente."
	case "prompt_build":
		return "Ocorreu um erro ao preparar o contexto da conversa. Tente novamente."
	case "load_history":
		return "Ocorreu um erro ao carregar o histórico da conversa. Tente novamente."
	case "tool_schema_build":
		return "Ocorreu um erro ao carregar as ferramentas do agente. Tente novamente."
	case "budget_exceeded":
		return "O limite de custo desta sessão foi atingido. A execução foi interrompida."
	case "max_iterations":
		return "O agente atingiu o limite máximo de iterações sem concluir a tarefa. Tente reformular sua solicitação."
	case "compact_circuit_breaker":
		return "Ocorreu um erro ao compactar o contexto da conversa. Tente iniciar uma nova sessão."
	case "context_cancelled":
		return "A solicitação foi cancelada."
	default:
		return fmt.Sprintf("Não foi possível completar a solicitação. Tente novamente. (código: %s)", code)
	}
}

// logPermissionDecision writes a permission audit entry if the logger is configured.
// Non-fatal: errors are silently discarded to avoid interrupting the agentic loop.
func (r *Runner) logPermissionDecision(ctx context.Context, in RunInput, toolName, toolInput string, decision PermissionAuditDecision) {
	if in.PermissionAudit == nil {
		return
	}
	entry := PermissionAuditEntry{
		SessionID:    in.SessionID,
		RunID:        &in.RunID,
		ToolName:     toolName,
		Decision:     decision,
		InputSnippet: toolInput,
	}
	_ = in.PermissionAudit.LogDecision(ctx, entry)
}

// askConsentViaElicitation presents a confirmation prompt to the user and returns
// true if the user approved the tool call, false otherwise.
// Uses the ElicitationSubmitter to block until the user responds.
func (r *Runner) askConsentViaElicitation(ctx context.Context, in RunInput, toolName, toolInput string) bool {
	requestID := "consent-" + toolName + "-" + r.runID.String()
	snippet := truncateInput(toolInput, 300)

	question := AskUserQuestion{
		ID:       "consent",
		Question: fmt.Sprintf("Permit tool '%s' to run?\n\nInput preview:\n%s", toolName, snippet),
		Type:     "confirm",
	}

	params := ElicitationParams{
		Mode:          ElicitationModeForm,
		Message:       fmt.Sprintf("Tool '%s' requires your approval before executing.", toolName),
		ElicitationID: requestID,
		Questions:     []AskUserQuestion{question},
	}

	result := in.Elicitation.Submit(ctx, "", requestID, params)
	return result.Action == ElicitationAccept
}
