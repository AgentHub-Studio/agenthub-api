package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

const ChatRunQueue = "chat.run.queue"

// ChatRunTask is the message payload for RabbitMQ.
type ChatRunTask struct {
	RunID     uuid.UUID `json:"runId"`
	SessionID uuid.UUID `json:"sessionId"`
	TenantID  string    `json:"tenantId"`
	Message   string    `json:"message"`
}

// AsyncExecutor handles asynchronous execution of chat runs via RabbitMQ.
type AsyncExecutor struct {
	repo    Repository
	runner  SessionRunner
	connURL string
}

func NewAsyncExecutor(repo Repository, runner SessionRunner, connURL string) *AsyncExecutor {
	return &AsyncExecutor{
		repo:    repo,
		runner:  runner,
		connURL: connURL,
	}
}

// EnqueueRun persists a new run and sends a task to RabbitMQ.
func (e *AsyncExecutor) EnqueueRun(ctx context.Context, sessionID uuid.UUID, tenantID, message string) (uuid.UUID, error) {
	// 1. Create Run in DB
	run, err := e.repo.CreateRun(ctx, ChatRun{
		SessionID: sessionID,
		TenantID:  tenantID,
		Status:    ChatRunStatusActive,
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
	ctx := context.Background()
	// Set tenant context for DB calls
	// (Needs tenant middleware logic or similar if not using a dedicated background ctx)

	slog.Info("chat: background run starting", "runId", task.RunID, "sessionId", task.SessionID)

	// Since we are in a background worker, we don't have the original SSE stream.
	// We just run the session and let the Runner handle persistence of messages.
	// The Runner uses the provided runID if available (needs update to Runner).

	// For now, we call the runner.
	events, err := e.runner.RunSession(ctx, RunInput{
		SessionID:   task.SessionID,
		TenantID:    task.TenantID,
		UserMessage: task.Message,
		// runID: task.RunID, // Needs to be passed down
	})

	if err != nil {
		slog.Error("chat: background run failed to start", "runId", task.RunID, "err", err)
		e.repo.UpdateRunStatus(ctx, task.RunID, ChatRunStatusFailed, "")
		return
	}

	// Drain events to allow the runner to finish and persist messages.
	// Events are also buffered in memory (RunEventBufferRegistry) for SSE Resume.
	for event := range events {
		// We could publish events back to RabbitMQ for real-time notifications
		// or just let them be buffered for polling/resume.
		_ = event
	}

	e.repo.MarkRunCompleted(ctx, task.RunID)
	slog.Info("chat: background run completed", "runId", task.RunID)
}
