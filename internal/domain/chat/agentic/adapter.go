package agentic

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	chatTask "github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/task"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledge"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/mcp"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
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
	// ResolveDefaultProvider returns the tenant's configured default LLM provider
	// from the "llm.defaultProvider" setting. Returns "" if not configured.
	ResolveDefaultProvider(ctx context.Context) string
}

type toolSuspendStateStore interface {
	CreateToolSuspendState(ctx context.Context, sessionID uuid.UUID, requestID, toolName string, payload json.RawMessage) error
	ResolveToolSuspendState(ctx context.Context, sessionID uuid.UUID, requestID string, result chat.ElicitationResult) (bool, error)
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
	agentRepo    agent.Repository
	agentSvc     agent.Service
	skillRepo    *skill.Repository
	toolRepo     *tool.Repository
	integRepo    *integration.Service
	mcpRepo      mcp.Repository
	skillManager *skill.Service
	toolManager  *tool.Service
	mcpManager   *mcp.Service
	mcpClient    MCPClientService
	docSearch    knowledge.DocumentSearchClient // P-E1-2: wired when embedding service is available
	audit        ManagementAuditRecorder

	// llmCallTimeout overrides the default per-LLM-call timeout set by DefaultRunConfig.
	// P-C102-1: sourced from LLM_CALL_TIMEOUT_SECS env var at server startup.
	// Zero means "use the default from DefaultRunConfig".
	llmCallTimeout time.Duration

	// elicitation manages a registry of active ElicitationHandlers keyed by
	// session ID so that HTTP respond calls can be routed to the correct run.
	elicitation elicitationRegistry

	// clientState holds CopilotKit client declarations (frontend actions and
	// readables) and routes action results back to the active run. Lazily
	// created — nil until [WithClientStateStore] is called or [ClientStateStore]
	// is accessed.
	clientState *ClientStateStore

	// permAudit, when set, records permission decisions for every tool call.
	permAudit PermissionAuditLogger
	// taskRepo persists delegated sub-agent work for the parent session.
	taskRepo chatTask.Repository
}

// WithClientStateStore wires a [ClientStateStore] for CopilotKit Phase 1
// frontend actions. When unset, the adapter falls back to a default in-memory
// store (TTL 6h) created lazily via [SessionRunnerAdapter.ClientStateStore].
func (a *SessionRunnerAdapter) WithClientStateStore(store *ClientStateStore) *SessionRunnerAdapter {
	a.clientState = store
	return a
}

