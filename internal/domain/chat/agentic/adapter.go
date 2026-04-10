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
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
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

	// llmCallTimeout overrides the default per-LLM-call timeout set by DefaultRunConfig.
	// P-C102-1: sourced from LLM_CALL_TIMEOUT_SECS env var at server startup.
	// Zero means "use the default from DefaultRunConfig".
	llmCallTimeout time.Duration

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

func (f staticModelFactory) ResolveModel(_ context.Context, _ string) string          { return "" }
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
		managementExec := NewManagementExecutor(f.adapter.agentRepo, f.adapter.skillRepo, skill.NewService(f.adapter.skillRepo), f.adapter.toolRepo, f.adapter.mcpRepo)
		runner.WithManagementExecutor(managementExec)
	}
	if f.adapter.mcpClient != nil {
		runner.WithMCPClient(f.adapter.mcpClient)
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

	// P-C178-1: reject runs for agents that are not PUBLISHED.
	switch agentCfg.Status {
	case string(agent.StatusDraft):
		return nil, fmt.Errorf("%w: agent %s is in DRAFT status", chat.ErrAgentNotPublished, in.AgentID)
	case string(agent.StatusArchived):
		return nil, fmt.Errorf("%w: agent %s", chat.ErrAgentArchived, in.AgentID)
	case string(agent.StatusPublished), "": // empty = legacy records without status
		// OK — proceed
	}

	config := RunConfigFromModelConfig(agentCfg.ModelConfig)
	defaultCfg := DefaultRunConfig()

	// P-C102-1: apply server-level LLM call timeout when the agent's model_config
	// did not explicitly override it (i.e. still equals the compiled default).
	if a.llmCallTimeout > 0 && config.LLMCallTimeout == defaultCfg.LLMCallTimeout {
		config.LLMCallTimeout = a.llmCallTimeout
	}

	// When the agent has no explicit provider, use the tenant's configured default.
	// This avoids hard-coding the fallback to "anthropic" when the tenant is on
	// a different provider (P-C75-1).
	if config.Provider == "" || config.Provider == defaultCfg.Provider {
		if defaultProvider := a.modelFactory.ResolveDefaultProvider(ctx); defaultProvider != "" {
			config.Provider = defaultProvider
		}
	}

	// If the agent doesn't specify a model, resolve it from the tenant's settings.
	if config.Model == "" || config.Model == defaultCfg.Model {
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
	if a.agentRepo != nil {
		// Pass skill.NewService as skillDeleter so delete operations enforce binding checks. P-C185-1.
		managementExec := NewManagementExecutor(a.agentRepo, a.skillRepo, skill.NewService(a.skillRepo), a.toolRepo, a.mcpRepo)
		runner.WithManagementExecutor(managementExec)
	}
	if a.mcpClient != nil {
		runner.WithMCPClient(a.mcpClient)
	}
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

	// P-C115-1: use session snapshot when available to preserve persona consistency.
	effectiveSystemPrompt := resolveSystemPrompt(in, agentCfg)
	effectiveModelConfig := resolveModelConfig(in, agentCfg)
	_ = effectiveModelConfig // model config snapshot used for future provider resolution

	// P-C173-1: detect modelConfig changes between turns and persist a system
	// notification so the LLM is aware the configuration has changed.
	currentHash := hashConfig(agentCfg.ModelConfig)
	if session, err := a.repo.GetSessionByID(ctx, in.SessionID); err == nil {
		if detectConfigChange(session, agentCfg.ModelConfig) {
			notif := chat.ChatMessage{
				SessionID:   in.SessionID,
				Role:        "system",
				Content:     "[system] Agent configuration was updated since the last turn. The new settings are now in effect.",
				MessageType: chat.MessageTypeSystem,
			}
			if _, msgErr := a.repo.CreateMessage(ctx, notif); msgErr != nil {
				slog.Warn("agentic: failed to persist config-change notification", "error", msgErr)
			}
		}
	}

	agenticCh := runner.Run(ctx, RunInput{
		RunID:            in.RunID,
		SessionID:        in.SessionID,
		AgentID:          in.AgentID,
		UserMessage:      in.UserMessage,
		SystemPrompt:     effectiveSystemPrompt,
		TenantID:         in.TenantID,
		PermissionRules:  ParsePermissionRules(agentCfg.PermissionRules),
		Elicitation:      elicHandler,
		IsAdmin:          callerHasAdminRole(ctx),    // P-C298-1
		EnableManagement: agentCfg.EnableManagement, // P-C184-2
		SkillIDsSnapshot: in.SkillIDsSnapshot,       // P-C115-1: use snapshot if available
	})

	chatCh := make(chan chat.RunEvent, config.StreamBufferSize)
	go func() {
		defer func() {
			a.elicitation.unregister(runKey)
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

// detectConfigChange returns true when the agent's current modelConfig differs
// from the hash stored in the session (i.e. the config was updated since the
// previous run). Returns false when the session has no stored hash (first run).
func detectConfigChange(session chat.ChatSession, currentConfig json.RawMessage) bool {
	if session.ConfigHash == nil || *session.ConfigHash == "" {
		return false
	}
	return *session.ConfigHash != hashConfig(currentConfig)
}
