package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
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
const defaultRunTimeout = 15 * time.Minute

// AsyncExecutor handles asynchronous execution of chat runs via RabbitMQ.
type AsyncExecutor struct {
	repo       Repository
	runner     SessionRunner
	connURL    string
	runTimeout time.Duration
}

func NewAsyncExecutor(repo Repository, runner SessionRunner, connURL string) *AsyncExecutor {
	return &AsyncExecutor{
		repo:       repo,
		runner:     runner,
		connURL:    connURL,
		runTimeout: defaultRunTimeout,
	}
}

// GetRunByID looks up a run from the persistent store (DB).
// Used by the handler to resolve async runs that are not in the in-memory bgRegistry.
func (e *AsyncExecutor) GetRunByID(ctx context.Context, id uuid.UUID) (ChatRun, error) {
	return e.repo.GetRunByID(ctx, id)
}

// EnqueueRun persists a new run and sends a task to RabbitMQ.
// P-C99-1: rejects the request with ErrRunAlreadyActive when a run is already
// in progress for the session, preventing concurrent runs that corrupt history.
func (e *AsyncExecutor) EnqueueRun(ctx context.Context, sessionID uuid.UUID, tenantID, message string) (uuid.UUID, error) {
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
			_ = e.repo.UpdateRunStatus(ctx, stale.ID, ChatRunStatusFailed, "")
		} else if !stale.StartedAt.IsZero() && time.Since(stale.StartedAt) > staleThreshold {
			slog.Warn("chat: auto-expiring orphaned active run", "runId", stale.ID, "age", time.Since(stale.StartedAt))
			_ = e.repo.UpdateRunStatus(ctx, stale.ID, ChatRunStatusFailed, "")
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
		return uuid.Nil, err
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
	// cleanupCtx: used for DB writes after ctx is cancelled — has tenant but no deadline.
	cleanupCtx := baseCtx

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
		e.repo.UpdateRunStatus(ctx, task.RunID, ChatRunStatusFailed, "")
		return
	}

	if session.AgentID == nil {
		// P-C292-1: mirror the SSE path — attempt to bind the default published agent
		// before giving up, so async runs on agent-less sessions behave identically.
		defaultID, err := e.repo.FindDefaultAgentID(ctx)
		if err != nil || defaultID == nil {
			slog.Error("chat: background run session has no agent and no default agent found", "runId", task.RunID, "err", err)
			e.repo.UpdateRunStatus(ctx, task.RunID, ChatRunStatusFailed, "")
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
			e.repo.UpdateRunStatus(ctx, task.RunID, ChatRunStatusFailed, "")
			return
		}
		session.AgentID = defaultID
		slog.Info("chat: background run bound default agent to session", "runId", task.RunID, "agentId", *defaultID)
	}

	// 2. Execute the run
	runEvents, err := e.runner.RunSession(ctx, RunInput{
		RunID:       task.RunID,
		SessionID:   task.SessionID,
		AgentID:     *session.AgentID,
		TenantID:    task.TenantID,
		UserMessage: task.Message,
	})

	if err != nil {
		slog.Error("chat: background run failed to start", "runId", task.RunID, "err", err)
		e.repo.UpdateRunStatus(ctx, task.RunID, ChatRunStatusFailed, "")
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
	for event := range runEvents {
		if event.Type == "error" {
			slog.Error("chat: background run error event", "runId", task.RunID, "data", string(event.Data))
			var errData struct {
				Code string `json:"code"`
			}
			if json.Unmarshal(event.Data, &errData) == nil {
				switch errData.Code {
				case "stream_consume", "llm_call":
					// Runner deferred cleanup will persist a user-facing message for these.
					runnerWroteUserMessage = true
				}
			}
		}
	}

	// P-C102-1: if the context expired (timeout), mark run failed and surface error to user.
	// Use cleanupCtx (has tenant, no deadline) so the DB writes succeed.
	// P-C106-1: skip the timeout message only when the runner already persisted one
	// (prevents two consecutive error messages in the chat history).
	if ctx.Err() != nil {
		slog.Warn("chat: background run timed out", "runId", task.RunID, "timeout", e.runTimeout)
		e.repo.UpdateRunStatus(cleanupCtx, task.RunID, ChatRunStatusFailed, "")
		if !runnerWroteUserMessage {
			_, _ = e.repo.CreateMessage(cleanupCtx, ChatMessage{
				SessionID:   task.SessionID,
				Role:        "assistant",
				Content:     "O agente excedeu o tempo máximo de resposta. Tente novamente ou verifique a configuração do modelo de IA.",
				MessageType: MessageTypeText,
			})
		}
		return
	}

	e.repo.MarkRunCompleted(ctx, task.RunID)
	slog.Info("chat: background run completed", "runId", task.RunID)
}

// friendlyStartupError maps errors that occur before the runner loop starts
// to user-facing messages suitable for persisting to chat history.
func friendlyStartupError(rawMsg string) string {
	if strings.Contains(rawMsg, "unsupported provider") || strings.Contains(rawMsg, "unknown provider") {
		return "Não foi possível iniciar o agente: o provedor de IA configurado não é suportado. Verifique a configuração do modelo nas configurações do agente."
	}
	if strings.Contains(rawMsg, "no AI provider configured") || strings.Contains(rawMsg, "api key") || strings.Contains(rawMsg, "API key") {
		return "Não foi possível iniciar o agente: chave de API não configurada. Verifique as configurações do provedor de IA."
	}
	if strings.Contains(rawMsg, "build model") {
		return "Não foi possível iniciar o agente: erro ao configurar o modelo de IA. Verifique o provedor e o modelo nas configurações do agente."
	}
	return "Não foi possível iniciar o agente. Verifique a configuração do modelo de IA nas configurações do agente."
}
