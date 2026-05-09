package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/audit"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// htmlDangerousPattern matches dangerous HTML elements including their content.
// These are stripped completely (tag + content) because their inner text is executable.
var htmlDangerousPattern = regexp.MustCompile(`(?is)<(script|style|iframe|object|embed|noscript)[^>]*>.*?</(script|style|iframe|object|embed|noscript)>`)

// htmlTagPattern matches any remaining HTML tag including attributes.
var htmlTagPattern = regexp.MustCompile(`<[^>]*>`)

// slugPattern enforces kebab-case: lowercase letters, digits and hyphens.
// Must start with alphanumeric to avoid leading-hyphen collisions.
var slugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// stripHTML removes all HTML from s.
// Dangerous elements (script, style, etc.) are removed including their content.
// Other tags are stripped but their text content is preserved.
// P-C280-1: prevents stored XSS in name/description fields.
func stripHTML(s string) string {
	// Step 1: remove dangerous elements including their inner text.
	s = htmlDangerousPattern.ReplaceAllString(s, "")
	// Step 2: strip remaining HTML tags, keeping their text content.
	s = htmlTagPattern.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

// Service defines business logic operations for Agent.
type Service interface {
	List(ctx context.Context, status AgentStatus, q string, req pagination.PageRequest) (pagination.Page[AgentResponse], error)
	Get(ctx context.Context, id uuid.UUID) (AgentResponse, error)
	// GetWithReadiness returns an agent with its computed ReadinessScore attached.
	// Used by GET /api/agents/:id so the frontend can surface actionable feedback.
	GetWithReadiness(ctx context.Context, id uuid.UUID) (AgentResponse, error)
	Create(ctx context.Context, req CreateAgentRequest) (AgentResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateAgentRequest) (AgentResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// BulkDelete deletes multiple agents by ID. Returns the count of successfully deleted agents.
	// Partial success is allowed — agents that do not exist are silently skipped (ACT-F3-19).
	BulkDelete(ctx context.Context, ids []uuid.UUID) (int, error)
	Publish(ctx context.Context, id uuid.UUID) (AgentResponse, error)
	Archive(ctx context.Context, id uuid.UUID) (AgentResponse, error)
	// Restore transitions an ARCHIVED agent back to DRAFT, allowing it to be
	// reconfigured and republished without creating a new agent (BUG-ARCHIVE-NOREACTIVATE).
	Restore(ctx context.Context, id uuid.UUID) (AgentResponse, error)
	Clone(ctx context.Context, id uuid.UUID, req CloneAgentRequest) (AgentResponse, error)
}

type service struct {
	repo        Repository
	bindingRepo BindingRepository
	skillRepo   skill.SkillRepository
	audit       AuditRecorder
}

// NewService creates a new agent Service.
func NewService(repo Repository, bindingRepo BindingRepository, skillRepo skill.SkillRepository) Service {
	return NewServiceWithAudit(repo, bindingRepo, skillRepo, nil)
}

// AuditRecorder is the subset of the audit service used by the agent domain.
type AuditRecorder interface {
	Record(ctx context.Context, tenantID string, req audit.RecordRequest) (audit.AuditLog, error)
}

// NewServiceWithAudit creates a new agent Service with optional audit logging.
func NewServiceWithAudit(repo Repository, bindingRepo BindingRepository, skillRepo skill.SkillRepository, auditRecorder AuditRecorder) Service {
	return &service{repo: repo, bindingRepo: bindingRepo, skillRepo: skillRepo, audit: auditRecorder}
}

func recordAudit(ctx context.Context, recorder AuditRecorder, req audit.RecordRequest) {
	if recorder == nil {
		return
	}
	tenantID := tenantctx.FromContext(ctx)
	if tenantID == "" {
		return
	}
	if _, err := recorder.Record(ctx, tenantID, req); err != nil {
		slog.Warn("agent: failed to record audit log", "entity_type", req.EntityType, "entity_id", req.EntityID, "action", req.Action, "error", err)
	}
}

// auditJSON marshals v for the audit_log.old_value / new_value JSONB columns.
// Those columns reject the empty string (SQLSTATE 22P02 "invalid input syntax
// for type json"), so fall back to a valid JSON null whenever marshalling
// fails — we'd rather record a null than drop the entire audit row.
func auditJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil || len(raw) == 0 {
		return "null"
	}
	return string(raw)
}

func (s *service) List(ctx context.Context, status AgentStatus, q string, req pagination.PageRequest) (pagination.Page[AgentResponse], error) {
	agents, total, err := s.repo.FindAll(ctx, status, q, req)
	if err != nil {
		return pagination.Page[AgentResponse]{}, err
	}
	responses := make([]AgentResponse, len(agents))
	for i, a := range agents {
		responses[i] = ResponseFrom(a)
	}
	return pagination.NewPage(responses, total, req), nil
}

func (s *service) Get(ctx context.Context, id uuid.UUID) (AgentResponse, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return AgentResponse{}, err
	}
	resp := ResponseFrom(a)
	if skillIDs, err := s.bindingRepo.ListSkillIDs(ctx, id); err == nil {
		resp.SkillIDs = skillIDs
	}
	if kbIDs, err := s.bindingRepo.ListKnowledgeBaseIDs(ctx, id); err == nil {
		resp.KnowledgeBaseIDs = kbIDs
	}
	return resp, nil
}

