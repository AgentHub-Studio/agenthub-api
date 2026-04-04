package agentic

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

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
	// Build returns a ChatModel for the given provider. The model parameter
	// allows the factory to select the correct API variant (e.g. OpenAI Chat
	// Completions vs Responses API based on the model name).
	Build(ctx context.Context, provider, model string) (ai.ChatModel, error)
	// ResolveModel returns the default model name for the given provider
	// from the tenant's settings. Returns "" if not configured.
	ResolveModel(ctx context.Context, provider string) string
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

	// elicitation manages a registry of active ElicitationHandlers keyed by
	// session ID so that HTTP respond calls can be routed to the correct run.
	elicitation elicitationRegistry
}

// elicitationRegistry maps active session runs to their ElicitationHandlers.
type elicitationRegistry struct {
	mu      sync.Mutex
	byRunID map[string]*ElicitationHandler // runKey → handler
}

func (r *elicitationRegistry) register(runKey string, h *ElicitationHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byRunID == nil {
		r.byRunID = make(map[string]*ElicitationHandler)
	}
	r.byRunID[runKey] = h
}

func (r *elicitationRegistry) unregister(runKey string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byRunID, runKey)
}

// Respond routes a user response to the correct handler.
// Returns false when no active run is found for the given run key.
func (r *elicitationRegistry) Respond(runKey, requestID string, result ElicitationResult) bool {
	r.mu.Lock()
	h, ok := r.byRunID[runKey]
	r.mu.Unlock()
	if !ok {
		return false
	}
	return h.Respond(requestID, result)
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

func (f staticModelFactory) Build(_ context.Context, _, _ string) (ai.ChatModel, error) {
	if f.model == nil {
		return nil, fmt.Errorf("no AI provider configured")
	}
	return f.model, nil
}

func (f staticModelFactory) ResolveModel(_ context.Context, _ string) string { return "" }

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
	f.adapter.attachAuxiliaryComponents(runner, f, f.chatModel, config)

	// Register memory as turn-end handler (decoupled from runner loop).
	if f.adapter.memory != nil {
		runner.WithTurnEndHandlers(NewMemoryTurnEndHandler(f.adapter.memory))
	}
	return runner
}