// ClientStateStore returns the active store, creating a default one on first
// access. Safe for concurrent use after the adapter is constructed.
func (a *SessionRunnerAdapter) ClientStateStore() *ClientStateStore {
	if a.clientState == nil {
		a.clientState = NewClientStateStore(6 * time.Hour)
	}
	return a.clientState
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

// Pending returns all unresolved requests for the given run key (sessionID).
// Returns nil when no active run is found.
func (r *elicitationRegistry) Pending(runKey string) []*ElicitationRequest {
	r.mu.Lock()
	h, ok := r.byRunID[runKey]
	r.mu.Unlock()
	if !ok {
		return nil
	}
	return h.Pending()
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
	agentRepo agent.Repository,
	skillRepo *skill.Repository,
	toolRepo *tool.Repository,
	integRepo *integration.Service,
	mcpRepo mcp.Repository,
) *SessionRunnerAdapter {
	return NewSessionRunnerAdapterWithFactory(
		staticModelFactory{model: chatModel},
		skillClient, prompt, tools, ctxManager, memory, hookExecutor, repo, agentLoader,
		agentRepo, skillRepo, toolRepo, integRepo, mcpRepo,
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
	agentRepo agent.Repository,
	skillRepo *skill.Repository,
	toolRepo *tool.Repository,
	integRepo *integration.Service,
	mcpRepo mcp.Repository,
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
		agentRepo:    agentRepo,
		agentSvc:     nil,
		skillRepo:    skillRepo,
		toolRepo:     toolRepo,
		integRepo:    integRepo,
		mcpRepo:      mcpRepo,
	}
}

// WithMCPClient wires the runtime-backed MCP client into runners spawned by the adapter.
func (a *SessionRunnerAdapter) WithMCPClient(client MCPClientService) *SessionRunnerAdapter {
	a.mcpClient = client
	return a
}

// WithLLMCallTimeout sets the server-level per-LLM-call timeout.
// P-C102-1: sourced from LLM_CALL_TIMEOUT_SECS at startup.
// Zero means "keep the RunConfig default (5 minutes)".
func (a *SessionRunnerAdapter) WithLLMCallTimeout(d time.Duration) *SessionRunnerAdapter {
	a.llmCallTimeout = d
	return a
}

// WithPermissionAuditLogger wires a permission audit logger so every permission
// decision during a run is persisted to the permission_audit_log table.
func (a *SessionRunnerAdapter) WithPermissionAuditLogger(logger PermissionAuditLogger) *SessionRunnerAdapter {
	a.permAudit = logger
	return a
}

// WithTaskRepository enables persistence of delegated sub-agent task lifecycles.
func (a *SessionRunnerAdapter) WithTaskRepository(repo chatTask.Repository) *SessionRunnerAdapter {
	a.taskRepo = repo
	return a
}

// WithDocumentSearchClient wires the document search client so the document_search
// builtin tool executes locally via pgvector instead of failing with "not available".
// P-E1-2: call this when the embedding service URL is configured.
func (a *SessionRunnerAdapter) WithDocumentSearchClient(client knowledge.DocumentSearchClient) *SessionRunnerAdapter {
	a.docSearch = client
	return a
}

// WithMemoryBridge wires the memory bridge so the memory_store builtin tool
// can persist and recall memories across sessions.
func (a *SessionRunnerAdapter) WithMemoryBridge(bridge *MemoryBridge) *SessionRunnerAdapter {
	a.memory = bridge
	return a
}

// WithAgentService wires an agent service used by management operations.
// Management deletes go through this service to keep audit logging consistent
// with other write paths.
func (a *SessionRunnerAdapter) WithAgentService(svc agent.Service) *SessionRunnerAdapter {
	a.agentSvc = svc
	return a
}

// WithManagementServices wires the same configured services used by the public
// API into agenthub_manage, keeping its mutations on the canonical boundaries.
func (a *SessionRunnerAdapter) WithManagementServices(skillSvc *skill.Service, toolSvc *tool.Service, mcpSvc *mcp.Service) *SessionRunnerAdapter {
	a.skillManager = skillSvc
	a.toolManager = toolSvc
	a.mcpManager = mcpSvc
	return a
}

// WithManagementAuditRecorder wires audit logging for destructive management operations.
func (a *SessionRunnerAdapter) WithManagementAuditRecorder(recorder ManagementAuditRecorder) *SessionRunnerAdapter {
	a.audit = recorder
	return a
}

func (a *SessionRunnerAdapter) newManagementExecutor() *ManagementExecutor {
	skillManager := a.skillManager
	if skillManager == nil && a.skillRepo != nil {
		skillManager = skill.NewService(a.skillRepo)
	}
	toolManager := a.toolManager
	if toolManager == nil && a.toolRepo != nil {
		toolManager = tool.NewService(a.toolRepo)
	}
	mcpManager := a.mcpManager
	if mcpManager == nil && a.mcpRepo != nil {
		mcpManager = mcp.NewService(a.mcpRepo)
	}
	return NewManagementExecutor(
		a.agentSvc,
		a.agentRepo,
		a.skillRepo,
		skillManager,
		a.toolRepo,
		a.mcpRepo,
	).WithToolManager(toolManager).WithMCPManager(mcpManager).WithAuditRecorder(a.audit)
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
func (f staticModelFactory) ResolveDefaultProvider(_ context.Context) string { return "" }

func (a *SessionRunnerAdapter) withFallbackModels(ctx context.Context, config RunConfig, primary ai.ChatModel) (ai.ChatModel, error) {
	if len(config.ModelFallbackChain) == 0 || primary == nil {
		return primary, nil
	}
	router := &fallbackRoutingModel{
		primary: primary,
		byModel: map[string]ai.ChatModel{
			config.Model: primary,
		},
	}
	for _, step := range config.ModelFallbackChain {
		if step.Model == "" {
			continue
		}
		provider := step.Provider
		if provider == "" {
			provider = config.Provider
		}
		if provider == "" || provider == config.Provider {
			router.byModel[step.Model] = primary
			continue
		}
		model, err := a.modelFactory.Build(ctx, provider, step.Model)
		if err != nil {
			return nil, fmt.Errorf("session runner: build fallback model for provider %q: %w", provider, err)
		}
		router.byModel[step.Model] = model
	}
	if len(router.byModel) <= 1 {
		return primary, nil
	}
	return router, nil
}

type fallbackRoutingModel struct {
	primary ai.ChatModel
	byModel map[string]ai.ChatModel
}

func (m *fallbackRoutingModel) Chat(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (*ai.ChatResponse, error) {
	return m.modelFor(opts.Model).Chat(ctx, messages, opts)
}

func (m *fallbackRoutingModel) ChatStream(ctx context.Context, messages []ai.Message, opts ai.ChatOptions) (<-chan ai.StreamChunk, error) {
	return m.modelFor(opts.Model).ChatStream(ctx, messages, opts)
}

func (m *fallbackRoutingModel) GetProviderName() string {
	return m.primary.GetProviderName()
}

func (m *fallbackRoutingModel) modelFor(model string) ai.ChatModel {
	if m != nil && model != "" {
		if selected, ok := m.byModel[model]; ok && selected != nil {
			return selected
		}
	}
	return m.primary
}

// nopPersister is a MessagePersister that accepts writes without hitting the
// database. It is used for sub-agent (subtask) runners whose sessions are
// ephemeral and never inserted into the chat_session table, preventing FK
// violations on chat_message.session_id_fkey.
type nopPersister struct{}

func (nopPersister) CreateMessage(_ context.Context, msg chat.ChatMessage) (chat.ChatMessage, error) {
	return msg, nil
}

// adapterRunnerFactory implements RunnerFactory for sub-runner spawning.
type adapterRunnerFactory struct {
	adapter      *SessionRunnerAdapter
	chatModel    ai.ChatModel
	agentMailbox *AgentMailbox
	coordinator  *CoordinatorState
}

func (f *adapterRunnerFactory) NewRunner(config RunConfig) *Runner {
	runner := NewRunner(
		f.chatModel,
		f.adapter.skillClient,
		f.adapter.prompt,
		f.adapter.tools,
		f.adapter.ctxManager,
		f.adapter.memory,
		nopPersister{}, // sub-sessions are ephemeral — skip DB persistence to avoid FK violations
		&repoHistoryLoader{repo: f.adapter.repo},
		f.adapter.hookExecutor,
		config,
	)
	if f.adapter.agentRepo != nil {
		managementExec := f.adapter.newManagementExecutor()
		runner.WithManagementExecutor(managementExec)
	}
	if f.adapter.mcpClient != nil {
		runner.WithMCPClient(f.adapter.mcpClient)
	}
	if f.adapter.docSearch != nil {
		runner.WithDocumentSearch(f.adapter.docSearch, nil)
	}
	runner.WithAgentMailbox(f.agentMailbox)
	subtaskExec := NewSubtaskExecutor(f)
	subtaskExec.WithAgentMailbox(f.agentMailbox)
	subtaskExec.WithCoordinatorState(f.coordinator)
	runner.WithSubtaskExecutor(subtaskExec)
	f.adapter.attachAuxiliaryComponents(runner, f, f.chatModel, config)

	// Register memory as turn-end handler (decoupled from runner loop).
	if f.adapter.memory != nil {
		runner.WithTurnEndHandlers(NewMemoryTurnEndHandler(f.adapter.memory))
	}
	return runner
}

// resolveRunConfig builds the effective RunConfig for a run: it overlays the
// agent's model_config on the compiled defaults, then fills in an empty
// provider/model from the tenant's settings via the factory. Extracted from
// RunSession so the resolution chain — critical for fresh tenants whose
// auto-seeded default agent ships with an empty provider/model — is unit-testable.
func resolveRunConfig(ctx context.Context, factory ChatModelFactory, modelConfig json.RawMessage, llmCallTimeout time.Duration) RunConfig {
	config := RunConfigFromModelConfig(modelConfig)
	defaultCfg := DefaultRunConfig()

	// P-C102-1: apply the server-level LLM call timeout when the agent's
	// model_config did not explicitly override it (still equals the default).
	if llmCallTimeout > 0 && config.LLMCallTimeout == defaultCfg.LLMCallTimeout {
		config.LLMCallTimeout = llmCallTimeout
	}

	// When the agent has no explicit provider, use the tenant's configured
	// default. This avoids hard-coding the fallback to "anthropic" when the
	// tenant is on a different provider (P-C75-1).
	if config.Provider == "" || config.Provider == defaultCfg.Provider {
		if defaultProvider := factory.ResolveDefaultProvider(ctx); defaultProvider != "" {
			config.Provider = defaultProvider
		}
	}

	// When the agent has no explicit model, resolve it from the tenant's settings.
	if config.Model == "" || config.Model == defaultCfg.Model {
		if settingsModel := factory.ResolveModel(ctx, config.Provider); settingsModel != "" {
			config.Model = settingsModel
		}
	}

	return config
}

// RunSession implements chat.SessionRunner. It resolves the ChatModel for the
// agent's configured provider from the settings table, then runs the agentic loop.
// If an ElicitationHandler is enqueued during the run, the adapter emits
// EventInputRequest events into the SSE stream and registers the handler so that
// HTTP respond calls (POST /elicitation/{requestId}/respond) can unblock the loop.
func (a *SessionRunnerAdapter) RunSession(ctx context.Context, in chat.RunInput) (<-chan chat.RunEvent, error) {
	// P-C343-1: invalidate cached prompt sections for this agent at the start of
	// every run so that edits to skills, tools, or KBs are reflected without a
	// server restart. The cache is per-agent-ID, so other agents are unaffected.
	if a.prompt != nil {
		a.prompt.ClearCacheForAgent(in.AgentID)
	}

	agentCfg := in.AgentConfig
	if agentCfg == nil || agentCfg.ID != in.AgentID {
		var err error
		agentCfg, err = a.agentLoader.GetAgentForRun(ctx, in.AgentID)
		if err != nil {
			return nil, fmt.Errorf("session runner: load agent: %w", err)
		}
	}

	// P-C178-1: reject runs for agents that are not PUBLISHED.
	switch agentCfg.Status {
	case string(agent.StatusDraft):
		return nil, fmt.Errorf("%w: agent %s is in DRAFT status", chat.ErrAgentNotPublished, in.AgentID)
	case string(agent.StatusArchived):
		return nil, fmt.Errorf("%w: agent %s", chat.ErrAgentArchived, in.AgentID)
	case string(agent.StatusPublished), "": // empty = legacy records without status
		// OK — proceed
	}

	effectiveModelConfig := resolveModelConfig(in, agentCfg)
	config := resolveRunConfig(ctx, a.modelFactory, effectiveModelConfig, a.llmCallTimeout)

	// Resolve the ChatModel for this agent's provider from the factory.
	// The model name is passed so the factory can select the correct API
	// variant (e.g. OpenAI Responses API for gpt-5+ models).
	chatModel, err := a.modelFactory.Build(ctx, config.Provider, config.Model)
	if err != nil {
		return nil, fmt.Errorf("session runner: build model for provider %q: %w", config.Provider, err)
	}
	chatModel, err = a.withFallbackModels(ctx, config, chatModel)
	if err != nil {
		return nil, err
	}
	// P-I1-1: log the effective model/provider at run start so operators can verify
	// which model is executing without querying the DB.
	slog.Info("agentic: run started", "model", config.Model, "provider", config.Provider, "agentID", in.AgentID)

	// Create a shared mailbox for inter-agent messaging within this run.
	agentMailbox := NewAgentMailbox()
	var coordinator *CoordinatorState
	if a.taskRepo != nil {
		coordinator = NewCoordinatorState().
			WithRepository(a.taskRepo, in.SessionID).
			WithContext(ctx)
	}

	factory := &adapterRunnerFactory{
		adapter:      a,
		chatModel:    chatModel,
		agentMailbox: agentMailbox,
		coordinator:  coordinator,
	}
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
	// P-C325-2: wire metadata persister so run metrics are recorded on completion.
	runner.WithMetadataPersister(a.repo)
	if a.agentRepo != nil {
		managementExec := a.newManagementExecutor()
		runner.WithManagementExecutor(managementExec)
	}
	if a.mcpClient != nil {
		runner.WithMCPClient(a.mcpClient)
	}
	// P-E1-2: wire document search client so document_search builtin tool executes
	// locally via pgvector when an embedding service is available.
	if a.docSearch != nil {
		runner.WithDocumentSearch(a.docSearch, nil) // kbIDs=nil → search all active KBs
	}
	runner.WithAgentMailbox(agentMailbox)
	subtaskExec := NewSubtaskExecutor(factory)
	subtaskExec.WithAgentMailbox(agentMailbox)
	subtaskExec.WithCoordinatorState(coordinator)
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

	// CopilotKit Phase 1: attach a FrontendActionHandler for this run. The store
	// already holds the client-declared actions/readables (posted before the run
	// started) and now routes any incoming action results to the handler that
	// will block in Submit() while the LLM waits.
	store := a.ClientStateStore()
	frontendHandler := NewFrontendActionHandler()
	store.AttachHandler(in.SessionID, frontendHandler)

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
		if store, ok := a.repo.(toolSuspendStateStore); ok {
			if err := store.CreateToolSuspendState(ctx, in.SessionID, req.RequestID, "ask_user", json.RawMessage(data)); err != nil {
				slog.Warn("agentic: failed to persist suspended tool state",
					"requestID", req.RequestID,
					"sessionID", in.SessionID,
					"error", err,
				)
			}
		}
		slog.Info("agentic: emitting input_request SSE event",
			"requestID", req.RequestID,
			"payloadSize", len(payload),
			"sessionID", in.SessionID,
		)
		elicEventCh <- NewRunEvent(EventInputRequest, json.RawMessage(data))
	})

	// P-C115-1: use session snapshot when available to preserve persona consistency.
	effectiveSystemPrompt := resolveSystemPrompt(in, agentCfg)
	requestContext := requestContextFromContext(ctx, in.TenantID)
	outputProcessors := in.OutputProcessors
	if len(outputProcessors) == 0 {
		outputProcessors = agentCfg.OutputProcessors
	}

	// P-C173-1 / RT-01: detect agent config changes between turns and persist a
	// system notification so the LLM is aware the session remains pinned to its
	// snapshot. ConfigHash remains modelConfig-specific for backward compatibility;
	// the SSE payload uses the canonical agent snapshot hash.
	currentHash := hashConfig(agentCfg.ModelConfig)
	currentSnapshotHash := hashAgentSnapshot(agentCfg)
	var configChangedEvent *RunEvent
	if session, err := a.repo.GetSessionByID(ctx, in.SessionID); err == nil {
		if detectAgentConfigChange(session, agentCfg) {
			ev := NewRunEvent(EventConfigChanged, ConfigChangedData{
				OldPersona:      "redacted",
				NewSnapshotHash: currentSnapshotHash,
			})
			configChangedEvent = &ev
			notif := chat.ChatMessage{
				SessionID:   in.SessionID,
				Role:        "system",
				Content:     "[system] Agent configuration changed after this session snapshot was created. This run continues with the session snapshot; start a new session to use the updated agent configuration.",
				MessageType: chat.MessageTypeSystem,
			}
			if _, msgErr := a.repo.CreateMessage(ctx, notif); msgErr != nil {
				slog.Warn("agentic: failed to persist config-change notification", "error", msgErr)
			}
		}
	}

	agenticCh := runner.Run(ctx, RunInput{
		RunID:                  in.RunID,
		SessionID:              in.SessionID,
		AgentID:                in.AgentID,
		UserMessage:            in.UserMessage,
		UserMessageID:          in.UserMessageID,
		SystemPrompt:           effectiveSystemPrompt,
		TenantID:               in.TenantID,
		PermissionRules:        ParsePermissionRules(agentCfg.PermissionRules),
		RequestContext:         requestContext,
		Elicitation:            elicHandler,
		FrontendActions:        store,                     // CopilotKit Phase 1 (ClientStateStore satisfies FrontendActionsProvider)
		IsAdmin:                callerHasAdminRole(ctx),   // P-C298-1
		EnableManagement:       agentCfg.EnableManagement, // P-C184-2
		DisableAskUser:         agentCfg.DisableAskUser,
		DisableAgentDelegation: agentCfg.DisableAgentDelegation,
		SkillIDsSnapshot:       in.SkillIDsSnapshot,       // P-C115-1: use snapshot if available
		MCPServerNamesSnapshot: in.MCPServerNamesSnapshot, // P-C253-1: filter MCP tools by bound servers
		OutputProcessors:       append([]string(nil), outputProcessors...),
		PermissionAudit:        a.permAudit,
	})

	chatCh := make(chan chat.RunEvent, config.StreamBufferSize)
	go func() {
		defer func() {
			a.elicitation.unregister(runKey)
			store.DetachHandler(in.SessionID)
			// P-C173-1: persist the current config hash so the next run can detect changes.
			if currentHash != "" {
				if hashErr := a.repo.UpdateSessionConfigHash(ctx, in.SessionID, currentHash); hashErr != nil {
					slog.Warn("agentic: failed to update session config hash", "error", hashErr)
				}
			}
			close(chatCh)
		}()

		// Heartbeat ticker keeps the SSE connection alive while the runner is
		// blocked waiting for user input (elicitation). Without this, reverse
		// proxies (Traefik, nginx) or browsers may close the idle connection.
		heartbeat := time.NewTicker(15 * time.Second)
		defer heartbeat.Stop()

		if configChangedEvent != nil {
			chatCh <- chat.RunEvent{Type: string(configChangedEvent.Type), Data: configChangedEvent.Data}
		}

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

// EffectivePrompt renders the session's agent prompt using the same dynamic
// RequestContext rules as the runner, but without starting an LLM run.
func (a *SessionRunnerAdapter) EffectivePrompt(ctx context.Context, sessionID uuid.UUID, identity chat.PromptIdentity) (chat.EffectivePromptResponse, error) {
	session, err := a.repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return chat.EffectivePromptResponse{}, err
	}
	if session.AgentID == nil {
		return chat.EffectivePromptResponse{}, chat.ErrNoAgentAvailable
	}
	agentCfg, err := a.agentLoader.GetAgentForRun(ctx, *session.AgentID)
	if err != nil {
		return chat.EffectivePromptResponse{}, err
	}
	rawPrompt := resolveSystemPrompt(chat.RunInput{SystemPromptSnapshot: session.SystemPromptSnapshot}, agentCfg)
	systemPrompt, warnings := ResolveSystemPromptPlaceholdersWithWarnings(rawPrompt, requestContextFromPromptIdentity(identity))
	if warnings == nil {
		warnings = []string{}
	}
	return chat.EffectivePromptResponse{
		SessionID:    session.ID,
		AgentID:      session.AgentID,
		SystemPrompt: systemPrompt,
		Warnings:     warnings,
	}, nil
}

// EffectiveTools renders the session's callable tools for the current request
// identity without starting an LLM run.
func (a *SessionRunnerAdapter) EffectiveTools(ctx context.Context, sessionID uuid.UUID, identity chat.PromptIdentity) (chat.EffectiveToolsResponse, error) {
	session, err := a.repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return chat.EffectiveToolsResponse{}, err
	}
	if session.AgentID == nil {
		return chat.EffectiveToolsResponse{}, chat.ErrNoAgentAvailable
	}
	agentCfg, err := a.agentLoader.GetAgentForRun(ctx, *session.AgentID)
	if err != nil {
		return chat.EffectiveToolsResponse{}, err
	}
	toolBuilder := a.tools.Clone()
	if toolBuilder == nil {
		return chat.EffectiveToolsResponse{}, fmt.Errorf("effective tools: tool builder not configured")
	}
	if a.mcpClient != nil {
		bridge := NewMCPToolBridge(a.mcpClient, identity.TenantID)
		// Keep the same nil-versus-empty binding contract as Runner: nil means
		// no MCP bindings are configured, while an empty non-nil slice means
		// bindings exist but none are enabled.
		if agentCfg.MCPServerNames != nil {
			bridge.WithAllowedServerNames(agentCfg.MCPServerNames)
		}
		toolBuilder.WithMCPBridge(bridge)
	}
	skillIDsSnapshot := skillIDsFromSessionSnapshot(session.SkillBindingsSnapshot)
	if len(skillIDsSnapshot) > 0 {
		toolBuilder.WithSkillIDsSnapshot(skillIDsSnapshot)
	}
	toolBuilder.
		WithDepthLimits(0, 3).
		WithAdminScope(roleListHas(identity.Roles, "admin")).
		WithEnableManagement(agentCfg.EnableManagement).
		WithDisableAskUser(agentCfg.DisableAskUser).
		WithDisableAgentDelegation(agentCfg.DisableAgentDelegation).
		WithRequestRoles(identity.Roles)

	result, err := toolBuilder.BuildWithDeferred(ctx, *session.AgentID)
	if err != nil {
		return chat.EffectiveToolsResponse{}, err
	}
	deferred := make(map[string]struct{}, len(result.Deferred))
	for _, tool := range result.Deferred {
		deferred[tool.Name] = struct{}{}
	}
	tools := make([]chat.EffectiveToolResponse, 0, len(result.All))
	for _, tool := range result.All {
		_, isDeferred := deferred[tool.Name]
		tools = append(tools, chat.EffectiveToolResponse{
			Name:        tool.Name,
			Description: tool.Description,
			Builtin:     tool.Builtin,
			SkillSlug:   tool.SkillSlug,
			Deferred:    isDeferred,
		})
	}
	warnings := result.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return chat.EffectiveToolsResponse{
		SessionID: session.ID,
		AgentID:   session.AgentID,
		Tools:     tools,
		Warnings:  warnings,
	}, nil
}

func skillIDsFromSessionSnapshot(raw json.RawMessage) []uuid.UUID {
	if len(raw) <= 2 {
		return nil
	}
	var snap chat.SkillBindingsSnapshotData
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil
	}
	return snap.SkillIDs
}

