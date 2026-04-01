// Package search provides cross-domain full-text search for tenant resources.
package search

// SearchResult is a generic resource match returned by global search.
type SearchResult struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Status      string `json:"status,omitempty"`
	Slug        string `json:"slug,omitempty"`
}

// GlobalSearchResponse aggregates results from all searchable resource types.
type GlobalSearchResponse struct {
	Query          string         `json:"query"`
	Agents         []SearchResult `json:"agents"`
	Skills         []SearchResult `json:"skills"`
	Tools          []SearchResult `json:"tools"`
	KnowledgeBases []SearchResult `json:"knowledgeBases"`
}