// GetWithReadiness returns the agent with its computed ReadinessScore attached.
// The score reflects the current binding state (skills + tools).
func (s *service) GetWithReadiness(ctx context.Context, id uuid.UUID) (AgentResponse, error) {
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return AgentResponse{}, err
	}
	resp := ResponseFrom(a)
	var skillIDs []uuid.UUID
	if ids, err := s.bindingRepo.ListSkillIDs(ctx, id); err == nil {
		skillIDs = ids
		resp.SkillIDs = skillIDs
	}
	if kbIDs, err := s.bindingRepo.ListKnowledgeBaseIDs(ctx, id); err == nil {
		resp.KnowledgeBaseIDs = kbIDs
	}
	if score, err := s.agentReadiness(ctx, a, skillIDs); err == nil {
		resp.Readiness = &score
	}
	return resp, nil
}

// agentReadiness computes ReadinessScore for agent a given its current skill bindings.
// activeToolCount is fetched from the skill repository for the provided skillIDs.
func (s *service) agentReadiness(ctx context.Context, a Agent, skillIDs []uuid.UUID) (ReadinessScore, error) {
	activeToolCount := 0
	if len(skillIDs) > 0 {
		count, err := s.skillRepo.CountActiveToolsForSkills(ctx, skillIDs)
		if err != nil {
			return ReadinessScore{}, fmt.Errorf("agent: readiness: count tools: %w", err)
		}
		activeToolCount = count
	}
	return ComputeReadiness(a, len(skillIDs), activeToolCount), nil
}

// maxSystemPromptChars is the maximum allowed length for an agent's system prompt.
// ACT-F3-05 (P-C182-3, P-C150-2): large prompts consume context window and slow LLM responses.
const maxSystemPromptChars = 10000

func (s *service) Create(ctx context.Context, req CreateAgentRequest) (AgentResponse, error) {
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return AgentResponse{}, fmt.Errorf("name is required")
	}
	if len(req.Name) > 255 {
		return AgentResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrInvalidRequest, len(req.Name))
	}
	// ACT-F3-05: enforce maximum system prompt size.
	if req.SystemPrompt != nil && len(*req.SystemPrompt) > maxSystemPromptChars {
		return AgentResponse{}, fmt.Errorf("%w: systemPrompt exceeds maximum length of %d chars (got %d)", ErrInvalidRequest, maxSystemPromptChars, len(*req.SystemPrompt))
	}
	// P-C249-2: reject modelConfig nested inside the config field. Clients must
	// send modelConfig at the root level of the request body.
	if hasNestedModelConfig(req.Config) {
		return AgentResponse{}, fmt.Errorf("%w: modelConfig must be at the root of the request body, not inside config", ErrInvalidRequest)
	}
	// P-C97-1: reject invalid modelConfig at creation time so the agent is never
	// stored in a broken state (e.g. maxIterations=-5 makes the loop exit immediately).
	if err := validateModelConfig(req.ModelConfig); err != nil {
		return AgentResponse{}, fmt.Errorf("%w: %s", ErrInvalidModelConfig, err)
	}
	// P-C297-1: validate config.maxIterations even when nested in agent config
	// (separate from modelConfig). Range [1, 100].
	if err := validateConfigMaxIterations(req.Config); err != nil {
		return AgentResponse{}, fmt.Errorf("%w: %s", ErrInvalidRequest, err)
	}
	// P-C280-1: strip HTML from user-supplied text fields before persisting.
	req.Name = stripHTML(req.Name)
	req.Description = stripHTML(req.Description)
	// Bug 158: cap description em 32KB (espelha skill.instructions cap).
	// Sem isso 200KB+ aceita silenciosamente — DoS storage e perf
	// hit em listings.
	if len(req.Description) > 32000 {
		return AgentResponse{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrInvalidRequest, len(req.Description))
	}
	slug := req.Slug
	if slug == "" {
		slug = toSlug(req.Name)
	} else if !slugPattern.MatchString(slug) {
		return AgentResponse{}, fmt.Errorf("%w: slug must match [a-z0-9][a-z0-9-]* (got %q)", ErrInvalidRequest, slug)
	}
	if len(slug) > 255 {
		return AgentResponse{}, fmt.Errorf("%w: slug exceeds maximum length of 255 chars (got %d)", ErrInvalidRequest, len(slug))
	}
	config := req.Config
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	a := Agent{
		ID:               uuid.New(),
		Name:             req.Name,
		Slug:             slug,
		Description:      req.Description,
		Status:           StatusDraft,
		CurrentVersion:   1,
		SystemPrompt:     req.SystemPrompt,
		ModelConfig:      req.ModelConfig,
		PermissionRules:  req.PermissionRules,
		Config:           config,
		EnableManagement: req.EnableManagement,
	}
	created, err := s.repo.Create(ctx, a)
	if err != nil {
		return AgentResponse{}, err
	}
	resp := ResponseFrom(created)
	// Link skills provided in the creation request (P-C60-1 fix).
	if len(req.SkillIDs) > 0 {
		if syncErr := s.bindingRepo.SyncSkills(ctx, created.ID, req.SkillIDs); syncErr != nil {
			// Roll back by deleting the just-created agent so the caller sees a clean failure.
			_ = s.repo.Delete(ctx, created.ID)
			return AgentResponse{}, fmt.Errorf("%w: %w", ErrInvalidSkillIDs, syncErr)
		}
		resp.SkillIDs = req.SkillIDs
	}
	// P-C285-1: link knowledge bases provided in the creation request.
	if len(req.KnowledgeBaseIDs) > 0 {
		if syncErr := s.bindingRepo.SyncKnowledgeBases(ctx, created.ID, req.KnowledgeBaseIDs); syncErr != nil {
			_ = s.repo.Delete(ctx, created.ID)
			if errors.Is(syncErr, ErrInvalidKnowledgeBaseIDs) {
				return AgentResponse{}, syncErr
			}
			return AgentResponse{}, fmt.Errorf("%w: %w", ErrInvalidKnowledgeBaseIDs, syncErr)
		}
		resp.KnowledgeBaseIDs = req.KnowledgeBaseIDs
	}
	recordAudit(ctx, s.audit, audit.RecordRequest{
		EntityType: "agent",
		EntityID:   created.ID.String(),
		Action:     audit.AuditActionCreate,
		NewValue:   auditJSON(resp),
	})
	return resp, nil
}