func roleListHas(roles []string, want string) bool {
	for _, role := range roles {
		if strings.TrimSpace(role) == want {
			return true
		}
	}
	return false
}

// RespondElicitation routes a user's elicitation response to the active run
// for the given session. It accepts a chat.ElicitationResult (to satisfy the
// chat.ElicitationResponder interface without circular imports) and converts it
// to the agentic.ElicitationResult used by the handler queue.
// Returns false when no active run is found (session has already completed or
// the requestId does not exist).
func (a *SessionRunnerAdapter) RespondElicitation(ctx context.Context, sessionID, requestID string, result chat.ElicitationResult) bool {
	agResult := ElicitationResult{
		Action:  ElicitationAction(result.Action),
		Content: result.Content,
	}
	responded := a.elicitation.Respond(sessionID, requestID, agResult)
	parsedSessionID, parseErr := uuid.Parse(sessionID)
	if parseErr == nil {
		if store, ok := a.repo.(toolSuspendStateStore); ok {
			resolved, err := store.ResolveToolSuspendState(ctx, parsedSessionID, requestID, result)
			if err != nil {
				slog.Warn("agentic: failed to resolve suspended tool state",
					"requestID", requestID,
					"sessionID", sessionID,
					"error", err,
				)
			}
			responded = responded || resolved
		}
	}
	return responded
}

