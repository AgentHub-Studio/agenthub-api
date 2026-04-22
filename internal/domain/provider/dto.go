package provider

import "encoding/json"

// Response is the JSON shape returned to the admin UI.
type Response struct {
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Icon        string          `json:"icon,omitempty"`
	Category    string          `json:"category,omitempty"`
	Kind        Kind            `json:"kind"`
	Template    json.RawMessage `json:"template"`
	IsBuiltin   bool            `json:"isBuiltin"`
	Enabled     bool            `json:"enabled"`
}

// ResponseFrom converts a domain Provider into a Response.
func ResponseFrom(p Provider) Response {
	return Response{
		Slug:        p.Slug,
		Name:        p.Name,
		Description: p.Description,
		Icon:        p.Icon,
		Category:    p.Category,
		Kind:        p.Kind,
		Template:    p.TemplateJSON,
		IsBuiltin:   p.IsBuiltin,
		Enabled:     p.Enabled,
	}
}
