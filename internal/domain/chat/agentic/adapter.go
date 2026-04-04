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

// ProviderRegistry maps provider names (e.g. "anthropic", "openai") to their
// ChatModel implementations. The default provider is used when the agent has no
// provider configured or when the requested provider is not available.
type ProviderRegistry struct {
	providers map[string]ai.ChatModel
	def       ai.ChatModel
}

// NewProviderRegistry creates a registry with a default model and optional
// named overrides. The default is used as a fallback when an agent's provider
// is not present in the map.
func NewProviderRegistry(defaultModel ai.ChatModel, named map[string]ai.ChatModel) *ProviderRegistry {
	m := make(map[string]ai.ChatModel, len(named)+1)
	for k, v := range named {
		if v != nil {
			m[k] = v
		}
	}
	if defaultModel != nil {
		m[defaultModel.GetProviderName()] = defaultModel
	}
	return &ProviderRegistry{providers: m, def: defaultModel}
}

// Default returns the fallback ChatModel (nil when no providers are configured).
func (r *ProviderRegistry) Default() ai.ChatModel {
	return r.def
}

// Select returns the ChatModel for the given provider name.
// Falls back to the default model when the provider is empty or not registered.
func (r *ProviderRegistry) Select(provider string) ai.ChatModel {
	if provider != "" {
		if m, ok := r.providers[provider]; ok {
			return m
		}
	}
	return r.def
}

// SessionRunnerAdapter implements chat.SessionRunner by creating a Runner
// on-demand and bridging agentic.RunEvent → chat.RunEvent.
type SessionRunnerAdapter struct {
	registry     *ProviderRegistry
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
// to the agentic.Runner.
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
	return NewSessionRunnerAdapterWithRegistry(
		NewProviderRegistry(chatModel, nil),
		skillClient, prompt, tools, ctxManager, memory, hookExecutor, repo, agentLoader,
	)
}

// NewSessionRunnerAdapterWithRegistry creates an adapter that supports
// per-agent provider selection via the ProviderRegistry.
func NewSessionRunnerAdapterWithRegistry(
	registry *ProviderRegistry,
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
		registry:     registry,
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

// adapterRunnerFactory implements RunnerFactory using the adapter's dependencies.
type adapterRunnerFactory struct {
	adapter   *SessionRunnerAdapter
	chatModel ai.ChatModel
}

func (f *adapterRunnerFactory) NewRunner(config RunConfig) *Runner {
	// Sub-runners inherit the same provider selected for the parent session.
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
	subtaskExec := NewSubtaskExecutor(f)
	runner.WithSubtaskExecutor(subtaskExec)
	return runner
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

	// Select the ChatModel for this agent's configured provider.
	// Falls back to the registry default when no provider is set.
	chatModel := a.registry.Select(config.Provider)

	factory := &adapterRunnerFactory{adapter: a, chatModel: chatModel}
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
	subtaskExec := NewSubtaskExecutor(factory)
	runner.WithSubtaskExecutor(subtaskExec)

	agenticCh := runner.Run(ctx, RunInput{
		SessionID:       in.SessionID,
		AgentID:         in.AgentID,
		UserMessage:     in.UserMessage,
		SystemPrompt:    agentCfg.SystemPrompt,
		TenantID:        in.TenantID,
		PermissionRules: ParsePermissionRules(agentCfg.PermissionRules),
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