// ApplyClientState merges a CopilotKit client-state patch into the per-session
// store. Action results are dispatched to the active run (if any); declared
// actions and readables persist in memory until the next run picks them up.
// Implements [chat.ClientStateApplier] without exposing the agentic types.
func (a *SessionRunnerAdapter) ApplyClientState(sessionID uuid.UUID, patch chat.ClientStatePatch) {
	agPatch := ClientStatePatch{}
	if patch.FrontendActions != nil {
		agPatch.FrontendActions = make([]FrontendAction, 0, len(patch.FrontendActions))
		for _, fa := range patch.FrontendActions {
			agPatch.FrontendActions = append(agPatch.FrontendActions, FrontendAction{
				Name:        fa.Name,
				Description: fa.Description,
				Parameters:  fa.Parameters,
			})
		}
	}
	if patch.Readables != nil {
		agPatch.Readables = make([]Readable, 0, len(patch.Readables))
		for _, r := range patch.Readables {
			agPatch.Readables = append(agPatch.Readables, Readable{
				ID:          r.ID,
				Description: r.Description,
				Value:       r.Value,
				ParentID:    r.ParentID,
			})
		}
	}
	if patch.ActionResults != nil {
		agPatch.ActionResults = make([]FrontendActionResult, 0, len(patch.ActionResults))
		for _, ar := range patch.ActionResults {
			agPatch.ActionResults = append(agPatch.ActionResults, FrontendActionResult{
				ID:     ar.ID,
				Status: ar.Status,
				Result: ar.Result,
				Error:  ar.Error,
			})
		}
	}
	a.ClientStateStore().Apply(sessionID, agPatch)
}

