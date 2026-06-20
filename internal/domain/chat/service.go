package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	agentdomain "github.com/AgentHub-Studio/agenthub-api/internal/domain/agent"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
)

// AgentLoader returns agent configuration needed by the Runner.
type AgentLoader interface {
	GetAgentForRun(ctx context.Context, id uuid.UUID) (*AgentRunConfig, error)
}

// AgentRunConfig carries agent fields consumed by the agentic Runner.
type AgentRunConfig struct {
	ID              uuid.UUID
	SystemPrompt    string
	ModelConfig     json.RawMessage // raw JSON — passed to RunConfigFromModelConfig
	PermissionRules json.RawMessage // raw JSON — {"allow":[],"deny":[],"confirm":[]}
	// EnableManagement controls whether the agenthub_manage builtin tool is included.
	// P-C184-2: both this flag AND the caller's admin role must be true.
	EnableManagement bool
	// DisableAskUser removes the ask_user builtin from the agent's tool set.
	// Stored in agent.Config["disableAskUser"]. Default false.
	DisableAskUser bool
	// DisableAgentDelegation removes the agent (sub-agent spawner) builtin.
	// Stored in agent.Config["disableAgentDelegation"]. Default false.
	DisableAgentDelegation bool
	// Status is the agent's lifecycle status. Used to reject runs for DRAFT/ARCHIVED agents.
	// P-C178-1.
	Status string
	// SkillIDs are the IDs of skills bound to the agent at the time of loading.
	// Captured at session creation for snapshotting (P-C115-1).
	SkillIDs []uuid.UUID
	// MCPServerNames are the names of MCP servers bound to the agent.
	// P-C253-1: used to filter MCP tools in the agentic loop.
	MCPServerNames []string
}

// ErrAgentNotPublished is returned when an agent is not in PUBLISHED status.
var ErrAgentNotPublished = errors.New("agent is not published")

// ErrAgentArchived is returned when an agent has been archived.
var ErrAgentArchived = errors.New("agent is archived and no longer accepts new sessions")

// ErrNoAgentAvailable is returned when a session has no agent bound and the
// tenant has no published agent to route the message to. The handler maps this
// to a friendly, actionable error (create and publish an agent).
var ErrNoAgentAvailable = errors.New("no published agent available for routing")

// RunEvent is the envelope emitted by the agentic loop.
// Defined here (in the chat package) to avoid an import cycle:
// chat → chat/agentic → chat.
type RunEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// RunInput carries everything needed to start an agentic run.
type RunInput struct {
	RunID        uuid.UUID // ID of the persisted run
	SessionID    uuid.UUID
	AgentID      uuid.UUID
	UserMessage  string
	SystemPrompt string
	TenantID     string
	// UserMessageID is non-nil when the user message was already persisted by the
	// caller (typically chat.Service.RunSession). When set, the runner skips
	// its own user message persistence to avoid duplicates. P-C178-2.
	UserMessageID *uuid.UUID
	// SystemPromptSnapshot and ModelConfigSnapshot carry the values captured at
	// session creation time (P-C115-1 / P-C330-1). When set, the adapter uses
	// them instead of the agent's current values to ensure consistency throughout
	// the conversation even if the agent is updated between runs.
	SystemPromptSnapshot *string
	ModelConfigSnapshot  json.RawMessage
	// SkillIDsSnapshot, when non-empty, contains the skill IDs captured at session
	// creation time. The runner uses these IDs instead of the agent's current bindings
	// so the tool set stays consistent throughout the conversation. P-C115-1.
	SkillIDsSnapshot []uuid.UUID
	// MCPServerNamesSnapshot, when non-empty, contains the MCP server names bound to
	// the agent at session creation time. Passed to the runner to filter MCP tools.
	// P-C253-1: agent-level MCP binding enforcement.
	MCPServerNamesSnapshot []string
}

// RunOverrides contains transient Agent Studio values for a single run.
// They override the session snapshot only in memory and are never persisted
// back to chat_session.
type RunOverrides struct {
	SystemPrompt *string         `json:"systemPrompt,omitempty"`
	ModelConfig  json.RawMessage `json:"modelConfig,omitempty"`
}