func (s *service) Update(ctx context.Context, id uuid.UUID, req UpdateAgentRequest) (AgentResponse, error) {
	// P-C249-2: same guard as Create — reject nested modelConfig.
	if hasNestedModelConfig(req.Config) {
		return AgentResponse{}, fmt.Errorf("%w: modelConfig must be at the root of the request body, not inside config", ErrInvalidRequest)
	}
	// ACT-F3-05: enforce maximum system prompt size on update.
	if req.SystemPrompt != nil && len(*req.SystemPrompt) > maxSystemPromptChars {
		return AgentResponse{}, fmt.Errorf("%w: systemPrompt exceeds maximum length of %d chars (got %d)", ErrInvalidRequest, maxSystemPromptChars, len(*req.SystemPrompt))
	}
	a, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return AgentResponse{}, err
	}
	before := ResponseFrom(a)
	if req.Name != nil {
		// P-C280-1: strip HTML from user-supplied text fields.
		trimmed := strings.TrimSpace(stripHTML(*req.Name))
		if trimmed == "" {
			// Bug 116: era 500 — wrap em ErrInvalidRequest pra
			// handler mapear → 422.
			return AgentResponse{}, fmt.Errorf("%w: name is required", ErrInvalidRequest)
		}
		// Bug 136: name varchar(255) — gate length em Update (Create já gateava).
		if len(trimmed) > 255 {
			return AgentResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrInvalidRequest, len(trimmed))
		}
		a.Name = trimmed
	}
	if req.Slug != nil {
		// Bug 121: Update precisa do mesmo gate que Create —
		// pattern [a-z0-9][a-z0-9-]* + length <= 255. Sem isso admin
		// podia salvar slug="INVALID!" via PATCH e quebrar lookups.
		if !slugPattern.MatchString(*req.Slug) {
			return AgentResponse{}, fmt.Errorf("%w: slug must match [a-z0-9][a-z0-9-]* (got %q)", ErrInvalidRequest, *req.Slug)
		}
		if len(*req.Slug) > 255 {
			return AgentResponse{}, fmt.Errorf("%w: slug exceeds maximum length of 255 chars (got %d)", ErrInvalidRequest, len(*req.Slug))
		}
		a.Slug = *req.Slug
	}
	if req.Description != nil {
		// P-C280-1: strip HTML from user-supplied text fields.
		desc := stripHTML(*req.Description)
		// Bug 158: mesmo cap 32KB do Create.
		if len(desc) > 32000 {
			return AgentResponse{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrInvalidRequest, len(desc))
		}
		a.Description = desc
	}
	if req.SystemPrompt != nil {
		a.SystemPrompt = req.SystemPrompt
	}
	if len(req.ModelConfig) > 0 {
		if err := validateModelConfig(req.ModelConfig); err != nil {
			return AgentResponse{}, fmt.Errorf("%w: %s", ErrInvalidModelConfig, err)
		}
		a.ModelConfig = req.ModelConfig
	}
	if len(req.PermissionRules) > 0 {
		a.PermissionRules = req.PermissionRules
	}
	if len(req.Config) > 0 {
		// Bug 107: Update precisa do mesmo gate de validateConfigMaxIterations
		// que Create — senão admin podia criar agent sano e depois PATCH
		// com maxIterations=-5 (loop sai imediatamente) ou =99999 (custos
		// LLM descontrolados).
		if err := validateConfigMaxIterations(req.Config); err != nil {
			return AgentResponse{}, fmt.Errorf("%w: %s", ErrInvalidRequest, err)
		}
		a.Config = req.Config
	}
	if req.EnableManagement != nil {
		a.EnableManagement = *req.EnableManagement
	}
	updated, err := s.repo.Update(ctx, a)
	if err != nil {
		return AgentResponse{}, err
	}
	resp := ResponseFrom(updated)
	if req.SkillIDs != nil {
		if syncErr := s.bindingRepo.SyncSkills(ctx, id, req.SkillIDs); syncErr != nil {
			return AgentResponse{}, syncErr
		}
		resp.SkillIDs = req.SkillIDs
	} else if skillIDs, err := s.bindingRepo.ListSkillIDs(ctx, id); err == nil {
		resp.SkillIDs = skillIDs
	}
	// P-C285-1: sync knowledge base bindings when KnowledgeBaseIDs is explicitly provided.
	if req.KnowledgeBaseIDs != nil {
		if syncErr := s.bindingRepo.SyncKnowledgeBases(ctx, id, req.KnowledgeBaseIDs); syncErr != nil {
			return AgentResponse{}, syncErr
		}
		resp.KnowledgeBaseIDs = req.KnowledgeBaseIDs
	} else if kbIDs, err := s.bindingRepo.ListKnowledgeBaseIDs(ctx, id); err == nil {
		resp.KnowledgeBaseIDs = kbIDs
	}
	recordAudit(ctx, s.audit, audit.RecordRequest{
		EntityType: "agent",
		EntityID:   id.String(),
		Action:     audit.AuditActionUpdate,
		OldValue:   auditJSON(before),
		NewValue:   auditJSON(resp),
	})
	return resp, nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	recordAudit(ctx, s.audit, audit.RecordRequest{
		EntityType: "agent",
		EntityID:   id.String(),
		Action:     audit.AuditActionDelete,
		OldValue:   auditJSON(ResponseFrom(existing)),
	})
	return nil
}

