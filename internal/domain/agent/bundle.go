package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/skill"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
)

const bundleFormatVersion = "1"

// BundleTool is the portable representation of a tool inside a bundle. Bug 294:
// previously skills exported with allowedTools (slug list) but the actual tool
// definitions (HTTP/SQL/RAG configs) were missing, leaving imported agents
// non-functional. The bundle now ships the full tool spec so a fresh tenant
// can re-hydrate the wired skill+tool graph without manual rebuilding.
type BundleTool struct {
	OriginalID  uuid.UUID       `json:"originalId"`
	Name        string          `json:"name"`
	Slug        string          `json:"slug"`
	Type        string          `json:"type"`
	Config      json.RawMessage `json:"config,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema,omitempty"`
	Description string          `json:"description,omitempty"`
	Labels      []string        `json:"labels,omitempty"`
	ReadOnly    bool            `json:"readOnly,omitempty"`
	// SkillSlugs lists the bundle skills this tool is bound to. The importer
	// uses these slugs to recreate the skill_tool bindings after both skills
	// and tools have been provisioned in the destination tenant.
	SkillSlugs []string `json:"skillSlugs,omitempty"`
}

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
	FormatVersion string         `json:"formatVersion"` // always "1"
	ExportedAt    time.Time      `json:"exportedAt"`
	Agent         BundleAgentDef `json:"agent"`
	Skills        []BundleSkill  `json:"skills"`
	// Tools holds full definitions of every tool bound to any of the bundled
	// skills, so the importer can recreate them and re-bind. Bug 294.
	Tools []BundleTool `json:"tools,omitempty"`
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
	// toolRepo is optional. When provided (bug 294), the export includes full
	// tool definitions and their skill bindings — enabling true re-hydration
	// of the agent into a fresh tenant. When nil, falls back to skills-only.
	toolRepo tool.ToolRepository
}

// NewExporter creates an Exporter.
func NewExporter(svc Service, skillRepo skill.SkillRepository, binding BindingRepository) *Exporter {
	return &Exporter{svc: svc, skillRepo: skillRepo, binding: binding}
}

// WithToolRepo wires a tool repository so the exporter can include full tool
// definitions in the bundle. Optional — without it, bundles ship skills only.
func (e *Exporter) WithToolRepo(toolRepo tool.ToolRepository) *Exporter {
	e.toolRepo = toolRepo
	return e
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
	skillIDToSlug := map[uuid.UUID]string{}
	if len(skillIDs) > 0 {
		skills, err := e.skillRepo.ListByIDs(ctx, skillIDs)
		if err != nil {
			return AgentBundle{}, fmt.Errorf("bundle: load skills: %w", err)
		}
		for _, s := range skills {
			bundleSkills = append(bundleSkills, skillToBundleSkill(s))
			skillIDToSlug[s.ID] = s.Slug
		}
	}

	// Bug 294: gather tools bound to any of the bundle skills, dedup by ID.
	var bundleTools []BundleTool
	if e.toolRepo != nil && len(skillIDs) > 0 {
		seen := map[uuid.UUID]int{} // toolID → index in bundleTools
		for _, sid := range skillIDs {
			_, tools, err := e.toolRepo.ListBySkill(ctx, sid)
			if err != nil {
				return AgentBundle{}, fmt.Errorf("bundle: list tools for skill %s: %w", sid, err)
			}
			skillSlug := skillIDToSlug[sid]
			for _, t := range tools {
				if idx, ok := seen[t.ID]; ok {
					bundleTools[idx].SkillSlugs = append(bundleTools[idx].SkillSlugs, skillSlug)
					continue
				}
				seen[t.ID] = len(bundleTools)
				bundleTools = append(bundleTools, BundleTool{
					OriginalID:  t.ID,
					Name:        t.Name,
					Slug:        t.Slug,
					Type:        t.Type,
					Config:      t.Config,
					InputSchema: t.InputSchema,
					Description: t.Description,
					Labels:      t.Labels,
					ReadOnly:    t.ReadOnly,
					SkillSlugs:  []string{skillSlug},
				})
			}
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
		Tools:  bundleTools,
	}, nil
}

// --- Importer ---

// SkillCreator is the narrow interface for creating a skill.
// Implemented by *skill.Service.
type SkillCreator interface {
	Create(ctx context.Context, req skill.CreateRequest) (skill.Response, error)
}

// ToolCreator is the narrow interface for creating + binding a tool.
// Implemented by *tool.Service. Bug 294.
type ToolCreator interface {
	Create(ctx context.Context, req tool.CreateRequest) (tool.Response, error)
	BindToSkill(ctx context.Context, skillID uuid.UUID, req tool.BindRequest) (tool.SkillToolResponse, error)
}

// Importer provisions an AgentBundle into the current tenant.
type Importer struct {
	agentSvc    Service
	skillSvc    SkillCreator
	bindingRepo BindingRepository
	// toolSvc is optional. When provided (bug 294), the importer recreates
	// tools from bundle.Tools and re-binds them to skills via SkillSlugs.
	toolSvc ToolCreator
}

// NewImporter creates an Importer.
func NewImporter(agentSvc Service, skillSvc SkillCreator, bindingRepo BindingRepository) *Importer {
	return &Importer{agentSvc: agentSvc, skillSvc: skillSvc, bindingRepo: bindingRepo}
}

// WithToolSvc wires the tool service so the importer can recreate tools and
// rebind them to skills. Optional — without it, bundle.Tools is ignored.
func (imp *Importer) WithToolSvc(toolSvc ToolCreator) *Importer {
	imp.toolSvc = toolSvc
	return imp
}

// Import provisions all entities from the bundle into the current tenant context.
// The agent is always created as DRAFT. Skills are created one by one; failures
// are logged but do not abort the whole import.
func (imp *Importer) Import(ctx context.Context, bundle AgentBundle) (ImportResult, error) {
	// 1. Create skills and collect their new tenant-local IDs. Track slug→newID
	// so step 3 (bug 294) can rebind tools to the correct skills.
	var newSkillIDs []uuid.UUID
	skillSlugToID := map[string]uuid.UUID{}
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
		skillSlugToID[bs.Slug] = created.ID
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

	// 4. Bug 294: recreate tools and rebind them to the new skills via slug map.
	// Each tool may be bound to multiple skills (SkillSlugs). Failures are
	// non-fatal — partial recovery is preferable to aborting the whole import.
	if imp.toolSvc != nil && len(bundle.Tools) > 0 {
		for _, bt := range bundle.Tools {
			toolReq := tool.CreateRequest{
				Name:        bt.Name,
				Slug:        bt.Slug,
				Type:        bt.Type,
				Config:      bt.Config,
				InputSchema: bt.InputSchema,
				Description: bt.Description,
				Labels:      bt.Labels,
				ReadOnly:    bt.ReadOnly,
			}
			createdTool, err := imp.toolSvc.Create(ctx, toolReq)
			if err != nil {
				// Tool slug might already exist in destination tenant — non-fatal.
				continue
			}
			for _, sslug := range bt.SkillSlugs {
				sid, ok := skillSlugToID[sslug]
				if !ok {
					continue // skill failed to import; skip this binding.
				}
				_, _ = imp.toolSvc.BindToSkill(ctx, sid, tool.BindRequest{ToolID: createdTool.ID})
			}
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