// Empty reports whether the override payload has any effective values.
func (o RunOverrides) Empty() bool {
	n := o.normalized()
	return n.SystemPrompt == nil && len(n.ModelConfig) == 0
}

// Validate rejects malformed override payloads before a run is started.
func (o RunOverrides) Validate() error {
	n := o.normalized()
	if len(n.ModelConfig) == 0 {
		return nil
	}
	if !json.Valid(n.ModelConfig) {
		return fmt.Errorf("modelConfig override must be valid JSON")
	}
	trimmed := strings.TrimSpace(string(n.ModelConfig))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	if trimmed[0] != '{' {
		return fmt.Errorf("modelConfig override must be a JSON object")
	}
	return nil
}

func (o RunOverrides) normalized() RunOverrides {
	var out RunOverrides
	if o.SystemPrompt != nil {
		if prompt := strings.TrimSpace(*o.SystemPrompt); prompt != "" {
			out.SystemPrompt = &prompt
		}
	}
	if raw := strings.TrimSpace(string(o.ModelConfig)); raw != "" && raw != "null" {
		out.ModelConfig = json.RawMessage(raw)
	}
	return out
}

func applyRunOverrides(systemPrompt *string, modelConfig json.RawMessage, overrides RunOverrides) (*string, json.RawMessage) {
	n := overrides.normalized()
	effectiveSystemPrompt := systemPrompt
	effectiveModelConfig := modelConfig
	if n.SystemPrompt != nil {
		prompt := *n.SystemPrompt
		effectiveSystemPrompt = &prompt
	}
	if len(n.ModelConfig) > 0 {
		effectiveModelConfig = append(json.RawMessage(nil), n.ModelConfig...)
	}
	return effectiveSystemPrompt, effectiveModelConfig
}

// ElicitationResponder routes a user's elicitation response to the active run.
// Implemented by agentic.SessionRunnerAdapter; no-op on other implementations.
type ElicitationResponder interface {
	RespondElicitation(sessionID, requestID string, result ElicitationResult) bool
}

// ElicitationResult mirrors agentic.ElicitationResult to avoid circular imports.
type ElicitationResult struct {
	Action  string                 `json:"action"`
	Content map[string]interface{} `json:"content,omitempty"`
}

// ClientStateApplier merges a CopilotKit-style patch into the per-session store
// owned by the runner. Mirrors agentic.ClientStatePatch to avoid circular
// imports. Implemented by agentic.SessionRunnerAdapter; no-op on others.
type ClientStateApplier interface {
	ApplyClientState(sessionID uuid.UUID, patch ClientStatePatch)
}

// ClientStateFrontendAction mirrors agentic.FrontendAction. JSON-encoded
// parameters follow the JSON Schema shape (`type=object`, `properties`, ...).
type ClientStateFrontendAction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

// ClientStateReadable mirrors agentic.Readable.
type ClientStateReadable struct {
	ID          string          `json:"id"`
	Description string          `json:"description"`
	Value       json.RawMessage `json:"value"`
	ParentID    string          `json:"parentId,omitempty"`
}

