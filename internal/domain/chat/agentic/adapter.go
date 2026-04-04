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

// ChatModelFactory builds a ChatModel for a given provider name.
// Implementations are responsible for loading credentials (e.g. from settings)
// and constructing the appropriate client. The context carries the tenant ID,
// so per-tenant settings can be fetched.
type ChatModelFactory interface {
	Build(ctx context.Context, provider string) (ai.ChatModel, error)
}

// SessionRunnerAdapter implements chat.SessionRunner by creating a Runner
// on-demand and bridging agentic.RunEvent → chat.RunEvent.
type SessionRunnerAdapter struct {
	modelFactory ChatModelFactory
	skillClient  *SkillRuntimeClient
	prompt       *PromptBuilder
	tools        *ToolSchemaBuilder
	ctxManager   *ContextManager
	memory       *MemoryBridge
	hookExecutor *HookExecutor
	repo         chat.Repository
	agentLoader  AgentConfigLoader
}

// NewSessionRunnerAdapter creates an adapter that wires the chat.Service
// to the agentic.Runner. The provided chatModel is used as a static fallback
// factory (all agents use the same provider). Prefer NewSessionRunnerAdapterWithFactory
// when per-agent provider selection from settings is needed.
func NewSessionRunnerAdapter(
	chatModel ai.ChatModel,
	skillClient *SkillRuntimeClient,
	prompt *PromptBuilder,
	tools *ToolSchemaBuilder,
	ctxManager *ContextManager,
	memory *MemoryBridge,
	hookExecutor *HookExecutor,
	repo chat.Repository,
	agentLoader AgentConfigLoader,
) *SessionRunnerAdapter {
	return NewSessionRunnerAdapterWithFactory(
		staticModelFactory{model: chatModel},
		skillClient, prompt, tools, ctxManager, memory, hookExecutor, repo, agentLoader,
	)
}

// NewSessionRunnerAdapterWithFactory creates an adapter that uses the provided
// ChatModelFactory to select the correct provider per agent at request time.
func NewSessionRunnerAdapterWithFactory(
	factory ChatModelFactory,
	skillClient *SkillRuntimeClient,
	prompt *PromptBuilder,
	tools *ToolSchemaBuilder,
	ctxManager *ContextManager,
	memory *MemoryBridge,
	hookExecutor *HookExecutor,
	repo chat.Repository,
	agentLoader AgentConfigLoader,
) *SessionRunnerAdapter {
	return &SessionRunnerAdapter{
		modelFactory: factory,
		skillClient:  skillClient,
		prompt:       prompt,
		tools:        tools,
		ctxManager:   ctxManager,
		memory:       memory,
		hookExecutor: hookExecutor,
		repo:         repo,
		agentLoader:  agentLoader,
	}
}

// staticModelFactory always returns the same ChatModel regardless of provider.
type staticModelFactory struct {
	model ai.ChatModel
}

func (f staticModelFactory) Build(_ context.Context, _ string) (ai.ChatModel, error) {
	if f.model == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}
	return f.model, nil
}

// adapterRunnerFactory implements RunnerFactory for sub-runner spawning.
type adapterRunnerFactory struct {
	adapter      *SessionRunnerAdapter
	chatModel    ai.ChatModel
	agentMailbox *AgentMailbox
}

func (f *adapterRunnerFactory) NewRunner(config RunConfig) *Runner {
	runner := NewRunner(
		f.chatModel,
		f.adapter.skillClient,
		f.adapter.prompt,
		f.adapter.tools,
		f.adapter.ctxManager,
		f.adapter.memory,
		f.adapter.repo,
		&repoHistoryLoader{repo: f.adapter.repo},
		f.adapter.hookExecutor,
		config,
	)
	runner.WithAgentMailbox(f.agentMailbox)
	subtaskExec := NewSubtaskExecutor(f)
	subtaskExec.WithAgentMailbox(f.agentMailbox)
	runner.WithSubtaskExecutor(subtaskExec)

	// Register memory as turn-end handler (decoupled from runner loop).
	if f.adapter.memory != nil {
		runner.WithTurnEndHandlers(NewMemoryTurnEndHandler(f.adapter.memory))
	}
	return runner
}

// RunSession implements chat.SessionRunner. It resolves the ChatModel for the
// agent's configured provider from the settings table, then runs the agentic loop.
func (a *SessionRunnerAdapter) RunSession(ctx context.Context, in chat.RunInput) (<-chan chat.RunEvent, error) {
	agentCfg, err := a.agentLoader.GetAgentForRun(ctx, in.AgentID)
	if err != nil {
		return nil, fmt.Errorf("session runner: load agent: %w", err)
	}

	config := RunConfigFromModelConfig(agentCfg.ModelConfig)

	// Resolve the ChatModel for this agent's provider from the factory.
	// The context carries the tenant ID so settings can be fetched per-tenant.
	chatModel, err := a.modelFactory.Build(ctx, config.Provider)
	if err != nil {
		return nil, fmt.Errorf("session runner: build model for provider %q: %w", config.Provider, err)
	}

	// Create a shared mailbox for inter-agent messaging within this run.
	agentMailbox := NewAgentMailbox()

	factory := &adapterRunnerFactory{adapter: a, chatModel: chatModel, agentMailbox: agentMailbox}
	runner := NewRunner(
		chatModel,
		a.skillClient,
		a.prompt,
		a.tools,
		a.ctxManager,
		a.memory,
		a.repo,
		&repoHistoryLoader{repo: a.repo},
		a.hookExecutor,
		config,
	)
	runner.WithAgentMailbox(agentMailbox)
	subtaskExec := NewSubtaskExecutor(factory)
	subtaskExec.WithAgentMailbox(agentMailbox)
	runner.WithSubtaskExecutor(subtaskExec)

	// Register memory as turn-end handler (decoupled from runner loop).
	if a.memory != nil {
		runner.WithTurnEndHandlers(NewMemoryTurnEndHandler(a.memory))
	}

	agenticCh := runner.Run(ctx, RunInput{
		SessionID:       in.SessionID,
		AgentID:         in.AgentID,
		UserMessage:     in.UserMessage,
		SystemPrompt:    agentCfg.SystemPrompt,
		TenantID:        in.TenantID,
		PermissionRules: ParsePermissionRules(agentCfg.PermissionRules),
	})

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