// BulkDelete deletes agents by their IDs, silently skipping those not found.
// Returns the number of agents successfully deleted.
func (s *service) BulkDelete(ctx context.Context, ids []uuid.UUID) (int, error) {
	deleted := 0
	for _, id := range ids {
		existing, err := s.repo.FindByID(ctx, id)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return deleted, fmt.Errorf("agent: bulk delete: %w", err)
		}
		if err := s.repo.Delete(ctx, id); err != nil {
			if errors.Is(err, ErrNotFound) {
				continue // not found — skip silently
			}
			return deleted, fmt.Errorf("agent: bulk delete: %w", err)
		}
		recordAudit(ctx, s.audit, audit.RecordRequest{
			EntityType: "agent",
			EntityID:   id.String(),
			Action:     audit.AuditActionDelete,
			OldValue:   auditJSON(ResponseFrom(existing)),
		})
		deleted++
	}
	return deleted, nil
}

func (s *service) Publish(ctx context.Context, id uuid.UUID) (AgentResponse, error) {
	// P-C278-1: validate agent state before publishing.
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return AgentResponse{}, err
	}
	if err := validatePublishStatus(current); err != nil {
		return AgentResponse{}, err
	}
	// Readiness gate: require at least STANDARD (score ≥ 60) to publish.
	skillIDs, err := s.bindingRepo.ListSkillIDs(ctx, id)
	if err != nil {
		return AgentResponse{}, fmt.Errorf("agent: publish: list skills: %w", err)
	}
	score, err := s.agentReadiness(ctx, current, skillIDs)
	if err != nil {
		return AgentResponse{}, err
	}
	const minPublishScore = 60
	if score.Score < minPublishScore {
		return AgentResponse{}, fmt.Errorf(
			"%w: readiness score %d/%d (level: %s) — address the failing checks before publishing",
			ErrInvalidRequest, score.Score, minPublishScore, score.Level,
		)
	}
	a, err := s.repo.UpdateStatus(ctx, id, StatusPublished)
	if err != nil {
		return AgentResponse{}, err
	}
	resp := ResponseFrom(a)
	recordAudit(ctx, s.audit, audit.RecordRequest{
		EntityType: "agent",
		EntityID:   id.String(),
		Action:     audit.AuditActionUpdate,
		OldValue:   auditJSON(ResponseFrom(current)),
		NewValue:   auditJSON(resp),
		Metadata:   `{"operation":"publish"}`,
	})
	return resp, nil
}

// validatePublishStatus checks that the agent's current status allows publishing.
// P-C278-1: only DRAFT agents can be published; ARCHIVED agents require an explicit
// status reset first (no direct archive→publish path exists); already PUBLISHED agents
// are rejected to avoid duplicate publishes.
func validatePublishStatus(a Agent) error {
	switch a.Status {
	case StatusDraft:
		// allowed — readiness check follows in service.Publish
	case StatusPublished:
		return fmt.Errorf("%w: agent is already published", ErrInvalidStatusTransition)
	case StatusArchived:
		return fmt.Errorf("%w: archived agents cannot be published directly", ErrInvalidStatusTransition)
	default:
		return fmt.Errorf("%w: unknown status %q", ErrInvalidStatusTransition, a.Status)
	}
	if a.Name == "" {
		return fmt.Errorf("%w: agent name is required before publishing", ErrInvalidRequest)
	}
	return nil
}

