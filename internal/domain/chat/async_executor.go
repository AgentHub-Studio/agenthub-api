package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

const ChatRunQueue = "chat.run.queue"

// ChatRunTask is the message payload for RabbitMQ.
type ChatRunTask struct {
	RunID     uuid.UUID `json:"runId"`
	SessionID uuid.UUID `json:"sessionId"`
	TenantID  string    `json:"tenantId"`
	Message   string    `json:"message"`
	// RawToken is the caller's Bearer JWT forwarded so that background workers
	// can authenticate outbound calls to the skill-runtime. Without this, all
	// tool executions fail with 401 because the async context has no token.
	// Note: tokens are short-lived; runs that start near expiry may still fail.
	RawToken string `json:"rawToken,omitempty"`
}

// defaultRunTimeout is the maximum time a single background run may take.
// P-C102-1: prevents a slow/unresponsive LLM from blocking a session forever.
//
// Configurable via the CHAT_RUN_TIMEOUT_SECS env var (read at process start).
// Slow hardware (CPU-only ollama with large models) benefits from longer caps.
var defaultRunTimeout = resolveRunTimeout()

// resolveRunTimeout reads CHAT_RUN_TIMEOUT_SECS; falls back to 15 min on empty
// or malformed input. Bounds: [60s, 2h] to prevent accidental misconfiguration.
func resolveRunTimeout() time.Duration {
	const fallback = 15 * time.Minute
	raw := strings.TrimSpace(os.Getenv("CHAT_RUN_TIMEOUT_SECS"))
	if raw == "" {
		return fallback
	}
	secs, err := strconv.Atoi(raw)
	if err != nil || secs < 60 {
		return fallback
	}
	const maxSecs = 2 * 60 * 60
	if secs > maxSecs {
		secs = maxSecs
	}
	return time.Duration(secs) * time.Second
}

// MetricsRecorder is the slim interface AsyncExecutor needs to persist
// agent_metrics rows after a run completes. metrics.Service satisfies it.
type MetricsRecorder interface {
	Record(ctx context.Context, tenantID string, req MetricsRecord) error
}

// MetricsRecord mirrors metrics.RecordRequest with only the fields the
// AsyncExecutor populates from RunMetadata. Defined here to avoid an
// import cycle between chat and metrics.
type MetricsRecord struct {
	AgentID          uuid.UUID
	SessionID        string
	ModelName        string
	Provider         string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
	EstimatedCostUSD float64 // Bug 221: era omitido — todas analytics mostravam $0
	LatencyMs        int64
}

// AsyncExecutor handles asynchronous execution of chat runs via RabbitMQ.
// AgentExister checks if an agent ID exists in the tenant's catalog.
// Bug 244: chat session can outlive its agent (DELETE agent leaves the
// session orphaned). EnqueueRun must reject runs whose agent has been
// deleted instead of accepting and failing later in the worker.
type AgentExister interface {
	GetByID(ctx context.Context, id uuid.UUID) error
}

// RunCompletionHook is called after a chat run finishes (success/fail/cancel).
// Bug 291: triggers register one of these to close their trigger_run rows.
// Hooks must be non-blocking and safe under concurrent runs.
type RunCompletionHook func(ctx context.Context, sessionID, runID uuid.UUID, status ChatRunStatus, turns, tokens int, errMsg string)

type AsyncExecutor struct {
	repo            Repository
	runner          SessionRunner
	agentLoader     AgentLoader // optional: used to enrich RunInput with agent bindings
	agentExister    AgentExister
	metricsRecorder MetricsRecorder
	completionHooks []RunCompletionHook
	connURL         string
	runTimeout      time.Duration
	bufferRegistry  *RunEventBufferRegistry
	// cancellers maps runID → context.CancelFunc for in-flight worker tasks
	// in this process. Populated in processTask and cleared on completion.
	// Allows the HTTP handler to abort an async run from POST /run/{id}/cancel.
	cancellers sync.Map
	// cancelled records runIDs explicitly cancelled by Cancel() so processTask
	// can distinguish a deliberate cancel from a timeout when ctx.Err() fires.
	cancelled sync.Map
}

