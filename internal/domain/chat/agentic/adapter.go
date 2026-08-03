package agentic

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
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
	// from the "general.defaultProvider" setting. Returns "" if not configured.
	ResolveDefaultProvider(ctx context.Context) string
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
	skillRepo    *skill.Repository
	toolRepo     *tool.Repository
	integRepo    *integration.Service
	mcpRepo      mcp.Repository
	mcpClient    MCPClientService
	docSearch    knowledge.DocumentSearchClient // P-E1-2: wired when embedding service is available
	// skillResolver selects the tenant-scoped subset used by DYNAMIC_SKILL
	// sessions. It is nil only when no embedding service was configured.
	skillResolver *SkillSetResolver

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

	// agentDeleter routes agent deletions through the service layer (SEC-01).
	agentDeleter agent.Deleter
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

// WithAgentDeleter wires agent deletion through the service layer for agenthub_manage.
func (a *SessionRunnerAdapter) WithAgentDeleter(d agent.Deleter) *SessionRunnerAdapter {
	a.agentDeleter = d
	return a
}

func (a *SessionRunnerAdapter) newManagementExecutor() *ManagementExecutor {
	if a.agentRepo == nil {
		return nil
	}
	return NewManagementExecutor(
		a.agentRepo,
		a.agentDeleter,
		a.skillRepo,
		skill.NewService(a.skillRepo),
		a.toolRepo,
		a.mcpRepo,
	)
}

// WithDocumentSearchClient wires the document search client so the document_search
// builtin tool executes locally via pgvector instead of failing with "not available".
// P-E1-2: call this when the embedding service URL is configured.
func (a *SessionRunnerAdapter) WithDocumentSearchClient(client knowledge.DocumentSearchClient) *SessionRunnerAdapter {
	a.docSearch = client
	return a
}

// WithSkillSetResolver wires dynamic per-turn skill retrieval. The resolver is
// shared safely because tenant-specific thresholds are passed per Resolve call.
func (a *SessionRunnerAdapter) WithSkillSetResolver(resolver *SkillSetResolver) *SessionRunnerAdapter {
	a.skillResolver = resolver
	return a
}