func (s *service) Archive(ctx context.Context, id uuid.UUID) (AgentResponse, error) {
	current, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return AgentResponse{}, err
	}
	a, err := s.repo.UpdateStatus(ctx, id, StatusArchived)
	if err != nil {
		return AgentResponse{}, err
	}
	resp := ResponseFrom(a)
	recordAudit(ctx, s.audit, audit.RecordRequest{
		EntityType: "agent",
		EntityID:   id.String(),
		Action:     audit.AuditActionUpdate,
		OldValue:   auditJSON(ResponseFrom(current)),
		NewValue:   auditJSON(resp),
		Metadata:   `{"operation":"archive"}`,
	})
	return resp, nil
}

// Restore transitions an ARCHIVED agent back to DRAFT status.
// Only ARCHIVED agents can be restored; other statuses return ErrInvalidStatusTransition.
func (s *service) Restore(ctx context.Context, id uuid.UUID) (AgentResponse, error) {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return AgentResponse{}, err
	}
	if existing.Status != StatusArchived {
		return AgentResponse{}, fmt.Errorf("%w: only archived agents can be restored (current status: %s)", ErrInvalidStatusTransition, existing.Status)
	}
	a, err := s.repo.UpdateStatus(ctx, id, StatusDraft)
	if err != nil {
		return AgentResponse{}, err
	}
	resp := ResponseFrom(a)
	recordAudit(ctx, s.audit, audit.RecordRequest{
		EntityType: "agent",
		EntityID:   id.String(),
		Action:     audit.AuditActionUpdate,
		OldValue:   auditJSON(ResponseFrom(existing)),
		NewValue:   auditJSON(resp),
		Metadata:   `{"operation":"restore"}`,
	})
	return resp, nil
}

func (s *service) Clone(ctx context.Context, id uuid.UUID, req CloneAgentRequest) (AgentResponse, error) {
	original, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return AgentResponse{}, err
	}
	name := req.Name
	if name == "" {
		name = original.Name + " (copy)"
	}
	// Bug 128: name varchar(255). Sem este gate, Clone com name >255 chars
	// retornava 500 com SQL error 22001 vazando para o cliente. Mesmo gate
	// que Create deveria aplicar — falha rápido, mensagem útil, 422.
	if len(name) > 255 {
		return AgentResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrInvalidRequest, len(name))
	}
	clone := Agent{
		ID:              uuid.New(),
		Name:            name,
		Slug:            toSlug(name),
		Description:     original.Description,
		Status:          StatusDraft,
		CurrentVersion:  1,
		SystemPrompt:    original.SystemPrompt,
		ModelConfig:     original.ModelConfig,
		PermissionRules: original.PermissionRules,
		Config:          original.Config,
	}
	created, err := s.repo.Create(ctx, clone)
	if err != nil {
		return AgentResponse{}, err
	}

	// BUG-CLONE1: copy skill and knowledge-base bindings from the original agent
	// so the clone is a complete functional copy, not just a metadata copy.
	if skillIDs, err := s.bindingRepo.ListSkillIDs(ctx, id); err == nil && len(skillIDs) > 0 {
		if syncErr := s.bindingRepo.SyncSkills(ctx, created.ID, skillIDs); syncErr != nil {
			// Non-fatal: log but don't fail the clone operation.
			_ = syncErr
		}
	}
	if kbIDs, err := s.bindingRepo.ListKnowledgeBaseIDs(ctx, id); err == nil && len(kbIDs) > 0 {
		if syncErr := s.bindingRepo.SyncKnowledgeBases(ctx, created.ID, kbIDs); syncErr != nil {
			_ = syncErr
		}
	}

	resp, err := s.Get(ctx, created.ID)
	if err != nil {
		return AgentResponse{}, err
	}
	recordAudit(ctx, s.audit, audit.RecordRequest{
		EntityType: "agent",
		EntityID:   created.ID.String(),
		Action:     audit.AuditActionCreate,
		NewValue:   auditJSON(resp),
		Metadata:   fmt.Sprintf(`{"operation":"clone","sourceAgentId":"%s"}`, id),
	})
	return resp, nil
}

// SupportedProviders lists the LLM providers recognised by the platform runner.
// P-C268-1, P-C327-3: provider is validated at agent creation/update time.
var SupportedProviders = []string{
	"openai",
	"openrouter",
	"anthropic",
	"ollama",
	"azure-openai",
	"google",
}

// ErrUnsupportedProvider is returned when modelConfig.provider is not in SupportedProviders.
var ErrUnsupportedProvider = fmt.Errorf("unsupported LLM provider")

