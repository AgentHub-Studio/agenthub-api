package skill

import (
	"time"

	"github.com/google/uuid"
)

// CreateRequest is the payload for creating a skill.
type CreateRequest struct {
	Name                   string   `json:"name"`
	Slug                   string   `json:"slug"`
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
