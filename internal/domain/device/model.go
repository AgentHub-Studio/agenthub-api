// Package device provides the Device Node Network registry.
// Devices are MCP-discoverable nodes (sensors, actuators, gateways) discovered
// by the MCP Client Runtime and registered here for agent subscription.
package device

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a device cannot be found.
var ErrNotFound = errors.New("device: not found")

// ErrNameConflict is returned when a device name is already in use.
var ErrNameConflict = errors.New("device: name already in use")

// ErrValidation is returned when a device request fails business validation
// (name vazio, type inválido, etc).
var ErrValidation = errors.New("device: validation failed")

// DeviceType classifies the role of a device in the network.
type DeviceType string

const (
	DeviceTypeSensor   DeviceType = "SENSOR"
	DeviceTypeActuator DeviceType = "ACTUATOR"
	DeviceTypeGateway  DeviceType = "GATEWAY"
	DeviceTypeCompute  DeviceType = "COMPUTE"
)

// DeviceStatus represents the last known connectivity state.
type DeviceStatus string

const (
	DeviceStatusOnline  DeviceStatus = "ONLINE"
	DeviceStatusOffline DeviceStatus = "OFFLINE"
	DeviceStatusUnknown DeviceStatus = "UNKNOWN"
)

// Device is an MCP-discoverable node registered in the Device Node Network.
// Stored in ah_{tenantID}.device — no tenant_id column.
type Device struct {
	ID                 uuid.UUID
	Name               string
	Type               DeviceType
	Description        string
	// MCPServerConfigID references the MCP server that exposes this device.
	// Nil when the device is manually registered (not auto-discovered).
	MCPServerConfigID  *uuid.UUID
	// ResourceURI is the MCP Resource URI to read live data from this device.
	ResourceURI        string
	// Capabilities is a list of tags describing what the device can do.
	Capabilities       []string
	LastSeenAt         *time.Time
	Status             DeviceStatus
	// Metadata holds device-specific key/value pairs (firmware, location, etc.)
	Metadata           json.RawMessage
	Enabled            bool
	CreatedAt          time.Time
	UpdatedAt          time.Time
}
