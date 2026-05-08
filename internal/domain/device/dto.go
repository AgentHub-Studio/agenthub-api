package device

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DeviceResponse is the API representation of a Device.
type DeviceResponse struct {
	ID                uuid.UUID       `json:"id"`
	Name              string          `json:"name"`
	Type              string          `json:"type"`
	Description       string          `json:"description"`
	MCPServerConfigID *uuid.UUID      `json:"mcpServerConfigId,omitempty"`
	ResourceURI       string          `json:"resourceUri"`
	Capabilities      []string        `json:"capabilities"`
	LastSeenAt        *time.Time      `json:"lastSeenAt,omitempty"`
	Status            string          `json:"status"`
	Metadata          json.RawMessage `json:"metadata"`
	Enabled           bool            `json:"enabled"`
	CreatedAt         time.Time       `json:"createdAt"`
	UpdatedAt         time.Time       `json:"updatedAt"`
}

func responseFrom(d Device) DeviceResponse {
	caps := d.Capabilities
	if caps == nil {
		caps = []string{}
	}
	meta := d.Metadata
	if len(meta) == 0 {
		meta = json.RawMessage("{}")
	}
	return DeviceResponse{
		ID:                d.ID,
		Name:              d.Name,
		Type:              string(d.Type),
		Description:       d.Description,
		MCPServerConfigID: d.MCPServerConfigID,
		ResourceURI:       d.ResourceURI,
		Capabilities:      caps,
		LastSeenAt:        d.LastSeenAt,
		Status:            string(d.Status),
		Metadata:          meta,
		Enabled:           d.Enabled,
		CreatedAt:         d.CreatedAt,
		UpdatedAt:         d.UpdatedAt,
	}
}

// CreateDeviceRequest is the body for POST /api/devices.
type CreateDeviceRequest struct {
	Name              string          `json:"name"`
	Type              string          `json:"type"`
	Description       string          `json:"description"`
	MCPServerConfigID *uuid.UUID      `json:"mcpServerConfigId,omitempty"`
	ResourceURI       string          `json:"resourceUri"`
	Capabilities      []string        `json:"capabilities"`
	Metadata          json.RawMessage `json:"metadata"`
}

func (r CreateDeviceRequest) validate() error {
	if r.Name == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	if r.Type == "" {
		return fmt.Errorf("%w: type is required", ErrValidation)
	}
	return nil
}

func (r CreateDeviceRequest) toDevice() Device {
	dt := DeviceType(r.Type)
	if dt == "" {
		dt = DeviceTypeSensor
	}
	return Device{
		Name:              r.Name,
		Type:              dt,
		Description:       r.Description,
		MCPServerConfigID: r.MCPServerConfigID,
		ResourceURI:       r.ResourceURI,
		Capabilities:      r.Capabilities,
		Metadata:          r.Metadata,
		Status:            DeviceStatusUnknown,
		Enabled:           true,
	}
}

// UpdateDeviceRequest is the body for PUT /api/devices/:id.
type UpdateDeviceRequest struct {
	Name         *string         `json:"name,omitempty"`
	Description  *string         `json:"description,omitempty"`
	ResourceURI  *string         `json:"resourceUri,omitempty"`
	Capabilities []string        `json:"capabilities,omitempty"`
	Status       *string         `json:"status,omitempty"`
	Metadata     json.RawMessage `json:"metadata,omitempty"`
	Enabled      *bool           `json:"enabled,omitempty"`
}

// HeartbeatRequest is sent by the MCP client runtime to report a device is alive.
type HeartbeatRequest struct {
	Status   string          `json:"status"` // ONLINE, OFFLINE
	Metadata json.RawMessage `json:"metadata,omitempty"`
}