// WithMetricsRecorder wires the metrics service so each completed async
// run inserts a row in agent_metrics. Without this, the table is only
// populated by external POST /api/metrics calls.
func (e *AsyncExecutor) WithMetricsRecorder(rec MetricsRecorder) *AsyncExecutor {
	e.metricsRecorder = rec
	return e
}

// WithCompletionHook appends a hook fired when each run terminates.
func (e *AsyncExecutor) WithCompletionHook(hook RunCompletionHook) *AsyncExecutor {
	e.completionHooks = append(e.completionHooks, hook)
	return e
}

func NewAsyncExecutor(repo Repository, runner SessionRunner, connURL string) *AsyncExecutor {
	return &AsyncExecutor{
		repo:       repo,
		runner:     runner,
		connURL:    connURL,
		runTimeout: defaultRunTimeout,
	}
}

// WithEventBufferRegistry wires the shared SSE replay buffer registry used by
// the HTTP handler. Async runs must publish into the same registry so
// GET /api/chat/sessions/{id}/run/{runId}/resume works for RabbitMQ-backed runs.
func (e *AsyncExecutor) WithEventBufferRegistry(reg *RunEventBufferRegistry) *AsyncExecutor {
	e.bufferRegistry = reg
	return e
}

// WithAgentLoader wires an agent loader so that per-agent MCP server bindings
// (and other agent-level snapshots) are applied to background runs.
// P-C253-1: without this, async runs ignore agent MCP binding lists.
func (e *AsyncExecutor) WithAgentLoader(loader AgentLoader) *AsyncExecutor {
	e.agentLoader = loader
	return e
}

// WithAgentExister wires an agent existence checker. Bug 244: chat sessions
// can outlive their agent (DELETE /api/agents/{id} doesn't cascade or
// archive sessions). EnqueueRun must reject runs on orphaned sessions
// instead of accepting and failing in the worker.
func (e *AsyncExecutor) WithAgentExister(ax AgentExister) *AsyncExecutor {
	e.agentExister = ax
	return e
}

// GetRunByID looks up a run from the persistent store (DB).
// Used by the handler to resolve async runs that are not in the in-memory bgRegistry.
func (e *AsyncExecutor) GetRunByID(ctx context.Context, id uuid.UUID) (ChatRun, error) {
	return e.repo.GetRunByID(ctx, id)
}

// Cancel aborts an in-flight async run. Returns true if a cancel was
// dispatched to the worker, false when the runID is not currently being
// processed by this process. Records the cancellation intent so
// processTask can mark the run as cancelled (rather than failed) when
// the context-cancel propagates to the Runner.
//
// This complements the bgRegistry path used by legacy synchronous runs.
// Without this, POST /api/chat/sessions/{id}/run/{runId}/cancel returns
// 404 for any run dispatched via RabbitMQ — which is the production path.
func (e *AsyncExecutor) Cancel(runID uuid.UUID) bool {
	e.cancelled.Store(runID, true)
	v, ok := e.cancellers.Load(runID)
	if !ok {
		return false
	}
	if cancelFn, ok := v.(context.CancelFunc); ok {
		cancelFn()
		return true
	}
	return false
}

// wasCancelled reports whether Cancel was invoked for this runID since
// the worker took the task. Used to distinguish cancel from timeout.
func (e *AsyncExecutor) wasCancelled(runID uuid.UUID) bool {
	_, ok := e.cancelled.Load(runID)
	return ok
}

