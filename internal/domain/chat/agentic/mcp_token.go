package agentic

import "github.com/AgentHub-Studio/agenthub-api/internal/workloadidentity"

type MCPRuntimeTokenProvider = workloadidentity.TokenProvider

// KeycloakServiceTokenProvider is kept as an alias for the agentic package.
type KeycloakServiceTokenProvider = workloadidentity.KeycloakTokenProvider

func NewKeycloakServiceTokenProvider(keycloakBaseURL, clientID string, credentials workloadidentity.Resolver, scopes []string) (*KeycloakServiceTokenProvider, error) {
	return workloadidentity.NewKeycloakTokenProvider(keycloakBaseURL, clientID, credentials, scopes)
}
