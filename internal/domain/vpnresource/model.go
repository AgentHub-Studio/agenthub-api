// Package vpnresource manages VPN tunnel configurations per tenant.
package vpnresource

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a VpnResource is not found.
var ErrNotFound = errors.New("vpn resource not found")

// ErrValidation é retornado quando CreateRequest falha validação
// server-side (name vazio, etc). Antes do fix, qualquer erro do
// service caía em 500 no handler genérico — incluindo body {}
// que criava VpnResource com name="" persistido no banco.
var ErrValidation = errors.New("vpn resource validation failed")

// VpnResource represents an OpenVPN tunnel configuration.
type VpnResource struct {
	ID             uuid.UUID `db:"id"`
	Name           string    `db:"name"`
	Description    string    `db:"description"`
	Enabled        bool      `db:"enabled"`
	OvpnConfigPath string    `db:"ovpn_config_path"`
	AuthFilePath   string    `db:"auth_file_path"`
	SecretName     string    `db:"secret_name"`
	CreatedAt      time.Time `db:"created_at"`
	UpdatedAt      time.Time `db:"updated_at"`
}

// VpnResourceResponse is the public DTO for VpnResource.
type VpnResourceResponse struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Enabled        bool      `json:"enabled"`
	OvpnConfigPath string    `json:"ovpnConfigPath"`
	AuthFilePath   string    `json:"authFilePath"`
	SecretName     string    `json:"secretName"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// ResponseFrom converts a VpnResource to its public DTO.
func ResponseFrom(v VpnResource) VpnResourceResponse { return VpnResourceResponse(v) }

// CreateRequest is the payload for creating or updating a VpnResource.
type CreateRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description"`
	Enabled        bool   `json:"enabled"`
	OvpnConfigPath string `json:"ovpnConfigPath"`
	AuthFilePath   string `json:"authFilePath"`
	SecretName     string `json:"secretName"`
}

// TestConnectionResponse is the result of a VPN connectivity test.
type TestConnectionResponse struct {
	Connected bool   `json:"connected"`
	Message   string `json:"message"`
}
