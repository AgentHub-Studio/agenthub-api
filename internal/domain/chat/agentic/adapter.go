package agentic

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	"github.com/AgentHub-Studio/agenthub-go-commons/ai"
)

// AgentConfigLoader loads an agent's configuration from the database.
type AgentConfigLoader interface {
	GetAgentForRun(ctx context.Context, id uuid.UUID) (*chat.AgentRunConfig, error)
}

// SessionRunnerAdapter implements chat.SessionRunner by creating a Runner
// on-demand and bridging agentic.RunEvent → chat.RunEvent.
type SessionRunnerAdapter struct {
	chatModel   ai.ChatModel
	skillClient *SkillRuntimeClient
	prompt      *PromptBuilder
	tools       *ToolSchemaBuilder
	ctxManager  *ContextManager
	memory      *MemoryBridge
	repo        chat.Repository
	agentLoader AgentConfigLoader
}

// NewSessionRunnerAdapter creates an adapter that wires the chat.Service
// to the agentic.Runner.
func NewSessionRunnerAdapter(
	chatModel ai.ChatModel,
	skillClient *SkillRuntimeClient,
	prompt *PromptBuilder,
	tools *ToolSchemaBuilder,
	ctxManager *ContextManager,
	memory *MemoryBridge,
	repo chat.Repository,
	agentLoader AgentConfigLoader,
) *SessionRunnerAdapter {
	return &SessionRunnerAdapter{
		chatModel:   chatModel,
		skillClient: skillClient,
		prompt:      prompt,
		tools:       tools,
		ctxManager:  ctxManager,
		memory:      memory,
		repo:        repo,
		agentLoader: agentLoader,
	}
}

// RunSession implements chat.SessionRunner. It creates a Runner with the
// agent's model configuration and bridges agentic events to chat events.
func (a *SessionRunnerAdapter) RunSession(ctx context.Context, in chat.RunInput) (<-chan chat.RunEvent, error) {
	// Load agent config to determine model settings.
	agentCfg, err := a.agentLoader.GetAgentForRun(ctx, in.AgentID)
	if err != nil {
		return nil, fmt.Errorf("session runner: load agent: %w", err)
	}

	config := RunConfigFromModelConfig(agentCfg.ModelConfig)

	runner := NewRunner(
		a.chatModel,
		a.skillClient,
		a.prompt,
		a.tools,
		a.ctxManager,
		a.memory,
		a.repo, // MessagePersister — Repository implements CreateMessage
		&repoHistoryLoader{repo: a.repo},
		config,
	)

	agenticCh := runner.Run(ctx, RunInput{
		SessionID:    in.SessionID,
		AgentID:      in.AgentID,
		UserMessage:  in.UserMessage,
		SystemPrompt: agentCfg.SystemPrompt,
		TenantID:     in.TenantID,
	})

	// Bridge agentic.RunEvent → chat.RunEvent.
	chatCh := make(chan chat.RunEvent, config.StreamBufferSize)
	go func() {
		defer close(chatCh)
		for ev := range agenticCh {
			chatCh <- chat.RunEvent{
				Type: string(ev.Type),
				Data: ev.Data,
			}
		}
	}()

	return chatCh, nil
}

// repoHistoryLoader adapts chat.Repository to HistoryLoader.
type repoHistoryLoader struct {
	repo chat.Repository
}

func (l *repoHistoryLoader) FindAllMessages(ctx context.Context, sessionID uuid.UUID) ([]chat.ChatMessage, error) {
	return l.repo.FindAllMessages(ctx, sessionID)
}