// validateModelConfig checks that modelConfig contains valid JSON and that
// numeric fields are within safe ranges. Returns nil when raw is empty.
// P-C97-1: prevents agents with broken model_config from being stored.
// P-C294-2: validates provider/model consistency — if one is set, both must be.
// P-C268-1: validates provider against supported enum.
func validateModelConfig(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	// Bug 157: cap modelConfig em 16KB — qualquer config LLM real cabe
	// folgado nesse limite; 1MB+ é payload malformado e degrada perf
	// (cada agent load + serialize fica O(N) com N de modelConfig).
	if len(raw) > 16*1024 {
		return fmt.Errorf("modelConfig exceeds maximum size of 16KB (got %d bytes)", len(raw))
	}
	// Must be a valid JSON object (not a string, array, etc.)
	var mc struct {
		Provider         string   `json:"provider"`
		Model            string   `json:"model"`
		MaxIterations    *int     `json:"maxIterations"`
		MaxTokens        *int     `json:"maxTokens"`
		ContextWindow    *int     `json:"contextWindow"`
		MaxDepth         *int     `json:"maxDepth"`
		Temperature      *float64 `json:"temperature"`
		TopP             *float64 `json:"topP"`
		FrequencyPenalty *float64 `json:"frequencyPenalty"`
		PresencePenalty  *float64 `json:"presencePenalty"`
	}
	if err := json.Unmarshal(raw, &mc); err != nil {
		return fmt.Errorf("must be a valid JSON object")
	}
	// P-C268-1: validate provider against supported enum.
	if mc.Provider != "" {
		supported := false
		for _, p := range SupportedProviders {
			if mc.Provider == p {
				supported = true
				break
			}
		}
		if !supported {
			return fmt.Errorf("%w: %q — supported providers: %s",
				ErrUnsupportedProvider, mc.Provider, strings.Join(SupportedProviders, ", "))
		}
	}
	// P-C294-2: provider and model are a pair — both or neither.
	if mc.Provider != "" && mc.Model == "" {
		return fmt.Errorf("model is required when provider is specified")
	}
	if mc.Model != "" && mc.Provider == "" {
		return fmt.Errorf("provider is required when model is specified")
	}
	if mc.MaxIterations != nil && (*mc.MaxIterations <= 0 || *mc.MaxIterations > 100) {
		return fmt.Errorf("maxIterations must be between 1 and 100 (got %d)", *mc.MaxIterations)
	}
	if mc.MaxTokens != nil && (*mc.MaxTokens <= 0 || *mc.MaxTokens > 2_000_000) {
		// Bug 152: cap em 2M — LLMs reais aceitam até ~128k-1M; valor
		// maior é payload malformado e gera custo/timeout no provider.
		return fmt.Errorf("maxTokens must be between 1 and 2000000 (got %d)", *mc.MaxTokens)
	}
	if mc.ContextWindow != nil && (*mc.ContextWindow <= 0 || *mc.ContextWindow > 2_000_000) {
		return fmt.Errorf("contextWindow must be between 1 and 2000000 (got %d)", *mc.ContextWindow)
	}
	if mc.MaxDepth != nil && *mc.MaxDepth < 0 {
		return fmt.Errorf("maxDepth must be a non-negative integer (got %d)", *mc.MaxDepth)
	}
	if mc.Temperature != nil && (*mc.Temperature < 0 || *mc.Temperature > 2.0) {
		return fmt.Errorf("temperature must be between 0.0 and 2.0 (got %g)", *mc.Temperature)
	}
	// Bug 123: validar topP/frequencyPenalty/presencePenalty contra os ranges
	// da spec OpenAI/Anthropic — provider silenciosamente coerce ou falha
	// em runtime se valores absurdos chegarem ao LLM.
	if mc.TopP != nil && (*mc.TopP < 0 || *mc.TopP > 1.0) {
		return fmt.Errorf("topP must be between 0.0 and 1.0 (got %g)", *mc.TopP)
	}
	if mc.FrequencyPenalty != nil && (*mc.FrequencyPenalty < -2.0 || *mc.FrequencyPenalty > 2.0) {
		return fmt.Errorf("frequencyPenalty must be between -2.0 and 2.0 (got %g)", *mc.FrequencyPenalty)
	}
	if mc.PresencePenalty != nil && (*mc.PresencePenalty < -2.0 || *mc.PresencePenalty > 2.0) {
		return fmt.Errorf("presencePenalty must be between -2.0 and 2.0 (got %g)", *mc.PresencePenalty)
	}
	return nil
}

// toSlug converts a name to a kebab-case slug.
func toSlug(name string) string {
	s := strings.ToLower(name)
	// Replace non-alphanumeric characters with hyphens.
	var b strings.Builder
	prevHyphen := true
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
			prevHyphen = false
		} else if !prevHyphen {
			b.WriteRune('-')
			prevHyphen = true
		}
	}
	result := strings.TrimRight(b.String(), "-")
	if result == "" {
		return "agent-" + uuid.New().String()[:8]
	}
	return result
}