// RunSession implements chat.SessionRunner. It resolves the ChatModel for the
// agent's configured provider from the settings table, then runs the agentic loop.
// If an ElicitationHandler is enqueued during the run, the adapter emits
// EventInputRequest events into the SSE stream and registers the handler so that
// HTTP respond calls (POST /elicitation/{requestId}/respond) can unblock the loop.
func (a *SessionRunnerAdapter) RunSession(ctx context.Context, in chat.RunInput) (<-chan chat.RunEvent, error) {
	agentCfg, err := a.agentLoader.GetAgentForRun(ctx, in.AgentID)
	if err != nil {
		return nil, fmt.Errorf("session runner: load agent: %w", err)
	}

	config := RunConfigFromModelConfig(agentCfg.ModelConfig)

	// If the agent doesn't specify a model, resolve it from the tenant's settings.
	if config.Model == "" || config.Model == DefaultRunConfig().Model {
		if settingsModel := a.modelFactory.ResolveModel(ctx, config.Provider); settingsModel != "" {
			config.Model = settingsModel
		}
	}

	// Resolve the ChatModel for this agent's provider from the factory.
	// The model name is passed so the factory can select the correct API
	// variant (e.g. OpenAI Responses API for gpt-5+ models).
	chatModel, err := a.modelFactory.Build(ctx, config.Provider, config.Model)
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
	a.attachAuxiliaryComponents(runner, factory, chatModel, config)

	// Register memory as turn-end handler (decoupled from runner loop).
	if a.memory != nil {
		runner.WithTurnEndHandlers(NewMemoryTurnEndHandler(a.memory))
	}

	// Create an ElicitationHandler for this run and wire the OnEnqueue callback
	// so that each new elicitation request is forwarded to the SSE stream as an
	// EventInputRequest event. The handler is registered in the adapter registry
	// so that HTTP respond calls can be routed back.
	elicHandler := NewElicitationHandler()
	runKey := in.SessionID.String()
	a.elicitation.register(runKey, elicHandler)

	// The bridgeCh receives elicitation events from OnEnqueue before agenticCh
	// is created. We use a buffered channel so the callback never blocks.
	elicEventCh := make(chan RunEvent, 8)
	elicHandler.OnEnqueue(func(req *ElicitationRequest) {
		payload := buildUiFormPayload(req.Params)
		data, _ := json.Marshal(InputRequestData{
			RequestID:  req.RequestID,
			ServerName: req.ServerName,
			Payload:    payload,
		})
		slog.Info("agentic: emitting input_request SSE event",
			"requestID", req.RequestID,
			"payloadSize", len(payload),
			"sessionID", in.SessionID,
		)
		elicEventCh <- NewRunEvent(EventInputRequest, json.RawMessage(data))
	})

	agenticCh := runner.Run(ctx, RunInput{
		SessionID:       in.SessionID,
		AgentID:         in.AgentID,
		UserMessage:     in.UserMessage,
		SystemPrompt:    agentCfg.SystemPrompt,
		TenantID:        in.TenantID,
		PermissionRules: ParsePermissionRules(agentCfg.PermissionRules),
		Elicitation:     elicHandler,
	})

	chatCh := make(chan chat.RunEvent, config.StreamBufferSize)
	go func() {
		defer func() {
			a.elicitation.unregister(runKey)
			close(chatCh)
		}()

		// Heartbeat ticker keeps the SSE connection alive while the runner is
		// blocked waiting for user input (elicitation). Without this, reverse
		// proxies (Traefik, nginx) or browsers may close the idle connection.
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()

		for {
			select {
			case ev, ok := <-agenticCh:
				if !ok {
					// Drain any remaining elicitation events before closing.
					for {
						select {
						case elicEv := <-elicEventCh:
							chatCh <- chat.RunEvent{Type: string(elicEv.Type), Data: elicEv.Data}
						default:
							return
						}
					}
				}
				chatCh <- chat.RunEvent{Type: string(ev.Type), Data: ev.Data}
			case elicEv := <-elicEventCh:
				chatCh <- chat.RunEvent{Type: string(elicEv.Type), Data: elicEv.Data}
			case t := <-heartbeat.C:
				hb, _ := json.Marshal(map[string]int64{"ts": t.Unix()})
				chatCh <- chat.RunEvent{Type: string(EventHeartbeat), Data: json.RawMessage(hb)}
			}
		}
	}()

	return chatCh, nil
}

// RespondElicitation routes a user's elicitation response to the active run
// for the given session. It accepts a chat.ElicitationResult (to satisfy the
// chat.ElicitationResponder interface without circular imports) and converts it
// to the agentic.ElicitationResult used by the handler queue.
// Returns false when no active run is found (session has already completed or
// the requestId does not exist).
func (a *SessionRunnerAdapter) RespondElicitation(sessionID, requestID string, result chat.ElicitationResult) bool {
	agResult := ElicitationResult{
		Action:  ElicitationAction(result.Action),
		Content: result.Content,
	}
	return a.elicitation.Respond(sessionID, requestID, agResult)
}

func (a *SessionRunnerAdapter) attachAuxiliaryComponents(
	runner *Runner,
	factory RunnerFactory,
	chatModel ai.ChatModel,
	config RunConfig,
) {
	if runner == nil || chatModel == nil {
		return
	}

	cacheSnap := &CacheSafeParamsSnapshot{}
	runner.WithCacheSafeParamsSnapshot(cacheSnap)

	summaryGen := NewToolUseSummaryGenerator(chatModel, config.Model)
	summaryGen.WithPromptTemplateResolver(a.prompt.tpl)
	runner.WithToolUseSummaryGenerator(summaryGen)

	memoryExtractor := NewSessionMemoryExtractor(
		NewForkedAgentRunner(factory, config, chatModel),
		DefaultSessionMemoryConfig(),
	).WithPromptTemplateResolver(a.prompt.tpl)
	runner.WithSessionMemoryExtractor(memoryExtractor)
}

// repoHistoryLoader adapts chat.Repository to HistoryLoader.
type repoHistoryLoader struct {
	repo chat.Repository
}

func (l *repoHistoryLoader) FindAllMessages(ctx context.Context, sessionID uuid.UUID) ([]chat.ChatMessage, error) {
	return l.repo.FindAllMessages(ctx, sessionID)
}