// EnqueueRun persists a new run and sends a task to RabbitMQ.
// P-C99-1: rejects the request with ErrRunAlreadyActive when a run is already
// in progress for the session, preventing concurrent runs that corrupt history.
func (e *AsyncExecutor) EnqueueRun(ctx context.Context, sessionID uuid.UUID, tenantID, message string) (uuid.UUID, error) {
	// Bug 244: validar que session existe e o agent ainda existe ANTES de
	// criar o run e enfileirar. Sem isso, sessions órfãs (agent deletado)
	// aceitam runs que silenciosamente fazem fallback para um default agent
	// no worker — usuário pensa que está conversando com seu agent original.
	// Quando DELETE /api/agents/{id} executa, o FK ON DELETE SET NULL
	// reseta chat_session.agent_id para NULL — então cobrimos ambos casos:
	// (a) AgentID nil = agent já foi deletado e FK setou NULL
	// (b) AgentID set mas GetByID retorna erro = agent foi deletado
	//     em outro tenant ou inconsistência rara.
	if e.agentExister != nil {
		session, err := e.repo.GetSessionByID(ctx, sessionID)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				return uuid.Nil, ErrNotFound
			}
			return uuid.Nil, fmt.Errorf("chat: lookup session: %w", err)
		}
		if session.AgentID == nil {
			return uuid.Nil, ErrAgentNotFound
		}
		// Bug 246: rejeitar runs em session ARCHIVED — usuário arquivou
		// como sinal de "não usar mais"; aceitar runs subverte isso.
		if session.Status == StatusArchived {
			return uuid.Nil, ErrSessionArchived
		}
		if err := e.agentExister.GetByID(ctx, *session.AgentID); err != nil {
			return uuid.Nil, ErrAgentNotFound
		}
	}

	// Guard: reject if a run is already active for this session.
	// P-C103-1: auto-expire orphaned runs that have been active longer than 2x the timeout.
	// This handles pod restarts that leave runs stuck in 'active' indefinitely.
	if stale, active, err := e.repo.GetActiveRunBySession(ctx, sessionID); err != nil {
		return uuid.Nil, fmt.Errorf("chat: check active run: %w", err)
	} else if active {
		staleThreshold := e.runTimeout
		// P-C299-1: queued runs (StartedAt is zero) have been waiting in the RabbitMQ queue.
		// Expire them after 2× the run timeout — if the queue has been stagnant that long,
		// the worker is likely down. Active runs (StartedAt non-zero) expire at the normal threshold.
		if stale.Status == ChatRunStatusQueued && time.Since(stale.CreatedAt) > 2*staleThreshold {
			slog.Warn("chat: auto-expiring stale queued run (worker may be down)", "runId", stale.ID, "age", time.Since(stale.CreatedAt))
			_ = e.repo.MarkRunFailed(ctx, stale.ID, "run expired in queue — worker may be down")
		} else if stale.StartedAt != nil && !stale.StartedAt.IsZero() && time.Since(*stale.StartedAt) > staleThreshold {
			slog.Warn("chat: auto-expiring orphaned active run", "runId", stale.ID, "age", time.Since(*stale.StartedAt))
			_ = e.repo.MarkRunFailed(ctx, stale.ID, "run exceeded maximum duration and was auto-expired")
		} else {
			return uuid.Nil, ErrRunAlreadyActive
		}
	}

	// 1. Create Run in DB with 'queued' status (P-C299-1).
	// The worker transitions to 'active' when it actually starts processing.
	run, err := e.repo.CreateRun(ctx, ChatRun{
		SessionID: sessionID,
		TenantID:  tenantID,
		Status:    ChatRunStatusQueued,
	})
	if err != nil {
		// FK violation (23503) significa session_id inexistente.
		// Mapeia para ErrNotFound para que handler retorne 404 em
		// vez de vazar 500 com SQL error exposto.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, err
	}

	// Pre-create the replay buffer before the worker starts so resume requests
	// can attach immediately after the 202 Accepted response.
	if e.bufferRegistry != nil {
		e.bufferRegistry.GetOrCreate(run.ID.String(), DefaultEventBufferSize)
	}

	// 2. Publish to RabbitMQ
	if e.connURL == "" {
		slog.Warn("rabbitmq: no URL configured, async execution disabled")
		return run.ID, nil
	}

	conn, err := amqp.Dial(e.connURL)
	if err != nil {
		return run.ID, fmt.Errorf("rabbitmq: dial: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return run.ID, fmt.Errorf("rabbitmq: channel: %w", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(ChatRunQueue, true, false, false, false, nil)
	if err != nil {
		return run.ID, fmt.Errorf("rabbitmq: queue declare: %w", err)
	}

	body, _ := json.Marshal(ChatRunTask{
		RunID:     run.ID,
		SessionID: sessionID,
		TenantID:  tenantID,
		Message:   message,
		RawToken:  tenant.TokenFromContext(ctx),
	})

	err = ch.PublishWithContext(ctx, "", q.Name, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err != nil {
		return run.ID, fmt.Errorf("rabbitmq: publish: %w", err)
	}

	return run.ID, nil
}

// StartWorker starts a blocking consumer for chat run tasks.
func (e *AsyncExecutor) StartWorker(ctx context.Context) error {
	if e.connURL == "" {
		return fmt.Errorf("rabbitmq: no URL configured")
	}

	conn, err := amqp.Dial(e.connURL)
	if err != nil {
		return err
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(ChatRunQueue, true, false, false, false, nil)
	if err != nil {
		return err
	}

	msgs, err := ch.Consume(q.Name, "agenthub-api-worker", false, false, false, false, nil)
	if err != nil {
		return err
	}

	slog.Info("rabbitmq: chat run worker started", "queue", ChatRunQueue)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case d, ok := <-msgs:
			if !ok {
				return fmt.Errorf("rabbitmq: channel closed")
			}

			var task ChatRunTask
			if err := json.Unmarshal(d.Body, &task); err != nil {
				slog.Error("rabbitmq: unmarshal task", "err", err)
				d.Nack(false, false)
				continue
			}

			// Execute the run
			go e.processTask(task)
			d.Ack(false)
		}
	}
}

func (e *AsyncExecutor) getOrCreateBuffer(runID uuid.UUID) *EventBuffer {
	if e.bufferRegistry == nil {
		return nil
	}
	return e.bufferRegistry.GetOrCreate(runID.String(), DefaultEventBufferSize)
}

func appendBufferedError(buf *EventBuffer, message, code string) {
	if buf == nil {
		return
	}
	data, _ := json.Marshal(map[string]any{
		"message": message,
		"code":    code,
	})
	buf.Append(RunEvent{Type: "error", Data: data})
}

func (e *AsyncExecutor) processTask(task ChatRunTask) {
	// task.TenantID already contains the schema name (e.g., "ah_test") in some contexts,
	// but the NewContext should receive the raw tenant ID if AcquireWithTenant
	// adds the "ah_" prefix.
	// Based on AcquireWithTenant: schema := fmt.Sprintf("ah_%s", tenantID)
	// If task.TenantID is "ah_test", we should pass "test".
	tenantID := task.TenantID
	if len(tenantID) > 3 && tenantID[:3] == "ah_" {
		tenantID = tenantID[3:]
	}
	// Restore the caller's token into the context so skill-runtime calls are
	// authenticated. The token was forwarded from the original HTTP request.
	// P-C102-1: apply a per-run deadline so a slow/unresponsive LLM cannot block
	// the session indefinitely. The timeout is configurable via runTimeout.
	// baseCtx retains tenant+token info and is used for post-timeout DB writes.
	baseCtx := tenant.NewContextWithToken(context.Background(), tenantID, task.RawToken)
	ctx, cancel := context.WithTimeout(baseCtx, e.runTimeout)
	defer cancel()

	// Register the cancel func so AsyncExecutor.Cancel(runID) can abort
	// this run from another goroutine (the HTTP handler). Cleared in the
	// deferred remove below regardless of completion path.
	e.cancellers.Store(task.RunID, cancel)
	defer func() {
		e.cancellers.Delete(task.RunID)
		e.cancelled.Delete(task.RunID)
	}()

	// cleanupCtx: used for DB writes after ctx is cancelled — has tenant but no deadline.
	cleanupCtx := baseCtx
	buf := e.getOrCreateBuffer(task.RunID)
	if buf != nil {
		defer buf.MarkDone()
	}

	slog.Info("chat: background run starting", "runId", task.RunID, "sessionId", task.SessionID, "tenant", task.TenantID)

	// P-C299-1: transition from 'queued' → 'active' now that the worker has picked up the task.
	// This lets polling clients distinguish "waiting in queue" from "actively processing".
	_ = e.repo.UpdateRunStatus(ctx, task.RunID, ChatRunStatusActive, "")

	// Since we are in a background worker, we don't have the original SSE stream.
	// We just run the session and let the Runner handle persistence of messages.

	// 1. Resolve session to get AgentID
	session, err := e.repo.GetSessionByID(ctx, task.SessionID)
	if err != nil {
		slog.Error("chat: background run failed to load session", "runId", task.RunID, "err", err)
		_ = e.repo.MarkRunFailed(ctx, task.RunID, "failed to load session: "+err.Error())
		appendBufferedError(buf, "failed to load session: "+err.Error(), "startup")
		return
	}

	if session.AgentID == nil {
		// P-C292-1: mirror the SSE path — attempt to bind the default published agent
		// before giving up, so async runs on agent-less sessions behave identically.
		defaultID, err := e.repo.FindDefaultAgentID(ctx)
		if err != nil || defaultID == nil {
			slog.Error("chat: background run session has no agent and no default agent found", "runId", task.RunID, "err", err)
			_ = e.repo.MarkRunFailed(ctx, task.RunID, "no agent configured for this session and no default published agent found")
			appendBufferedError(buf, "no agent configured for this session and no default published agent found", "startup")
			// P-C292-2: persist a user-facing error so chat history is not left empty.
			_, _ = e.repo.CreateMessage(ctx, ChatMessage{
				SessionID:   task.SessionID,
				Role:        "assistant",
				Content:     "Não foi possível iniciar o agente: nenhum agente está configurado para esta sessão e não há agente padrão publicado.",
				MessageType: MessageTypeText,
			})
			return
		}
		if err := e.repo.UpdateSessionAgent(ctx, task.SessionID, *defaultID); err != nil {
			slog.Error("chat: background run failed to bind default agent", "runId", task.RunID, "err", err)
			_ = e.repo.MarkRunFailed(ctx, task.RunID, "failed to bind default agent: "+err.Error())
			appendBufferedError(buf, "failed to bind default agent: "+err.Error(), "startup")
			return
		}
		session.AgentID = defaultID
		slog.Info("chat: background run bound default agent to session", "runId", task.RunID, "agentId", *defaultID)
	}

	// P-C253-1: load MCP server names so the runner filters tools to only those
	// from servers explicitly bound to this agent. Without this, all MCP tools
	// from all running servers would be included regardless of agent binding.
	var mcpServerNames []string
	if e.agentLoader != nil {
		if agentCfg, err := e.agentLoader.GetAgentForRun(ctx, *session.AgentID); err == nil {
			mcpServerNames = agentCfg.MCPServerNames
		}
	}

	// 2. Execute the run
	runEvents, err := e.runner.RunSession(ctx, RunInput{
		RunID:                  task.RunID,
		SessionID:              task.SessionID,
		AgentID:                *session.AgentID,
		TenantID:               task.TenantID,
		UserMessage:            task.Message,
		MCPServerNamesSnapshot: mcpServerNames,
	})

	if err != nil {
		slog.Error("chat: background run failed to start", "runId", task.RunID, "err", err)
		_ = e.repo.MarkRunFailed(ctx, task.RunID, err.Error())
		appendBufferedError(buf, err.Error(), "startup")
		// Persist a user-facing error message so the chat history is not left empty.
		// This covers failures that happen before the runner starts (e.g. unknown provider,
		// missing API key settings) which the runner's own error-persistence path cannot handle.
		errMsg := friendlyStartupError(err.Error())
		_, _ = e.repo.CreateMessage(ctx, ChatMessage{
			SessionID:   task.SessionID,
			Role:        "assistant",
			Content:     errMsg,
			MessageType: MessageTypeText,
		})
		return
	}

	// Drain events to allow the runner to finish and persist messages.
	// The Runner itself handles persistence of messages (CreateMessage)
	// so we just need to wait for the events channel to close.
	// P-C106-1: the runner's deferred cleanup writes a user-facing message only for
	// specific error codes (llm_call, stream_consume). Track whether such an event
	// was seen so we skip the duplicate timeout message below.
	// Infrastructure errors (persist_tool_result, etc.) also emit error events but
	// do NOT write user messages — those must NOT suppress the timeout message.
	runnerWroteUserMessage := false
	var lastLLMErrorMsg string // non-empty when a fatal LLM error (llm_call/stream_consume) occurred
	for event := range runEvents {
		if buf != nil {
			buf.Append(event)
		}
		if event.Type == "error" {
			slog.Error("chat: background run error event", "runId", task.RunID, "data", string(event.Data))
			var errData struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if json.Unmarshal(event.Data, &errData) == nil {
				switch errData.Code {
				case "stream_consume", "llm_call":
					// Runner deferred cleanup will persist a user-facing message for these.
					runnerWroteUserMessage = true
					lastLLMErrorMsg = errData.Message
				}
			}
		}
	}

	// P-C102-1: if the context expired (timeout), mark run failed and surface error to user.
	// Use cleanupCtx (has tenant, no deadline) so the DB writes succeed.
	// P-C106-1: skip the timeout message only when the runner already persisted one
	// (prevents two consecutive error messages in the chat history).
	if ctx.Err() != nil {
		// Differentiate explicit cancel (POST /cancel) from a timeout. In both
		// cases the Runner observed ctx.Done() and returned, but the user-facing
		// status and chat history must reflect the right cause.
		if e.wasCancelled(task.RunID) {
			slog.Info("chat: background run cancelled by user", "runId", task.RunID)
			_ = e.repo.UpdateRunStatus(cleanupCtx, task.RunID, ChatRunStatusCancelled, "")
			appendBufferedError(buf, "run cancelled by user", "cancelled")
			e.fireCompletionHooks(cleanupCtx, task, ChatRunStatusCancelled, 0, 0, "cancelled by user")
			return
		}
		slog.Warn("chat: background run timed out", "runId", task.RunID, "timeout", e.runTimeout)
		_ = e.repo.MarkRunFailed(cleanupCtx, task.RunID, fmt.Sprintf("run exceeded the %s timeout", e.runTimeout))
		appendBufferedError(buf, fmt.Sprintf("run exceeded the %s timeout", e.runTimeout), "timeout")
		if !runnerWroteUserMessage {
			_, _ = e.repo.CreateMessage(cleanupCtx, ChatMessage{
				SessionID:   task.SessionID,
				Role:        "assistant",
				Content:     "O agente excedeu o tempo máximo de resposta. Tente novamente ou verifique a configuração do modelo de IA.",
				MessageType: MessageTypeText,
			})
		}
		e.fireCompletionHooks(cleanupCtx, task, ChatRunStatusFailed, 0, 0, "timeout")
		return
	}

	// If the runner encountered a fatal LLM error (e.g. provider rejected the request),
	// mark the run as failed so callers get a clear, queryable status rather than a
	// misleading "completed" with an error buried in metadata.
	if lastLLMErrorMsg != "" {
		_ = e.repo.MarkRunFailed(cleanupCtx, task.RunID, lastLLMErrorMsg)
		slog.Warn("chat: background run failed due to LLM error", "runId", task.RunID, "err", lastLLMErrorMsg)
		e.fireCompletionHooks(cleanupCtx, task, ChatRunStatusFailed, 0, 0, lastLLMErrorMsg)
		return
	}

	_ = e.repo.MarkRunCompleted(ctx, task.RunID)
	slog.Info("chat: background run completed", "runId", task.RunID)
	e.fireCompletionHooks(cleanupCtx, task, ChatRunStatusCompleted, 0, 0, "")

	// Persist agent_metrics row from the run metadata so the metrics
	// dashboards and cost-tracking endpoints have data without requiring
	// an out-of-band POST /api/metrics. session.AgentID is non-nil at
	// this point — defaulted earlier in processTask if missing.
	if e.metricsRecorder != nil && session.AgentID != nil {
		e.recordMetricsFromRun(cleanupCtx, task, *session.AgentID)
	}
}

// fireCompletionHooks invokes registered hooks. Each hook runs in its own
// goroutine with a 5s timeout — a slow hook must never block the next run.
// Bug 292: when called for a successful run with turns/tokens=0, read
// run.metadata to populate them so trigger_run history shows real usage.
func (e *AsyncExecutor) fireCompletionHooks(ctx context.Context, task ChatRunTask, status ChatRunStatus, turns, tokens int, errMsg string) {
	if len(e.completionHooks) == 0 {
		return
	}
	if status == ChatRunStatusCompleted && turns == 0 && tokens == 0 {
		if run, err := e.repo.GetRunByID(ctx, task.RunID); err == nil && len(run.Metadata) > 0 {
			var meta struct {
				TotalTurns        int `json:"totalTurns"`
				TotalInputTokens  int `json:"totalInputTokens"`
				TotalOutputTokens int `json:"totalOutputTokens"`
			}
			if json.Unmarshal(run.Metadata, &meta) == nil {
				turns = meta.TotalTurns
				tokens = meta.TotalInputTokens + meta.TotalOutputTokens
			}
		}
	}
	for _, h := range e.completionHooks {
		hook := h
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("chat: completion hook panicked", "runId", task.RunID, "panic", r)
				}
			}()
			hookCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			hook(hookCtx, task.SessionID, task.RunID, status, turns, tokens, errMsg)
		}()
	}
}

