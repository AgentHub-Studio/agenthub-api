// Package suggest generates friendly PT-BR starter prompts for the chat
// welcome screen, derived from the tenant's active integrations. Goal:
// a layperson opening the chat for the first time sees "Buscar últimos
// pedidos no HubSpot" instead of a blank prompt field.
package suggest

import (
	"context"
	"fmt"
	"strings"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/integration"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// defaultLimit is the number of suggestions returned when the caller
// does not specify otherwise.
const defaultLimit = 3

// Suggestion is one starter prompt the UI can render as a clickable chip.
type Suggestion struct {
	Prompt          string `json:"prompt"`
	IntegrationName string `json:"integrationName"`
	IntegrationType string `json:"integrationType,omitempty"`
}

// integrationLister is the narrow dependency the suggestion service needs
// from integration.Service. Declared locally so tests can fake it without
// pulling the full service graph.
type integrationLister interface {
	List(ctx context.Context, req pagination.PageRequest, filters integration.ListFilters) (pagination.Page[integration.Response], error)
}

// Service generates suggestions from the tenant's integration catalog.
type Service struct {
	integrations integrationLister
}

// NewService wires the suggest Service to an integration lister.
func NewService(integrations integrationLister) *Service {
	return &Service{integrations: integrations}
}

// Generate returns up to `limit` suggestions (min 1, capped at 6) for the
// tenant inferred from the request context. Returns an empty slice — never
// nil — when the tenant has no integrations yet.
func (s *Service) Generate(ctx context.Context, limit int) ([]Suggestion, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > 6 {
		limit = 6
	}

	enabled := true
	req := pagination.PageRequest{Page: 0, Size: 20}
	page, err := s.integrations.List(ctx, req, integration.ListFilters{Enabled: &enabled})
	if err != nil {
		return nil, fmt.Errorf("suggest: list integrations: %w", err)
	}

	out := make([]Suggestion, 0, limit)
	for _, it := range page.Content {
		if len(out) >= limit {
			break
		}
		if !it.Enabled {
			continue
		}
		prompt := promptFor(it)
		if prompt == "" {
			continue
		}
		out = append(out, Suggestion{
			Prompt:          prompt,
			IntegrationName: it.Name,
			IntegrationType: string(it.Type),
		})
	}
	return out, nil
}

// promptFor renders a PT-BR starter prompt tailored to the integration's
// type. Returns empty when the integration is too generic to produce a
// useful suggestion.
func promptFor(it integration.Response) string {
	name := strings.TrimSpace(it.Name)
	if name == "" {
		return ""
	}
	switch it.Type {
	case integration.IntegrationTypeHTTPAPI:
		return fmt.Sprintf("O que você consegue fazer com %s?", name)
	case integration.IntegrationTypeDatabaseQuery:
		return fmt.Sprintf("Mostre os últimos registros do banco %s.", name)
	case integration.IntegrationTypeMCP:
		return fmt.Sprintf("Quais recursos estão disponíveis em %s?", name)
	}
	return fmt.Sprintf("O que você consegue fazer com %s?", name)
}