// VersionService defines business logic for AgentVersion.
type VersionService interface {
	CreateDraft(ctx context.Context, agentID uuid.UUID, req CreateAgentVersionRequest) (AgentVersionResponse, error)
	UpdateDraft(ctx context.Context, versionID uuid.UUID, req UpdateAgentVersionRequest) (AgentVersionResponse, error)
	Publish(ctx context.Context, versionID uuid.UUID) (AgentVersionResponse, error)
	GetDraft(ctx context.Context, agentID uuid.UUID) (AgentVersionResponse, error)
	GetLatestPublished(ctx context.Context, agentID uuid.UUID) (AgentVersionResponse, error)
	ListVersions(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) (pagination.Page[AgentVersionResponse], error)
	// Rollback restores an agent's system prompt and model config from a previously
	// published version and creates a new version entry for the rollback.
	Rollback(ctx context.Context, agentID, versionID uuid.UUID) (AgentVersionResponse, error)
	// GetVersionByID retrieves any version by its UUID regardless of status.
	GetVersionByID(ctx context.Context, versionID uuid.UUID) (AgentVersionResponse, error)
}

type versionService struct {
	repo    Repository
	verRepo VersionRepository
	audit   AuditRecorder
}

// NewVersionService creates a new VersionService.
func NewVersionService(repo Repository, verRepo VersionRepository) VersionService {
	return NewVersionServiceWithAudit(repo, verRepo, nil)
}

// NewVersionServiceWithAudit creates a new VersionService with optional audit logging.
func NewVersionServiceWithAudit(repo Repository, verRepo VersionRepository, auditRecorder AuditRecorder) VersionService {
	return &versionService{repo: repo, verRepo: verRepo, audit: auditRecorder}
}