// recordMetricsFromRun reads the chat_run row that processTask just
// completed and inserts a corresponding agent_metrics row. Token totals
// come from RunMetadata; provider/model from the same payload. Errors
// are logged but never propagate — a failed metric must not retry the
// whole run.
func (e *AsyncExecutor) recordMetricsFromRun(ctx context.Context, task ChatRunTask, agentID uuid.UUID) {
	run, err := e.repo.GetRunByID(ctx, task.RunID)
	if err != nil {
		slog.Warn("metrics: failed to load run for metric recording", "runId", task.RunID, "err", err)
		return
	}
	if len(run.Metadata) == 0 {
		return
	}
	var meta struct {
		TotalInputTokens  int     `json:"totalInputTokens"`
		TotalOutputTokens int     `json:"totalOutputTokens"`
		TotalCostUSD      float64 `json:"totalCostUsd"` // Bug 221: lê do RunMetadata
		ModelUsed         string  `json:"modelUsed"`
		ProviderUsed      string  `json:"providerUsed"`
		DurationMs        int64   `json:"durationMs"`
	}
	if err := json.Unmarshal(run.Metadata, &meta); err != nil {
		slog.Warn("metrics: failed to parse run metadata", "runId", task.RunID, "err", err)
		return
	}
	if meta.TotalInputTokens == 0 && meta.TotalOutputTokens == 0 {
		// No tokens reported (provider didn't include usage) — skip the
		// row to avoid polluting metrics with zeros.
		return
	}
	tenantID := strings.TrimPrefix(task.TenantID, "ah_")
	if err := e.metricsRecorder.Record(ctx, tenantID, MetricsRecord{
		AgentID:          agentID,
		SessionID:        task.SessionID.String(),
		ModelName:        meta.ModelUsed,
		Provider:         meta.ProviderUsed,
		PromptTokens:     meta.TotalInputTokens,
		CompletionTokens: meta.TotalOutputTokens,
		TotalTokens:      meta.TotalInputTokens + meta.TotalOutputTokens,
		EstimatedCostUSD: meta.TotalCostUSD, // Bug 221
		LatencyMs:        meta.DurationMs,
	}); err != nil {
		slog.Warn("metrics: record failed", "runId", task.RunID, "err", err)
	}
}