// WithMemoryBridge wires the memory bridge so the memory_store builtin tool
// can persist and recall memories across sessions.
func (a *SessionRunnerAdapter) WithMemoryBridge(bridge *MemoryBridge) *SessionRunnerAdapter {
	a.memory = bridge
	return a
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
		// Pass skill.NewService as skillDeleter so delete operations enforce binding checks. P-C185-1.
		managementExec := f.adapter.newManagementExecutor()
		if managementExec != nil {
			runner.WithManagementExecutor(managementExec)
		}
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
	dynamic := in.Mode == chat.ModeDynamicSkill
	var (
		agentCfg     *chat.AgentRunConfig
		preRunEvents []chat.RunEvent
		err          error
	)
	if dynamic {
		agentCfg, preRunEvents, err = a.prepareDynamicRun(ctx, &in)
		if err != nil {
			return nil, err
		}
	} else {
		// P-C343-1: invalidate cached prompt sections for this agent at the start of
		// every run so that edits to skills, tools, or KBs are reflected without a
		// server restart. The cache is per-agent-ID, so other agents are unaffected.
		if a.prompt != nil {
			a.prompt.ClearCacheForAgent(in.AgentID)
		}
		agentCfg, err = a.agentLoader.GetAgentForRun(ctx, in.AgentID)
		if err != nil {
			return nil, fmt.Errorf("session runner: load agent: %w", err)
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
	}

	// P-C115-1: use session snapshot when available to preserve persona consistency.
	effectiveSystemPrompt := resolveSystemPrompt(in, agentCfg)
	effectiveModelConfig := resolveModelConfig(in, agentCfg)

	config := resolveRunConfig(ctx, a.modelFactory, effectiveModelConfig, a.llmCallTimeout)

	// Resolve the ChatModel for this agent's provider from the factory.
	// The model name is passed so the factory can select the correct API
	// variant (e.g. OpenAI Responses API for gpt-5+ models).
	chatModel, err := a.modelFactory.Build(ctx, config.Provider, config.Model)
	if err != nil {
		return nil, fmt.Errorf("session runner: build model for provider %q: %w", config.Provider, err)
	}
	// P-I1-1: log the effective model/provider at run start so operators can verify
	// which model is executing without querying the DB.
	slog.Info("agentic: run started", "model", config.Model, "provider", config.Provider, "agentID", in.AgentID, "mode", in.Mode)

	// Create a shared mailbox for inter-agent messaging within this run.
	agentMailbox := NewAgentMailbox()

	factory := &adapterRunnerFactory{adapter: a, chatModel: chatModel, agentMailbox: agentMailbox}
	runnerMemory := a.memory
	if dynamic {
		// agent_memory has a strict agent FK; dynamic personas are not agents.
		runnerMemory = nil
	}
	runner := NewRunner(
		chatModel,
		a.skillClient,
		a.prompt,
		a.tools,
		a.ctxManager,
		runnerMemory,
		a.repo,
		&repoHistoryLoader{repo: a.repo},
		a.hookExecutor,
		config,
	)
	// P-C325-2: wire metadata persister so run metrics are recorded on completion.
	runner.WithMetadataPersister(a.repo)
	if a.agentRepo != nil {
		// Pass skill.NewService as skillDeleter so delete operations enforce binding checks. P-C185-1.
		managementExec := a.newManagementExecutor()
		if managementExec != nil {
			runner.WithManagementExecutor(managementExec)
		}
	}
	if a.mcpClient != nil && !dynamic {
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
	runner.WithSubtaskExecutor(subtaskExec)
	if !dynamic {
		a.attachAuxiliaryComponents(runner, factory, chatModel, config)
	}

	// Register memory as turn-end handler (decoupled from runner loop).
	if runnerMemory != nil {
		runner.WithTurnEndHandlers(NewMemoryTurnEndHandler(runnerMemory))
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
		slog.Info("agentic: emitting input_request SSE event",
			"requestID", req.RequestID,
			"payloadSize", len(payload),
			"sessionID", in.SessionID,
		)
		elicEventCh <- NewRunEvent(EventInputRequest, json.RawMessage(data))
	})

	// P-C173-1: detect modelConfig changes between turns and persist a system
	// notification so the LLM is aware the configuration has changed.
	currentHash := hashConfig(agentCfg.ModelConfig)
	shouldPersistConfigHash := false
	configChangedEvent := chat.RunEvent{}
	if !dynamic {
		if session, err := a.repo.GetSessionByID(ctx, in.SessionID); err == nil {
			shouldPersistConfigHash = session.ConfigHash == nil || *session.ConfigHash == ""
			if detectConfigChange(session, agentCfg.ModelConfig) {
				notif := chat.ChatMessage{
					SessionID:   in.SessionID,
					Role:        "system",
					Content:     "[system] Agent configuration changed, but this session remains pinned to its original snapshot.",
					MessageType: chat.MessageTypeSystem,
				}
				if _, msgErr := a.repo.CreateMessage(ctx, notif); msgErr != nil {
					slog.Warn("agentic: failed to persist config-change notification", "error", msgErr)
				}
				configChangedEvent = newConfigChangedEvent(in.SessionID, in.AgentID)
			}
		}
	}

	agenticCh := runner.Run(ctx, RunInput{
		RunID:                  in.RunID,
		SessionID:              in.SessionID,
		AgentID:                in.AgentID,
		UserMessage:            in.UserMessage,
		Attachments:            in.Attachments,
		SystemPrompt:           effectiveSystemPrompt,
		TenantID:               in.TenantID,
		PermissionRules:        ParsePermissionRules(agentCfg.PermissionRules),
		Elicitation:            elicHandler,
		FrontendActions:        store,                     // CopilotKit Phase 1 (ClientStateStore satisfies FrontendActionsProvider)
		IsAdmin:                callerHasAdminRole(ctx),   // P-C298-1
		EnableManagement:       agentCfg.EnableManagement, // P-C184-2
		DisableAskUser:         agentCfg.DisableAskUser,
		DisableAgentDelegation: dynamic || agentCfg.DisableAgentDelegation,
		SkillIDsSnapshot:       in.SkillIDsSnapshot, // P-C115-1: use snapshot if available
		UseSkillIDsSnapshot:    dynamic || len(in.SkillIDsSnapshot) > 0,
		MCPServerNamesSnapshot: in.MCPServerNamesSnapshot, // P-C253-1: filter MCP tools by bound servers
		PermissionAudit:        permissionAuditForRun(dynamic, a.permAudit),
	})

	chatCh := make(chan chat.RunEvent, config.StreamBufferSize)
	go func() {
		defer func() {
			a.elicitation.unregister(runKey)
			store.DetachHandler(in.SessionID)
			// P-C173-1: persist a hash only for legacy sessions that do not yet
			// have one. Snapshotted sessions keep the original hash pinned.
			if shouldPersistConfigHash && currentHash != "" {
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

		for _, event := range preRunEvents {
			chatCh <- event
		}

		if configChangedEvent.Type != "" {
			chatCh <- configChangedEvent
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

func (a *SessionRunnerAdapter) prepareDynamicRun(ctx context.Context, in *chat.RunInput) (*chat.AgentRunConfig, []chat.RunEvent, error) {
	if in.DynamicPersona == nil {
		return nil, nil, errors.New("session runner: dynamic persona is required")
	}
	if in.AgentID != uuid.Nil {
		return nil, nil, errors.New("session runner: dynamic sessions cannot use an agent ID")
	}
	if a.skillResolver == nil {
		return nil, nil, errors.New("session runner: dynamic skill retrieval requires an embedding service")
	}
	store, ok := a.repo.(chat.DynamicSessionStore)
	if !ok {
		return nil, nil, errors.New("session runner: dynamic skill session store is not configured")
	}

	previous, err := chat.ParseDynamicSkillSetSnapshot(in.StickySkillSet)
	if err != nil {
		return nil, nil, err
	}
	state := SkillSetState{
		SkillIDs:       append([]uuid.UUID(nil), previous.SkillIDs...),
		SourceHashes:   dynamicSourceHashes(previous.SourceHashes),
		QueryEmbedding: append([]float32(nil), previous.QueryEmbedding...),
	}
	policy := in.DynamicPersona.RetrievalConfig.Normalize()
	resolved, err := a.skillResolver.ResolveWithConfig(ctx, SkillSetResolveInput{
		UserMessage: in.UserMessage,
		Previous:    state,
	}, SkillSetResolverConfig{
		TopK:           policy.TopK,
		DriftThreshold: policy.DriftThreshold,
		MaxStickySize:  policy.MaxStickySize,
		AllowDrift:     policy.AllowDrift,
		RefreshPolicy:  policy.RefreshPolicy,
		MinScore:       policy.MinScore,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("session runner: resolve dynamic skills: %w", err)
	}
	in.SkillIDsSnapshot = append([]uuid.UUID(nil), resolved.SkillIDs...)

	var events []chat.RunEvent
	if resolved.Refreshed {
		snapshot := chat.DynamicSkillSetSnapshot{
			SkillIDs:           append([]uuid.UUID(nil), resolved.State.SkillIDs...),
			SourceHashes:       dynamicSourceHashesForStorage(resolved.State.SourceHashes),
			QueryEmbedding:     append([]float32(nil), resolved.State.QueryEmbedding...),
			QueryEmbeddingHash: chat.HashDynamicQueryEmbedding(resolved.State.QueryEmbedding),
			RetrievedAt:        time.Now().UTC(),
			ScoreP50:           medianSearchScore(resolved.SearchScore),
			Source:             "embedding",
		}
		if err := store.UpdateSessionDynamicSkillSet(ctx, in.SessionID, snapshot); err != nil {
			return nil, nil, fmt.Errorf("session runner: persist dynamic skill set: %w", err)
		}
		if event := a.dynamicSkillSetEvent(ctx, resolved); event.Type != "" {
			events = append(events, event)
		}
	}

	if a.prompt != nil {
		a.prompt.ClearCacheForDynamicSession(in.SessionID)
	}
	return &chat.AgentRunConfig{
		ID:                     uuid.Nil,
		SystemPrompt:           in.DynamicPersona.SystemPrompt,
		ModelConfig:            append(json.RawMessage(nil), in.DynamicPersona.ModelConfig...),
		EnableManagement:       in.DynamicPersona.EnableManagement,
		DisableAgentDelegation: true,
		Status:                 string(agent.StatusPublished),
	}, events, nil
}

func dynamicSourceHashes(source map[string]string) map[uuid.UUID]string {
	if len(source) == 0 {
		return map[uuid.UUID]string{}
	}
	result := make(map[uuid.UUID]string, len(source))
	for id, hash := range source {
		parsed, err := uuid.Parse(id)
		if err == nil {
			result[parsed] = hash
		}
	}
	return result
}

func dynamicSourceHashesForStorage(source map[uuid.UUID]string) map[string]string {
	result := make(map[string]string, len(source))
	for id, hash := range source {
		result[id.String()] = hash
	}
	return result
}

func medianSearchScore(scores map[uuid.UUID]float64) float64 {
	if len(scores) == 0 {
		return 0
	}
	values := make([]float64, 0, len(scores))
	for _, score := range scores {
		values = append(values, score)
	}
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}

func (a *SessionRunnerAdapter) dynamicSkillSetEvent(ctx context.Context, resolved SkillSetResolveResult) chat.RunEvent {
	if !resolved.Refreshed {
		return chat.RunEvent{}
	}
	entries := make([]SkillSetEntry, 0, len(resolved.SkillIDs))
	byID := map[uuid.UUID]string{}
	if a.skillRepo != nil && len(resolved.SkillIDs) > 0 {
		if skills, err := a.skillRepo.ListByIDs(ctx, resolved.SkillIDs); err == nil {
			for _, item := range skills {
				byID[item.ID] = item.Slug
			}
		}
	}
	for _, id := range resolved.SkillIDs {
		entries = append(entries, SkillSetEntry{
			ID: id.String(), Slug: byID[id], Score: resolved.SearchScore[id],
		})
	}
	payload := SkillSetData{Skills: entries, Reason: resolved.Reason, DriftScore: resolved.DriftScore}
	if resolved.Reason == "initial" {
		return chat.RunEvent{Type: string(EventSkillSetInitial), Data: mustMarshal(payload)}
	}
	return chat.RunEvent{Type: string(EventSkillSetChanged), Data: mustMarshal(payload)}
}

func permissionAuditForRun(dynamic bool, audit PermissionAuditLogger) PermissionAuditLogger {
	if dynamic {
		return nil
	}
	return audit
}

func mustMarshal(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
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
	raw := tenant.TokenFromContext(ctx)
	if raw == "" {
		return false
	}
	// JWT = header.payload.signature — parse only the payload segment.
	parts := strings.SplitN(raw, ".", 3)
	if len(parts) != 3 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var claims struct {
		RealmAccess struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
		// Keycloak may place roles in resource_access.<clientId>.roles
		ResourceAccess map[string]struct {
			Roles []string `json:"roles"`
		} `json:"resource_access"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
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
	if in.SystemPromptSnapshot != nil {
		return *in.SystemPromptSnapshot
	}
	return agentCfg.SystemPrompt
}

// resolveModelConfig returns the effective model config for a run.
// P-C330-1: uses the session snapshot when available.
func resolveModelConfig(in chat.RunInput, agentCfg *chat.AgentRunConfig) json.RawMessage {
	if len(in.ModelConfigSnapshot) > 0 {
		return in.ModelConfigSnapshot
	}
	return agentCfg.ModelConfig
}

// hashConfig returns the SHA-256 hex digest of the given JSON payload.
// P-C173-1: used to detect modelConfig changes between turns.
// Normalises the input by sorting JSON keys via re-marshal so that semantically
// equivalent configs with different key ordering produce the same hash.
func hashConfig(config json.RawMessage) string {
	return chat.HashModelConfig(config)
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

func newConfigChangedEvent(sessionID, agentID uuid.UUID) chat.RunEvent {
	return chat.RunEvent{
		Type: string(EventConfigChanged),
		Data: NewRunEvent(EventConfigChanged, ConfigChangedData{
			SessionID: sessionID.String(),
			AgentID:   agentID.String(),
			Message:   "Agent configuration changed; this session is still using its pinned snapshot.",
		}).Data,
	}
}
