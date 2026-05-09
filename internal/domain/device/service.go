package device

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
)

// Service manages the Device Node Network.
type Service interface {
	List(ctx context.Context, req pagination.PageRequest) (pagination.Page[DeviceResponse], error)
	GetByID(ctx context.Context, id uuid.UUID) (DeviceResponse, error)
	Create(ctx context.Context, req CreateDeviceRequest) (DeviceResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateDeviceRequest) (DeviceResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// Heartbeat is called by the MCP client runtime to report a device is alive.
	Heartbeat(ctx context.Context, id uuid.UUID, req HeartbeatRequest) error

	// ListByAgent returns all devices bound to a given agent.
	ListByAgent(ctx context.Context, agentID uuid.UUID) ([]DeviceResponse, error)
	// AttachToAgent binds a device to an agent (idempotent).
	AttachToAgent(ctx context.Context, agentID, deviceID uuid.UUID) error
	// DetachFromAgent removes the device binding from an agent.
	DetachFromAgent(ctx context.Context, agentID, deviceID uuid.UUID) error
}

type service struct {
	repo Repository
}

// NewService creates a Service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context, req pagination.PageRequest) (pagination.Page[DeviceResponse], error) {
	page, err := s.repo.List(ctx, req)
	if err != nil {
		return pagination.Page[DeviceResponse]{}, err
	}
	return pagination.MapPage(page, responseFrom), nil
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (DeviceResponse, error) {
	d, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return DeviceResponse{}, err
	}
	return responseFrom(d), nil
}

func (s *service) Create(ctx context.Context, req CreateDeviceRequest) (DeviceResponse, error) {
	if err := req.validate(); err != nil {
		return DeviceResponse{}, err
	}
	exists, err := s.repo.ExistsByName(ctx, req.Name)
	if err != nil {
		return DeviceResponse{}, fmt.Errorf("device: check duplicate name: %w", err)
	}
	if exists {
		return DeviceResponse{}, ErrNameConflict
	}
	// Bug 180: strip HTML do description (XSS prevention cross-cutting).
	req.Description = sanitize.StripHTML(req.Description)
	d := req.toDevice()
	created, err := s.repo.Create(ctx, d)
	if err != nil {
		return DeviceResponse{}, err
	}
	return responseFrom(created), nil
}

func (s *service) Update(ctx context.Context, id uuid.UUID, req UpdateDeviceRequest) (DeviceResponse, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return DeviceResponse{}, err
	}

	if req.Name != nil {
		// Bug 113: device.name não pode ser vazio. Sem este gate, admin
		// pode limpar o name via PATCH (Create rejeita name vazio).
		if *req.Name == "" {
			return DeviceResponse{}, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		// Bug 137: name varchar(255) — gate length em Update.
		if len(*req.Name) > 255 {
			return DeviceResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(*req.Name))
		}
		existing.Name = *req.Name
	}
	if req.Description != nil {
		// Bug 175: cap em Update (cross-cutting com Create — bug 160).
		if len(*req.Description) > 32000 {
			return DeviceResponse{}, fmt.Errorf("%w: description exceeds maximum length of 32000 chars (got %d)", ErrValidation, len(*req.Description))
		}
		// Bug 180: strip HTML (XSS prevention).
		existing.Description = sanitize.StripHTML(*req.Description)
	}
	if req.ResourceURI != nil {
		existing.ResourceURI = *req.ResourceURI
	}
	if req.Capabilities != nil {
		existing.Capabilities = req.Capabilities
	}
	if req.Status != nil {
		// Bug 127: status PATCH aceitava qualquer string. Heartbeat
		// já gateava para ONLINE/OFFLINE; PATCH /devices/:id ignorava.
		// Update permite UNKNOWN também (estado inicial pré-heartbeat).
		switch DeviceStatus(*req.Status) {
		case DeviceStatusOnline, DeviceStatusOffline, DeviceStatusUnknown:
		default:
			return DeviceResponse{}, fmt.Errorf("%w: status must be one of ONLINE|OFFLINE|UNKNOWN (got %q)", ErrValidation, *req.Status)
		}
		existing.Status = DeviceStatus(*req.Status)
	}
	if len(req.Metadata) > 2 {
		existing.Metadata = req.Metadata
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return DeviceResponse{}, err
	}
	return responseFrom(updated), nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) Heartbeat(ctx context.Context, id uuid.UUID, req HeartbeatRequest) error {
	status := DeviceStatus(req.Status)
	if status != DeviceStatusOnline && status != DeviceStatusOffline {
		return fmt.Errorf("device: invalid status %q, must be ONLINE or OFFLINE", req.Status)
	}
	return s.repo.Heartbeat(ctx, id, status, time.Now())
}

func (s *service) ListByAgent(ctx context.Context, agentID uuid.UUID) ([]DeviceResponse, error) {
	devices, err := s.repo.ListByAgent(ctx, agentID)
	if err != nil {
		return nil, err
	}
	out := make([]DeviceResponse, len(devices))
	for i, d := range devices {
		out[i] = responseFrom(d)
	}
	return out, nil
}

func (s *service) AttachToAgent(ctx context.Context, agentID, deviceID uuid.UUID) error {
	// Verify device exists before binding.
	if _, err := s.repo.GetByID(ctx, deviceID); err != nil {
		return err
	}
	return s.repo.AttachToAgent(ctx, agentID, deviceID)
}

func (s *service) DetachFromAgent(ctx context.Context, agentID, deviceID uuid.UUID) error {
	return s.repo.DetachFromAgent(ctx, agentID, deviceID)
}