// providerNameRE captures the provider slug out of error strings like
//   build model for provider "anthropic": chat model: claude.apiKey not configured
// so the friendly message can point the user at the specific provider row
// in the tenant settings.
var providerNameRE = regexp.MustCompile(`provider\s+"([^"]+)"`)

// settingKeyRE captures the settings key that the backend expected, e.g.
//   chat model: claude.apiKey not configured
// Used to tell the user which setting to fill in.
var settingKeyRE = regexp.MustCompile(`([a-z][a-zA-Z0-9_]*\.[a-zA-Z][a-zA-Z0-9_]*)\s+not configured`)

// friendlyStartupError maps errors that occur before the runner loop starts
// to user-facing messages suitable for persisting to chat history.
//
// Every branch must be explicit about (a) what failed, (b) the probable root
// cause and (c) where to fix it. If we fall through to the default, the message
// still names the setting key extracted from the raw error.
func friendlyStartupError(rawMsg string) string {
	if strings.Contains(rawMsg, "is not published") || strings.Contains(rawMsg, "DRAFT status") || strings.Contains(rawMsg, "agent is not published") {
		return "Não foi possível iniciar o agente: este agente ainda não foi publicado. Abra o agente na seção Administração → Agentes e clique em Publicar antes de usá-lo no chat."
	}
	if strings.Contains(rawMsg, "is archived") || strings.Contains(rawMsg, "ARCHIVED") {
		return "Não foi possível iniciar o agente: ele foi arquivado e não aceita novas conversas. Restaure-o em Administração → Agentes ou use outro agente."
	}

	provider := ""
	if m := providerNameRE.FindStringSubmatch(rawMsg); len(m) == 2 {
		provider = m[1]
	}
	settingKey := ""
	if m := settingKeyRE.FindStringSubmatch(rawMsg); len(m) == 2 {
		settingKey = m[1]
	}

	if strings.Contains(rawMsg, "unsupported provider") || strings.Contains(rawMsg, "unknown provider") {
		if provider != "" {
			return fmt.Sprintf("Não foi possível iniciar o agente: o provedor de IA %q informado no agente não é suportado pela plataforma. Edite o agente (Administração → Agentes → editar → modelo) e escolha um provedor válido (anthropic, openai, openrouter, ollama).", provider)
		}
		return "Não foi possível iniciar o agente: o provedor de IA configurado não é suportado. Edite o agente (Administração → Agentes → editar → modelo) e escolha um provedor válido."
	}
	if strings.Contains(rawMsg, "no AI provider configured") ||
		strings.Contains(rawMsg, "api key") ||
		strings.Contains(rawMsg, "API key") ||
		strings.Contains(rawMsg, "apiKey") ||
		strings.Contains(rawMsg, "not configured in settings") {
		// Most specific: we know both the provider AND the settings key.
		if provider != "" && settingKey != "" {
			return fmt.Sprintf("Não foi possível iniciar o agente: a chave de API do provedor %q não está configurada neste tenant (configuração %q ausente). Acesse Administração → Configurações → Provedores, selecione %s e preencha a chave de API.", provider, settingKey, provider)
		}
		if provider != "" {
			return fmt.Sprintf("Não foi possível iniciar o agente: a chave de API do provedor %q não está configurada neste tenant. Acesse Administração → Configurações → Provedores, selecione %s e preencha a chave de API.", provider, provider)
		}
		return "Não foi possível iniciar o agente: chave de API do provedor de IA não configurada neste tenant. Acesse Administração → Configurações → Provedores e preencha a chave do provedor usado pelo agente."
	}
	if strings.Contains(rawMsg, "build model") {
		if provider != "" {
			return fmt.Sprintf("Não foi possível iniciar o agente: erro ao configurar o provedor %q (verifique o modelo selecionado no agente e a chave de API em Administração → Configurações → Provedores).", provider)
		}
		return "Não foi possível iniciar o agente: erro ao configurar o modelo de IA. Verifique o provedor e o modelo em Administração → Agentes e a chave de API em Administração → Configurações → Provedores."
	}

	// Default: still explicit about next steps even when we don't recognize the cause.
	if provider != "" {
		return fmt.Sprintf("Não foi possível iniciar o agente (provedor %q). Verifique o modelo selecionado no agente e a chave de API em Administração → Configurações → Provedores. Detalhes técnicos: %s", provider, truncate(rawMsg, 200))
	}
	return fmt.Sprintf("Não foi possível iniciar o agente. Verifique o modelo selecionado em Administração → Agentes e a chave de API em Administração → Configurações → Provedores. Detalhes técnicos: %s", truncate(rawMsg, 200))
}

// truncate returns s bounded to at most n runes, adding an ellipsis when cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
