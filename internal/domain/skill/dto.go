package skill

import (
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the payload for creating a skill.
// ToolID is optional — when provided the newly-created tool is automatically
// bound to the skill, mirroring the skillId auto-bind on POST /api/tools.
// BUG-F1 fix: field was previously absent, causing clients that sent
// {"toolId": "..."} to have it silently ignored by the JSON decoder.
type CreateRequest struct {
	Name                   string            `json:"name"`
	Slug                   string            `json:"slug"`
	Description            string            `json:"description"`
	Instructions           string            `json:"instructions"`
	Category               string            `json:"category"`
	AllowedTools           []string          `json:"allowedTools"`
	DisableModelInvocation bool              `json:"disableModelInvocation"`
	ContextMode            string            `json:"contextMode"`
	WhenToUse              *string           `json:"whenToUse"`
	ArgumentHint           *string           `json:"argumentHint"`
	ShouldDefer            bool              `json:"shouldDefer"`
	// ModelOverrides allows per-skill model selection (EXT-006a, PDF Section 6.1).
	ModelOverrides   map[string]string `json:"modelOverrides,omitempty"`
	// EffortLevel sets the Anthropic thinking-budget tier (EXT-006a, PDF Section 6.1).
	EffortLevel      string            `json:"effortLevel,omitempty"`
	// AssociatedAgents lists agent slugs whose persona is activated by this skill.
	AssociatedAgents []string          `json:"associatedAgents,omitempty"`
	// DynamicHooks lists hook-event slugs registered for this skill's invocation.
	DynamicHooks     []string          `json:"dynamicHooks,omitempty"`
	ToolID           *uuid.UUID        `json:"toolId,omitempty"`  // optional: auto-bind single tool after creation
	ToolIDs          []uuid.UUID       `json:"toolIds,omitempty"` // optional: auto-bind multiple tools after creation (consistent with skillIds on agents)
}

// UpdateRequest is the payload for updating a skill.
type UpdateRequest struct {
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	Instructions           string   `json:"instructions"`
	Category               string   `json:"category"`
	AllowedTools           []string `json:"allowedTools"`
	DisableModelInvocation bool     `json:"disableModelInvocation"`
	ContextMode            string   `json:"contextMode"`
	WhenToUse              *string  `json:"whenToUse"`
	ArgumentHint           *string  `json:"argumentHint"`
	ShouldDefer            bool     `json:"shouldDefer"`
}

// Response is the JSON representation of a Skill.
type Response struct {
	ID                     uuid.UUID `json:"id"`
	Name                   string    `json:"name"`
	Slug                   string    `json:"slug"`
	Description            string    `json:"description"`
	Instructions           string    `json:"instructions"`
	Category               string    `json:"category"`
	AllowedTools           []string  `json:"allowedTools"`
	DisableModelInvocation bool      `json:"disableModelInvocation"`
	ContextMode            string    `json:"contextMode"`
	WhenToUse              *string   `json:"whenToUse"`
	ArgumentHint           *string   `json:"argumentHint"`
	ShouldDefer            bool      `json:"shouldDefer"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

// ResponseFrom converts a Skill to a Response.
func ResponseFrom(s Skill) Response {
	return Response{
		ID:                     s.ID,
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
		ShouldDefer:            s.ShouldDefer,
		CreatedAt:              s.CreatedAt,
		UpdatedAt:              s.UpdatedAt,
	}
}
