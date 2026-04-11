package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
)

const bundleFormatVersion = "1"

// BundleSkill is the portable representation of a skill inside a bundle.
type BundleSkill struct {
	// OriginalID is the source skill UUID — stored so the importer can detect
	// duplicates and avoid creating the same skill twice on repeated installs.
	OriginalID             uuid.UUID `json:"originalId"`
	Name                   string    `json:"name"`
	Slug                   string    `json:"slug"`
	Description            string    `json:"description"`
	Instructions           string    `json:"instructions"`
	Category               string    `json:"category"`
	AllowedTools           []string  `json:"allowedTools,omitempty"`
	DisableModelInvocation bool      `json:"disableModelInvocation,omitempty"`
	ContextMode            string    `json:"contextMode,omitempty"`
	WhenToUse              *string   `json:"whenToUse,omitempty"`
	ArgumentHint           *string   `json:"argumentHint,omitempty"`
}

// AgentBundle is the portable snapshot of an agent together with its skills.
// Designed to be stored as a package asset (JSON) and re-hydrated into any tenant.
type AgentBundle struct {
	FormatVersion string          `json:"formatVersion"` // always "1"
	ExportedAt    time.Time       `json:"exportedAt"`
	Agent         BundleAgentDef  `json:"agent"`
	Skills        []BundleSkill   `json:"skills"`
}

// BundleAgentDef is the agent configuration captured in a bundle.
type BundleAgentDef struct {
	Name             string          `json:"name"`
	Slug             string          `json:"slug"`
	Description      string          `json:"description"`
	SystemPrompt     *string         `json:"systemPrompt,omitempty"`
	ModelConfig      json.RawMessage `json:"modelConfig,omitempty"`
	PermissionRules  json.RawMessage `json:"permissionRules,omitempty"`
	Config           json.RawMessage `json:"config,omitempty"`
	EnableManagement bool            `json:"enableManagement,omitempty"`
}

// ImportResult summarises the outcome of a bundle import.
type ImportResult struct {
	AgentID      uuid.UUID   `json:"agentId"`
	AgentName    string      `json:"agentName"`
	SkillsImported int       `json:"skillsImported"`
}

// --- Exporter ---

// Exporter exports an agent as a portable AgentBundle.
// It uses the agent service (for the agent) and skill repository (for its skills).
type Exporter struct {
	svc       Service
	skillRepo skill.SkillRepository
	binding   BindingRepository
}

// NewExporter creates an Exporter.
func NewExporter(svc Service, skillRepo skill.SkillRepository, binding BindingRepository) *Exporter {
	return &Exporter{svc: svc, skillRepo: skillRepo, binding: binding}
}

// Export serialises an agent and all its bound skills to an AgentBundle.
func (e *Exporter) Export(ctx context.Context, agentID uuid.UUID) (AgentBundle, error) {
	a, err := e.svc.Get(ctx, agentID)
	if err != nil {
		return AgentBundle{}, fmt.Errorf("bundle: export agent: %w", err)
	}

	skillIDs, err := e.binding.ListSkillIDs(ctx, agentID)
	if err != nil {
		return AgentBundle{}, fmt.Errorf("bundle: list skill ids: %w", err)
	}

	var bundleSkills []BundleSkill
	if len(skillIDs) > 0 {
		skills, err := e.skillRepo.ListByIDs(ctx, skillIDs)
		if err != nil {
			return AgentBundle{}, fmt.Errorf("bundle: load skills: %w", err)
		}
		for _, s := range skills {
			bundleSkills = append(bundleSkills, skillToBundleSkill(s))
		}
	}

	return AgentBundle{
		FormatVersion: bundleFormatVersion,
		ExportedAt:    time.Now().UTC(),
		Agent: BundleAgentDef{
			Name:             a.Name,
			Slug:             a.Slug,
			Description:      a.Description,
			SystemPrompt:     a.SystemPrompt,
			ModelConfig:      a.ModelConfig,
			PermissionRules:  a.PermissionRules,
			Config:           a.Config,
			EnableManagement: a.EnableManagement,
		},
		Skills: bundleSkills,
	}, nil
}

// --- Importer ---

// SkillCreator is the narrow interface for creating a skill.
// Implemented by *skill.Service.
type SkillCreator interface {
	Create(ctx context.Context, req skill.CreateRequest) (skill.Response, error)
}

// Importer provisions an AgentBundle into the current tenant.
type Importer struct {
	agentSvc    Service
	skillSvc    SkillCreator
	bindingRepo BindingRepository
}

// NewImporter creates an Importer.
func NewImporter(agentSvc Service, skillSvc SkillCreator, bindingRepo BindingRepository) *Importer {
	return &Importer{agentSvc: agentSvc, skillSvc: skillSvc, bindingRepo: bindingRepo}
}

// Import provisions all entities from the bundle into the current tenant context.
// The agent is always created as DRAFT. Skills are created one by one; failures
// are logged but do not abort the whole import.
func (imp *Importer) Import(ctx context.Context, bundle AgentBundle) (ImportResult, error) {
	// 1. Create skills and collect their new tenant-local IDs.
	var newSkillIDs []uuid.UUID
	imported := 0
	for _, bs := range bundle.Skills {
		req := skill.CreateRequest{
			Name:                   bs.Name,
			Slug:                   bs.Slug,
			Description:            bs.Description,
			Instructions:           bs.Instructions,
			Category:               bs.Category,
			AllowedTools:           bs.AllowedTools,
			DisableModelInvocation: bs.DisableModelInvocation,
			ContextMode:            bs.ContextMode,
			WhenToUse:              bs.WhenToUse,
			ArgumentHint:           bs.ArgumentHint,
		}
		created, err := imp.skillSvc.Create(ctx, req)
		if err != nil {
			// Non-fatal: continue with remaining skills.
			continue
		}
		newSkillIDs = append(newSkillIDs, created.ID)
		imported++
	}

	// 2. Create the agent.
	a := bundle.Agent
	agentReq := CreateAgentRequest{
		Name:             a.Name,
		Slug:             a.Slug,
		Description:      a.Description,
		SystemPrompt:     a.SystemPrompt,
		ModelConfig:      a.ModelConfig,
		PermissionRules:  a.PermissionRules,
		Config:           a.Config,
		EnableManagement: a.EnableManagement,
	}
	created, err := imp.agentSvc.Create(ctx, agentReq)
	if err != nil {
		return ImportResult{}, fmt.Errorf("bundle: import agent: %w", err)
	}

	// 3. Bind skills to the agent.
	if len(newSkillIDs) > 0 {
		if err := imp.bindingRepo.SyncSkills(ctx, created.ID, newSkillIDs); err != nil {
			// Non-fatal: agent is usable even without bindings.
			_ = err
		}
	}

	return ImportResult{
		AgentID:        created.ID,
		AgentName:      created.Name,
		SkillsImported: imported,
	}, nil
}

// --- helpers ---

func skillToBundleSkill(s skill.Skill) BundleSkill {
	return BundleSkill{
		OriginalID:             s.ID,
		Name:                   s.Name,
		Slug:                   s.Slug,
		Description:            s.Description,
		Instructions:           s.Instructions,
		Category:               s.Category,
		AllowedTools:           s.AllowedTools,
		DisableModelInvocation: s.DisableModelInvocation,
		ContextMode:            s.ContextMode,
		WhenToUse:              s.WhenToUse,
		ArgumentHint:           s.ArgumentHint,
	}
}