// GetPendingElicitations returns unresolved ask_user requests for the given session.
// P-C101-1: enables async (RabbitMQ) callers to discover pending elicitations by polling,
// since the SSE input_request event may have been drained by the background worker.
func (a *SessionRunnerAdapter) GetPendingElicitations(sessionID string) []chat.PendingElicitationInfo {
	reqs := a.elicitation.Pending(sessionID)
	if len(reqs) == 0 {
		return nil
	}
	result := make([]chat.PendingElicitationInfo, 0, len(reqs))
	for _, req := range reqs {
		result = append(result, chat.PendingElicitationInfo{
			RequestID: req.RequestID,
			Payload:   buildUiFormPayload(req.Params),
			CreatedAt: req.CreatedAt,
		})
	}
	return result
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

// callerHasAdminRole parses the raw JWT from the context and returns true when
// the caller has the "admin" role in either realm_access.roles or any
// resource_access.{client}.roles claim.
// P-C298-1: used to gate the agenthub_manage builtin tool to admin callers.
// Parsing is without signature verification — the token is already validated
// by the auth middleware before reaching this point.
func callerHasAdminRole(ctx context.Context) bool {
	claims, ok := requestClaimsFromContext(ctx)
	if !ok {
		return false
	}
	// Check realm-level roles first.
	for _, r := range claims.RealmAccess.Roles {
		if r == "admin" {
			return true
		}
	}
	// Check resource-level roles (e.g. resource_access.agenthub-frontend.roles).
	for _, access := range claims.ResourceAccess {
		for _, r := range access.Roles {
			if r == "admin" {
				return true
			}
		}
	}
	return false
}

type requestClaims struct {
	Subject     string `json:"sub"`
	Email       string `json:"email"`
	Username    string `json:"preferred_username"`
	RealmAccess struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
	// Keycloak may place roles in resource_access.<clientId>.roles.
	ResourceAccess map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`
}

func requestClaimsFromContext(ctx context.Context) (requestClaims, bool) {
	raw := tenant.TokenFromContext(ctx)
	if raw == "" {
		return requestClaims{}, false
	}
	// JWT = header.payload.signature — parse only the payload segment. Signature
	// verification already happened in auth middleware.
	parts := strings.SplitN(raw, ".", 3)
	if len(parts) != 3 {
		return requestClaims{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return requestClaims{}, false
	}
	var claims requestClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return requestClaims{}, false
	}
	return claims, true
}

func requestContextFromContext(ctx context.Context, tenantID string) RequestContext {
	claims, _ := requestClaimsFromContext(ctx)
	if tenantID == "" {
		tenantID = tenant.FromContext(ctx)
	}
	return requestContextFromPromptIdentity(chat.PromptIdentity{
		UserID:     claims.Subject,
		UserEmail:  claims.Email,
		Username:   claims.Username,
		Roles:      rolesFromRequestClaims(claims),
		TenantID:   tenantID,
		TenantName: tenantID,
	})
}

func requestContextFromPromptIdentity(identity chat.PromptIdentity) RequestContext {
	userEmail := identity.UserEmail
	if userEmail == "" {
		userEmail = identity.Username
	}
	if userEmail == "" {
		userEmail = identity.UserID
	}
	tenantName := identity.TenantName
	if tenantName == "" {
		tenantName = identity.TenantID
	}
	return RequestContext{
		UserID:     identity.UserID,
		UserEmail:  userEmail,
		UserRoles:  append([]string(nil), identity.Roles...),
		TenantID:   identity.TenantID,
		TenantName: tenantName,
	}
}

func rolesFromRequestClaims(claims requestClaims) []string {
	seen := map[string]struct{}{}
	var roles []string
	add := func(role string) {
		if role == "" {
			return
		}
		if _, ok := seen[role]; ok {
			return
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}
	for _, role := range claims.RealmAccess.Roles {
		add(role)
	}
	for _, access := range claims.ResourceAccess {
		for _, role := range access.Roles {
			add(role)
		}
	}
	return roles
}

// repoHistoryLoader adapts chat.Repository to HistoryLoader.
type repoHistoryLoader struct {
	repo chat.Repository
}

func (l *repoHistoryLoader) FindAllMessages(ctx context.Context, sessionID uuid.UUID) ([]chat.ChatMessage, error) {
	return l.repo.FindAllMessages(ctx, sessionID)
}

// resolveSystemPrompt returns the effective system prompt for a run.
// P-C115-1: uses the session snapshot when available to preserve persona consistency
// even when the agent is updated between turns.
func resolveSystemPrompt(in chat.RunInput, agentCfg *chat.AgentRunConfig) string {
	if strings.TrimSpace(in.SystemPrompt) != "" {
		return strings.TrimSpace(in.SystemPrompt)
	}
	if in.SystemPromptSnapshot != nil && *in.SystemPromptSnapshot != "" {
		return *in.SystemPromptSnapshot
	}
	return agentCfg.SystemPrompt
}

// resolveModelConfig returns the effective model config for a run.
// P-C330-1: uses the session snapshot when available.
func resolveModelConfig(in chat.RunInput, agentCfg *chat.AgentRunConfig) json.RawMessage {
	if len(in.ModelConfigSnapshot) > 2 {
		return in.ModelConfigSnapshot
	}
	return agentCfg.ModelConfig
}

// hashConfig returns the SHA-256 hex digest of the given JSON payload.
// P-C173-1: used to detect modelConfig changes between turns.
// Normalises the input by sorting JSON keys via re-marshal so that semantically
// equivalent configs with different key ordering produce the same hash.
func hashConfig(config json.RawMessage) string {
	if len(config) == 0 {
		return ""
	}
	// Normalise: unmarshal into a generic map and re-marshal so key order is stable.
	var v interface{}
	if err := json.Unmarshal(config, &v); err != nil {
		// If the payload is not valid JSON, hash the raw bytes so we still track changes.
		sum := sha256.Sum256(config)
		return hex.EncodeToString(sum[:])
	}
	normalised, err := json.Marshal(v)
	if err != nil {
		sum := sha256.Sum256(config)
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(normalised)
	return hex.EncodeToString(sum[:])
}

func hashAgentSnapshot(agentCfg *chat.AgentRunConfig) string {
	if agentCfg == nil {
		return ""
	}
	snapshot := chat.AgentSnapshotData{
		SystemPrompt: agentCfg.SystemPrompt,
		ModelConfig:  cloneRawMessage(agent.SanitizeModelConfig(agentCfg.ModelConfig)),
		SkillIDs:     append([]uuid.UUID(nil), agentCfg.SkillIDs...),
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	return append(json.RawMessage(nil), raw...)
}

// detectConfigChange returns true when the agent's current modelConfig differs
// from the hash stored in the session (i.e. the config was updated since the
// previous run). Returns false when the session has no stored hash (first run).
func detectConfigChange(session chat.ChatSession, currentConfig json.RawMessage) bool {
	if session.ConfigHash == nil || *session.ConfigHash == "" {
		return false
	}
	return *session.ConfigHash != hashConfig(currentConfig)
}

func detectAgentConfigChange(session chat.ChatSession, agentCfg *chat.AgentRunConfig) bool {
	if agentCfg == nil {
		return false
	}
	if session.SystemPromptSnapshot != nil &&
		*session.SystemPromptSnapshot != "" &&
		*session.SystemPromptSnapshot != agentCfg.SystemPrompt {
		return true
	}
	if detectConfigChange(session, agentCfg.ModelConfig) {
		return true
	}
	return skillBindingsChanged(session.SkillBindingsSnapshot, agentCfg.SkillIDs)
}

func skillBindingsChanged(snapshot json.RawMessage, current []uuid.UUID) bool {
	if len(snapshot) <= 2 {
		return false
	}
	var data chat.SkillBindingsSnapshotData
	if err := json.Unmarshal(snapshot, &data); err != nil {
		return false
	}
	if len(data.SkillIDs) != len(current) {
		return true
	}
	remaining := make(map[uuid.UUID]int, len(current))
	for _, id := range current {
		remaining[id]++
	}
	for _, id := range data.SkillIDs {
		if remaining[id] == 0 {
			return true
		}
		remaining[id]--
	}
	return false
}