func (s *versionService) CreateDraft(ctx context.Context, agentID uuid.UUID, req CreateAgentVersionRequest) (AgentVersionResponse, error) {
	// Ensure the agent exists.
	if _, err := s.repo.FindByID(ctx, agentID); err != nil {
		return AgentVersionResponse{}, err
	}
	// Ensure no existing draft.
	if _, err := s.verRepo.FindDraft(ctx, agentID); err == nil {
		return AgentVersionResponse{}, ErrDraftAlreadyExists
	}
	num, err := s.verRepo.NextVersionNumber(ctx, agentID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	v := AgentVersion{
		ID:             uuid.New(),
		AgentID:        agentID,
		VersionNumber:  num,
		Status:         VersionStatusDraft,
		Description:    req.Description,
		DefinitionJSON: req.DefinitionJSON,
		ConfigJSON:     req.ConfigJSON,
	}
	created, err := s.verRepo.Create(ctx, v)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	resp := VersionResponseFrom(created)
	recordAudit(ctx, s.audit, audit.RecordRequest{
		EntityType: "agent_version",
		EntityID:   created.ID.String(),
		Action:     audit.AuditActionCreate,
		NewValue:   auditJSON(resp),
		Metadata:   fmt.Sprintf(`{"operation":"create_draft","agentId":"%s"}`, agentID),
	})
	return resp, nil
}

func (s *versionService) UpdateDraft(ctx context.Context, versionID uuid.UUID, req UpdateAgentVersionRequest) (AgentVersionResponse, error) {
	v, err := s.verRepo.FindByID(ctx, versionID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	if v.Status != VersionStatusDraft {
		return AgentVersionResponse{}, ErrVersionImmutable
	}
	if req.Description != nil {
		v.Description = *req.Description
	}
	if len(req.DefinitionJSON) > 0 {
		v.DefinitionJSON = req.DefinitionJSON
	}
	if len(req.ConfigJSON) > 0 {
		v.ConfigJSON = req.ConfigJSON
	}
	updated, err := s.verRepo.Update(ctx, v)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	resp := VersionResponseFrom(updated)
	recordAudit(ctx, s.audit, audit.RecordRequest{
		EntityType: "agent_version",
		EntityID:   versionID.String(),
		Action:     audit.AuditActionUpdate,
		NewValue:   auditJSON(resp),
		Metadata:   `{"operation":"update_draft"}`,
	})
	return resp, nil
}

func (s *versionService) Publish(ctx context.Context, versionID uuid.UUID) (AgentVersionResponse, error) {
	v, err := s.verRepo.FindByID(ctx, versionID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	if v.Status != VersionStatusDraft {
		return AgentVersionResponse{}, ErrVersionImmutable
	}
	published, err := s.verRepo.Publish(ctx, versionID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	resp := VersionResponseFrom(published)
	recordAudit(ctx, s.audit, audit.RecordRequest{
		EntityType: "agent_version",
		EntityID:   versionID.String(),
		Action:     audit.AuditActionUpdate,
		NewValue:   auditJSON(resp),
		Metadata:   `{"operation":"publish"}`,
	})
	return resp, nil
}

func (s *versionService) GetDraft(ctx context.Context, agentID uuid.UUID) (AgentVersionResponse, error) {
	v, err := s.verRepo.FindDraft(ctx, agentID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	return VersionResponseFrom(v), nil
}

func (s *versionService) GetLatestPublished(ctx context.Context, agentID uuid.UUID) (AgentVersionResponse, error) {
	v, err := s.verRepo.FindLatestPublished(ctx, agentID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	return VersionResponseFrom(v), nil
}

func (s *versionService) ListVersions(ctx context.Context, agentID uuid.UUID, req pagination.PageRequest) (pagination.Page[AgentVersionResponse], error) {
	versions, total, err := s.verRepo.FindByAgentID(ctx, agentID, req)
	if err != nil {
		return pagination.Page[AgentVersionResponse]{}, err
	}
	responses := make([]AgentVersionResponse, len(versions))
	for i, v := range versions {
		responses[i] = VersionResponseFrom(v)
	}
	return pagination.NewPage(responses, total, req), nil
}

// Rollback restores agent configuration from a previously published version.
// It creates a new PUBLISHED version entry that documents the rollback, and
// applies the snapshot's definitionJson/configJson to the live agent record.
func (s *versionService) Rollback(ctx context.Context, agentID, versionID uuid.UUID) (AgentVersionResponse, error) {
	// 1. Load the target version — must be PUBLISHED.
	target, err := s.verRepo.FindByID(ctx, versionID)
	if err != nil {
		return AgentVersionResponse{}, ErrVersionNotFound
	}
	if target.AgentID != agentID {
		return AgentVersionResponse{}, ErrVersionNotFound
	}
	if target.Status != VersionStatusPublished {
		return AgentVersionResponse{}, fmt.Errorf("only published versions can be rolled back to")
	}

	// 2. Ensure no active draft exists — rollback is not allowed with a pending draft.
	if _, err := s.verRepo.FindDraft(ctx, agentID); err == nil {
		return AgentVersionResponse{}, ErrRollbackBlockedByDraft
	}

	// 3. Create a new PUBLISHED version recording the rollback.
	num, err := s.verRepo.NextVersionNumber(ctx, agentID)
	if err != nil {
		return AgentVersionResponse{}, err
	}
	now := time.Now()
	rollbackEntry := AgentVersion{
		ID:             uuid.New(),
		AgentID:        agentID,
		VersionNumber:  num,
		Status:         VersionStatusPublished,
		Description:    fmt.Sprintf("Rollback to version %d", target.VersionNumber),
		DefinitionJSON: target.DefinitionJSON,
		ConfigJSON:     target.ConfigJSON,
		PublishedAt:    &now,
	}
	created, err := s.verRepo.Create(ctx, rollbackEntry)
	if err != nil {
		return AgentVersionResponse{}, fmt.Errorf("rollback: create version entry: %w", err)
	}
	// Mark it published immediately.
	created, err = s.verRepo.Publish(ctx, created.ID)
	if err != nil {
		return AgentVersionResponse{}, fmt.Errorf("rollback: publish version entry: %w", err)
	}

	// 4. Apply snapshot to the live agent — extract systemPrompt from definitionJson.
	var def struct {
		SystemPrompt string `json:"systemPrompt"`
	}
	if len(target.DefinitionJSON) > 0 {
		_ = json.Unmarshal(target.DefinitionJSON, &def)
	}
	if def.SystemPrompt != "" {
		current, err := s.repo.FindByID(ctx, agentID)
		if err != nil {
			return AgentVersionResponse{}, fmt.Errorf("rollback: load agent: %w", err)
		}
		current.SystemPrompt = &def.SystemPrompt
		if _, err := s.repo.Update(ctx, current); err != nil {
			return AgentVersionResponse{}, fmt.Errorf("rollback: apply system prompt: %w", err)
		}
	}

	resp := VersionResponseFrom(created)
	recordAudit(ctx, s.audit, audit.RecordRequest{
		EntityType: "agent",
		EntityID:   agentID.String(),
		Action:     audit.AuditActionUpdate,
		NewValue:   auditJSON(resp),
		Metadata:   fmt.Sprintf(`{"operation":"rollback","targetVersionId":"%s","createdVersionId":"%s"}`, versionID, created.ID),
	})
	return resp, nil
}

func (s *versionService) GetVersionByID(ctx context.Context, versionID uuid.UUID) (AgentVersionResponse, error) {
	v, err := s.verRepo.FindByID(ctx, versionID)
	if err != nil {
		return AgentVersionResponse{}, ErrVersionNotFound
	}
	return VersionResponseFrom(v), nil
}

// validateConfigMaxIterations checks that config.maxIterations (when present)
// is in the range [1, 100]. Mirror of the modelConfig.maxIterations check but
// applied to the agent config blob. Bug 96: previously bypassed when sent
// in config instead of modelConfig.
func validateConfigMaxIterations(raw json.RawMessage) error {
	if len(raw) == 0 {
		return nil
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil
	}
	v, ok := cfg["maxIterations"]
	if !ok {
		return nil
	}
	n, ok := v.(float64)
	if !ok {
		return nil
	}
	if n < 1 || n > 100 {
		return fmt.Errorf("config.maxIterations must be between 1 and 100 (got %d)", int(n))
	}
	return nil
}

// hasNestedModelConfig returns true when the given config JSON blob contains a
// top-level "modelConfig" key. P-C249-2: clients that accidentally nest modelConfig
// inside the config field would silently produce non-functional agents; reject early.
func hasNestedModelConfig(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return false
	}
	_, ok := cfg["modelConfig"]
	return ok
}