// ClientStateActionResult mirrors agentic.FrontendActionResult.
type ClientStateActionResult struct {
	ID     string          `json:"id"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// ClientStatePatch is the body of `POST /api/chat/sessions/{id}/client-state`.
// Each top-level field is optional; nil slices mean "no change for this kind".
type ClientStatePatch struct {
	FrontendActions []ClientStateFrontendAction `json:"frontendActions,omitempty"`
	Readables       []ClientStateReadable       `json:"readables,omitempty"`
	ActionResults   []ClientStateActionResult   `json:"actionResults,omitempty"`
}

// SessionRunner starts an agentic loop and returns a channel of RunEvents.
// The chat.Service calls this; the concrete implementation lives in
// chat/agentic and is injected via the server wiring.
type SessionRunner interface {
	RunSession(ctx context.Context, in RunInput) (<-chan RunEvent, error)
}

// Service provides business logic for chat operations.
type Service struct {
	repo        Repository
	runner      SessionRunner
	agentLoader AgentLoader // optional — used to capture agent snapshot at session creation
}

// NewService creates a new Service backed by the given Repository.
// runner may be nil (disables agentic features).
func NewService(repo Repository, runner SessionRunner) *Service {
	return &Service{repo: repo, runner: runner}
}

// WithAgentLoader wires an agent loader for capturing snapshots at session creation.
func (s *Service) WithAgentLoader(loader AgentLoader) *Service {
	s.agentLoader = loader
	return s
}

// GetActiveRun returns the active run for a session if any.
func (s *Service) GetActiveRun(ctx context.Context, sessionID uuid.UUID) (ChatRunResponse, bool, error) {
	run, found, err := s.repo.GetActiveRunBySession(ctx, sessionID)
	if err != nil {
		return ChatRunResponse{}, false, fmt.Errorf("chat service: get active run: %w", err)
	}
	if !found {
		return ChatRunResponse{}, false, nil
	}
	return RunResponseFrom(run), true, nil
}

// ListSessions returns a paginated list of chat sessions.
func (s *Service) ListSessions(ctx context.Context, req pagination.PageRequest) (pagination.Page[ChatSessionResponse], error) {
	items, total, err := s.repo.FindSessions(ctx, req)
	if err != nil {
		return pagination.Page[ChatSessionResponse]{}, fmt.Errorf("chat service: list sessions: %w", err)
	}

	responses := make([]ChatSessionResponse, len(items))
	for i, item := range items {
		responses[i] = SessionResponseFrom(item)
	}

	return pagination.NewPage(responses, total, req), nil
}

// GetSession returns a single chat session by ID.
func (s *Service) GetSession(ctx context.Context, id uuid.UUID) (ChatSessionResponse, error) {
	session, err := s.repo.GetSessionByID(ctx, id)
	if err != nil {
		return ChatSessionResponse{}, err
	}
	return SessionResponseFrom(session), nil
}

// CreateSession creates a new chat session.
func (s *Service) CreateSession(ctx context.Context, req CreateSessionRequest) (ChatSessionResponse, error) {
	// Bug 183: strip HTML do title (XSS prevention).
	req.Title = sanitize.StripHTML(req.Title)
	if req.Title == "" {
		return ChatSessionResponse{}, fmt.Errorf("chat service: title is required")
	}
	// Bug 135: title varchar(500) — gate length antes do INSERT
	// (sem isso 422 vazava SQL 22001 para o cliente).
	if len(req.Title) > 500 {
		return ChatSessionResponse{}, fmt.Errorf("chat service: title exceeds maximum length of 500 chars (got %d)", len(req.Title))
	}

	session := ChatSession{
		AgentID: req.AgentID,
		Title:   req.Title,
		Status:  StatusActive,
	}

	// P-C115-1 / P-C330-1: capture agent snapshot at session creation so that
	// the persona, model config and skill bindings remain consistent throughout
	// the conversation even if the agent is updated between turns.
	if req.AgentID != nil && s.agentLoader != nil {
		if agentCfg, err := s.agentLoader.GetAgentForRun(ctx, *req.AgentID); err == nil {
			// Bug 245: rejeitar criação de session em agent que não está PUBLISHED.
			// ErrAgentNotPublished e ErrAgentArchived já estavam definidos no
			// package mas nunca eram disparados em CreateSession. Resultado:
			// usuários criavam sessions em agent DRAFT/ARCHIVED e descobriam só
			// no primeiro run que algo não funcionava.
			switch strings.ToUpper(agentCfg.Status) {
			case "ARCHIVED":
				return ChatSessionResponse{}, ErrAgentArchived
			case "DRAFT":
				return ChatSessionResponse{}, ErrAgentNotPublished
			}
			snapshotAgent(agentCfg, &session)
		}
		// Snapshot failure is non-fatal: session creation proceeds without snapshot.
	}

	created, err := s.repo.CreateSession(ctx, session)
	if err != nil {
		return ChatSessionResponse{}, fmt.Errorf("chat service: create session: %w", err)
	}

	return SessionResponseFrom(created), nil
}

// snapshotAgent captures the agent's persona, model config and skill bindings
// onto the session so they stay consistent across every run of the session even
// if the agent is edited later (P-C115-1 / P-C330-1). It mutates sess in place
// and is a no-op when agentCfg is nil. Shared by CreateSession (agent picked up
// front) and RunSession (agent picked by the router for an agentless session).
func snapshotAgent(agentCfg *AgentRunConfig, sess *ChatSession) {
	if agentCfg == nil {
		return
	}
	snapshot := agentCfg.SystemPrompt
	sess.SystemPromptSnapshot = &snapshot

	modelConfig := agentCfg.ModelConfig
	if len(modelConfig) == 0 {
		modelConfig = json.RawMessage(`{}`)
	}
	sess.ModelConfigSnapshot = agentdomain.SanitizeModelConfig(modelConfig)
	if hash := HashModelConfig(modelConfig); hash != "" {
		sess.ConfigHash = &hash
	}

	skillIDs := agentCfg.SkillIDs
	if skillIDs == nil {
		skillIDs = []uuid.UUID{}
	}
	snapshotData := SkillBindingsSnapshotData{SkillIDs: skillIDs}
	if snapshotJSON, err := json.Marshal(snapshotData); err == nil {
		sess.SkillBindingsSnapshot = snapshotJSON
	}
}

// ArchiveSession sets a session's status to ARCHIVED.
func (s *Service) ArchiveSession(ctx context.Context, id uuid.UUID) (ChatSessionResponse, error) {
	session, err := s.repo.UpdateSessionStatus(ctx, id, StatusArchived)
	if err != nil {
		return ChatSessionResponse{}, err
	}
	return SessionResponseFrom(session), nil
}

// RenameSession updates a session's title.
func (s *Service) RenameSession(ctx context.Context, id uuid.UUID, title string) (ChatSessionResponse, error) {
	// Bug 183: strip HTML do title (XSS prevention).
	title = sanitize.StripHTML(title)
	session, err := s.repo.UpdateSessionTitle(ctx, id, title)
	if err != nil {
		return ChatSessionResponse{}, err
	}
	return SessionResponseFrom(session), nil
}

// DeleteSession removes a chat session by ID.
func (s *Service) DeleteSession(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.DeleteSession(ctx, id); err != nil {
		return fmt.Errorf("chat service: delete session: %w", err)
	}
	return nil
}

// ListMessages returns a paginated list of messages for a session.
func (s *Service) ListMessages(ctx context.Context, sessionID uuid.UUID, req pagination.PageRequest) (pagination.Page[ChatMessageResponse], error) {
	items, total, err := s.repo.FindMessages(ctx, sessionID, req)
	if err != nil {
		return pagination.Page[ChatMessageResponse]{}, fmt.Errorf("chat service: list messages: %w", err)
	}

	responses := make([]ChatMessageResponse, len(items))
	for i, item := range items {
		responses[i] = MessageResponseFrom(item)
	}

	return pagination.NewPage(responses, total, req), nil
}

// GetLatestAssistantMessage delegates to the repository to find the newest assistant
// message created after the given time, returning it as a DTO.
func (s *Service) GetLatestAssistantMessage(ctx context.Context, sessionID uuid.UUID, after time.Time) (ChatMessageResponse, bool, error) {
	msg, found, err := s.repo.GetLatestAssistantMessage(ctx, sessionID, after)
	if err != nil {
		return ChatMessageResponse{}, false, fmt.Errorf("chat service: get latest assistant message: %w", err)
	}
	if !found {
		return ChatMessageResponse{}, false, nil
	}
	return MessageResponseFrom(msg), true, nil
}

// AddMessage adds a message to a chat session.
// Returns the persisted user message DTO. For agent-bound sessions the agentic
// loop is NOT started here — use RunSession instead.
func (s *Service) AddMessage(ctx context.Context, sessionID uuid.UUID, req CreateMessageRequest) (ChatMessageResponse, error) {
	// Bug 247: rejeitar addMessage em session ARCHIVED. Sem isso, mensagens
	// novas eram silenciosamente persistidas e poluíam o histórico de uma
	// session que o usuário marcou como concluída.
	if session, err := s.repo.GetSessionByID(ctx, sessionID); err == nil {
		if session.Status == StatusArchived {
			return ChatMessageResponse{}, ErrSessionArchived
		}
	}
	// Sem else: se a session não existe, deixa o INSERT abaixo falhar com FK
	// (bug 230 já mapeia para ErrNotFound).

	if req.Role == "" {
		return ChatMessageResponse{}, fmt.Errorf("chat service: role is required")
	}
	// Bug 185: clientes só podem postar role=user. Mensagens do tipo
	// assistant/system/tool são responsabilidade do runtime agêntico —
	// permitir client-side seria vetor de prompt injection (override
	// do system_prompt do agent) e poluição de histórico.
	switch req.Role {
	case "user":
	case "assistant", "system", "tool":
		return ChatMessageResponse{}, fmt.Errorf("chat service: role %q is reserved for the agentic runtime; only \"user\" is allowed from clients", req.Role)
	default:
		return ChatMessageResponse{}, fmt.Errorf("chat service: role must be \"user\" (got %q)", req.Role)
	}
	if req.Content == "" {
		return ChatMessageResponse{}, fmt.Errorf("chat service: content is required")
	}
	// Bug 163: cap content em 64KB. Chat messages podem ser longas (code
	// blocks, JSON inline) mas 1MB+ é DoS — cada mensagem vai pra LLM
	// e custa tokens absurdos, além de inflar storage por session.
	if len(req.Content) > 64000 {
		return ChatMessageResponse{}, fmt.Errorf("chat service: content exceeds maximum length of 64000 chars (got %d)", len(req.Content))
	}
	// Bug 151: messageType era silenciosamente coerced para "text" quando
	// o cliente passava qualquer string. Frontend que enviasse messageType
	// errado nunca via o erro — comportamento ficava confuso e bugs ficavam
	// invisíveis.
	//
	// Bug 186: cliente só pode postar messageType="text". Os demais tipos
	// (tool_use, tool_result, system, compact_summary) são produzidos pelo
	// runtime agêntico (Runner persiste via repo.CreateMessage diretamente,
	// bypassa este gate). Aceitar tool_use/tool_result de cliente permitia
	// spoofar fake tool history — vetor de prompt injection.
	if req.MessageType != "" && req.MessageType != MessageTypeText {
		return ChatMessageResponse{}, fmt.Errorf("chat service: messageType %q is reserved for the agentic runtime; only \"text\" is allowed from clients", req.MessageType)
	}

	m := ChatMessage{
		SessionID:   sessionID,
		Role:        req.Role,
		Content:     req.Content,
		MessageType: req.MessageType,
	}

	created, err := s.repo.CreateMessage(ctx, m)
	if err != nil {
		return ChatMessageResponse{}, fmt.Errorf("chat service: add message: %w", err)
	}

	return MessageResponseFrom(created), nil
}

// RespondElicitation routes a user response to an active elicitation request.
// Returns false when the session has no active run or the requestID is not found.
func (s *Service) RespondElicitation(sessionID, requestID string, result ElicitationResult) bool {
	if r, ok := s.runner.(ElicitationResponder); ok {
		return r.RespondElicitation(sessionID, requestID, result)
	}
	return false
}

// ApplyClientState forwards a CopilotKit client-state patch (frontend actions,
// readables, action results) to the runner's store. Silently ignored when the
// runner does not implement [ClientStateApplier] — keeps the chat service
// usable in tests without a full agentic adapter wired in.
func (s *Service) ApplyClientState(sessionID uuid.UUID, patch ClientStatePatch) {
	if r, ok := s.runner.(ClientStateApplier); ok {
		r.ApplyClientState(sessionID, patch)
	}
}

// RunSession starts an agentic run for the given session.
// It loads the session, validates it has an agent, then delegates to the SessionRunner.
// The caller (SSE handler) consumes the returned channel for streaming.
func (s *Service) RunSession(ctx context.Context, sessionID uuid.UUID, userMessage, tenantID string, overrides RunOverrides) (<-chan RunEvent, error) {
	if s.runner == nil {
		return nil, fmt.Errorf("chat service: agentic features not configured")
	}
	if err := overrides.Validate(); err != nil {
		return nil, fmt.Errorf("chat service: invalid run overrides: %w", err)
	}

	session, err := s.repo.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("chat service: get session: %w", err)
	}
	if session.AgentID == nil {
		routed, err := routeAgentID(ctx, s.repo, userMessage)
		if err != nil {
			return nil, fmt.Errorf("chat service: route agent: %w", err)
		}
		if routed == nil {
			return nil, fmt.Errorf("chat service: %w", ErrNoAgentAvailable)
		}
		if err := s.repo.UpdateSessionAgent(ctx, sessionID, *routed); err != nil {
			return nil, fmt.Errorf("chat service: bind routed agent: %w", err)
		}
		session.AgentID = routed
		// P-C115-1: snapshot the just-routed agent onto the session so the
		// persona/model/skills stay consistent across the rest of the
		// conversation. Non-fatal on failure — the run still proceeds using the
		// agent's current config via the adapter's snapshot fallback.
		if s.agentLoader != nil {
			if agentCfg, err := s.agentLoader.GetAgentForRun(ctx, *routed); err == nil {
				snapshotAgent(agentCfg, &session)
				if err := s.repo.UpdateSessionSnapshots(ctx, sessionID,
					session.SystemPromptSnapshot, session.ModelConfigSnapshot, session.SkillBindingsSnapshot, configHashValue(session.ConfigHash)); err != nil {
					slog.Warn("chat service: failed to persist routed-agent snapshot",
						"sessionID", sessionID, "agentID", *routed, "error", err)
				}
			}
		}
	}
	if session.AgentID != nil && s.agentLoader != nil && sessionNeedsSnapshot(session) {
		if agentCfg, err := s.agentLoader.GetAgentForRun(ctx, *session.AgentID); err == nil {
			snapshotAgent(agentCfg, &session)
			if err := s.repo.UpdateSessionSnapshots(ctx, sessionID,
				session.SystemPromptSnapshot, session.ModelConfigSnapshot, session.SkillBindingsSnapshot, configHashValue(session.ConfigHash)); err != nil {
				slog.Warn("chat service: failed to persist first-run snapshot",
					"sessionID", sessionID, "agentID", *session.AgentID, "error", err)
			}
		}
	}

	// P-C178-2: persist user message BEFORE starting the run so it is never lost
	// even if the run fails to initialise (e.g. agent not published, model error).
	var userMsgID *uuid.UUID
	if userMessage != "" {
		msg := ChatMessage{
			SessionID: sessionID,
			Role:      "user",
			Content:   userMessage,
		}
		persisted, err := s.repo.CreateMessage(ctx, msg)
		if err != nil {
			return nil, fmt.Errorf("chat service: persist user message: %w", err)
		}
		userMsgID = &persisted.ID
	}

	// P-C115-1: extract skill IDs from the session snapshot so the runner uses
	// the same tool set that was active when the session was created.
	var skillIDsSnapshot []uuid.UUID
	if len(session.SkillBindingsSnapshot) > 2 {
		var snap SkillBindingsSnapshotData
		if err := json.Unmarshal(session.SkillBindingsSnapshot, &snap); err == nil {
			skillIDsSnapshot = snap.SkillIDs
		}
	}

	// P-C253-1: load MCP server names bound to the agent so the runner can filter
	// MCP tools to only those from servers explicitly bound to this agent.
	var mcpServerNames []string
	if s.agentLoader != nil {
		if agentCfg, err := s.agentLoader.GetAgentForRun(ctx, *session.AgentID); err == nil {
			mcpServerNames = agentCfg.MCPServerNames
		}
	}
	systemPromptSnapshot, modelConfigSnapshot := applyRunOverrides(
		session.SystemPromptSnapshot,
		session.ModelConfigSnapshot,
		overrides,
	)

	return s.runner.RunSession(ctx, RunInput{
		SessionID:              sessionID,
		AgentID:                *session.AgentID,
		UserMessage:            userMessage,
		TenantID:               tenantID,
		UserMessageID:          userMsgID,
		SystemPromptSnapshot:   systemPromptSnapshot,
		ModelConfigSnapshot:    modelConfigSnapshot,
		SkillIDsSnapshot:       skillIDsSnapshot,
		MCPServerNamesSnapshot: mcpServerNames,
	})
}

func sessionNeedsSnapshot(session ChatSession) bool {
	return session.SystemPromptSnapshot == nil ||
		len(session.ModelConfigSnapshot) == 0 ||
		len(session.SkillBindingsSnapshot) == 0 ||
		session.ConfigHash == nil ||
		*session.ConfigHash == ""
}

func configHashValue(hash *string) string {
	if hash == nil {
		return ""
	}
	return *hash
}

// agentRouter is the subset of Repository needed to route a user message to a
// published agent. Both chat.Service (SSE path) and the async executor depend
// on routeAgentID, so it takes this narrow interface rather than *Service.
type agentRouter interface {
	FindAgentsForRouting(ctx context.Context) ([]AgentRoutingInfo, error)
}

// routeAgentID selects the best published agent for the given user message.
// When only one agent exists it is returned immediately. When multiple agents
// are available, each is scored by keyword overlap between the user message
// and the agent's name + description. The highest-scoring agent wins; ties
// are broken by the natural ordering returned by FindAgentsForRouting
// (agenthub-assistant slug first, then oldest created_at). Returns nil when the
// tenant has no published agent — callers map this to ErrNoAgentAvailable.
func routeAgentID(ctx context.Context, repo agentRouter, userMessage string) (*uuid.UUID, error) {
	agents, err := repo.FindAgentsForRouting(ctx)
	if err != nil {
		return nil, err
	}
	if len(agents) == 0 {
		return nil, nil
	}
	if len(agents) == 1 {
		id := agents[0].ID
		return &id, nil
	}

	best := selectBestAgent(agents, userMessage)
	return &best, nil
}

// selectBestAgent scores each agent by the number of tokens from the user
// message that appear in the agent's name or description (case-insensitive).
// Returns the ID of the highest-scoring agent; falls back to the first entry
// (which FindAgentsForRouting orders as agenthub-assistant first).
func selectBestAgent(agents []AgentRoutingInfo, userMessage string) uuid.UUID {
	tokens := tokenize(userMessage)
	if len(tokens) == 0 {
		return agents[0].ID
	}

	bestIdx := 0
	bestScore := -1

	for i, a := range agents {
		corpus := strings.ToLower(a.Name + " " + a.Description)
		score := 0
		for _, tok := range tokens {
			if strings.Contains(corpus, tok) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	return agents[bestIdx].ID
}

// tokenize splits text into lowercase alphabetic tokens of at least 3 chars,
// filtering out common stop words that carry no routing signal.
func tokenize(text string) []string {
	stop := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"is": true, "it": true, "to": true, "of": true, "in": true,
		"for": true, "on": true, "with": true, "me": true, "my": true,
		"can": true, "you": true, "how": true, "what": true, "that": true,
		"por": true, "para": true, "que": true, "com": true, "uma": true,
		"um": true, "de": true, "do": true, "da": true, "em": true,
	}

	var tokens []string
	current := strings.Builder{}

	flush := func() {
		if current.Len() >= 3 {
			tok := current.String()
			if !stop[tok] {
				tokens = append(tokens, tok)
			}
		}
		current.Reset()
	}

	for _, r := range strings.ToLower(text) {
		if unicode.IsLetter(r) {
			current.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return tokens
}
