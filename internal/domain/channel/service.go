package channel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// AgentDispatcher dispatches an inbound message to an agent's agentic loop
// and returns the agent's reply text. Implemented by the server wiring layer.
type AgentDispatcher interface {
	Dispatch(ctx context.Context, agentID uuid.UUID, userText, tenantID string) (string, error)
}

// Service defines business logic for Channel management.
type Service interface {
	List(ctx context.Context, req pagination.PageRequest) (pagination.Page[ChannelResponse], error)
	GetByID(ctx context.Context, id uuid.UUID) (ChannelResponse, error)
	Create(ctx context.Context, req CreateChannelRequest) (ChannelTokenResponse, error)
	Update(ctx context.Context, id uuid.UUID, req UpdateChannelRequest) (ChannelResponse, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// HandleInbound receives a raw platform request, verifies it, parses the message,
	// dispatches it to the bound agent, and sends the reply back through the adapter.
	// Returns InboundMessage so the HTTP handler can echo URL-verification challenges.
	HandleInbound(ctx context.Context, token string, headers map[string]string, body []byte) (InboundMessage, error)
}

type service struct {
	repo       Repository
	registry   *Registry
	dispatcher AgentDispatcher
}

// NewService creates a new channel Service.
func NewService(repo Repository, registry *Registry) *service {
	return &service{repo: repo, registry: registry}
}

// WithDispatcher wires an agent dispatcher used by HandleInbound.
func (s *service) WithDispatcher(d AgentDispatcher) *service {
	s.dispatcher = d
	return s
}

func (s *service) List(ctx context.Context, req pagination.PageRequest) (pagination.Page[ChannelResponse], error) {
	channels, err := s.repo.List(ctx)
	if err != nil {
		return pagination.Page[ChannelResponse]{}, fmt.Errorf("channel service: list: %w", err)
	}
	responses := make([]ChannelResponse, len(channels))
	for i, ch := range channels {
		responses[i] = ResponseFrom(ch)
	}
	return pagination.NewPage(responses, int64(len(responses)), req), nil
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (ChannelResponse, error) {
	ch, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ChannelResponse{}, err
	}
	return ResponseFrom(ch), nil
}

func (s *service) Create(ctx context.Context, req CreateChannelRequest) (ChannelTokenResponse, error) {
	if req.Name == "" {
		return ChannelTokenResponse{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	// Bug 130: name varchar(255) — gate length antes do INSERT.
	if len(req.Name) > 255 {
		return ChannelTokenResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	}
	if req.Type == "" {
		return ChannelTokenResponse{}, fmt.Errorf("%w: type is required", ErrValidation)
	}
	switch req.Type {
	case ChannelTypeWebhook, ChannelTypeSlack, ChannelTypeTelegram, ChannelTypeDiscord, ChannelTypeCustom:
		// ok
	default:
		return ChannelTokenResponse{}, fmt.Errorf("%w: type must be one of WEBHOOK|SLACK|TELEGRAM|DISCORD|CUSTOM (got %q)", ErrValidation, req.Type)
	}
	exists, err := s.repo.ExistsByName(ctx, req.Name)
	if err != nil {
		return ChannelTokenResponse{}, fmt.Errorf("channel: check duplicate name: %w", err)
	}
	if exists {
		return ChannelTokenResponse{}, ErrSlugConflict
	}

	token, err := generateToken()
	if err != nil {
		return ChannelTokenResponse{}, fmt.Errorf("channel: generate token: %w", err)
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	ch := Channel{
		Name:    req.Name,
		Type:    req.Type,
		AgentID: req.AgentID,
		Config:  req.Config,
		Token:   token,
		Enabled: enabled,
	}

	created, err := s.repo.Create(ctx, ch)
	if err != nil {
		return ChannelTokenResponse{}, err
	}

	return ChannelTokenResponse{
		ChannelResponse: ResponseFrom(created),
		Token:           created.Token,
	}, nil
}

func (s *service) Update(ctx context.Context, id uuid.UUID, req UpdateChannelRequest) (ChannelResponse, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return ChannelResponse{}, err
	}

	if req.Name != nil {
		// Bug 112: empty name nunca foi válido — Create rejeita via L62
		// (bug 12). Sem este gate, admin podia limpar o name via PATCH e
		// ficar com "channel sem nome" na UI.
		if *req.Name == "" {
			return ChannelResponse{}, fmt.Errorf("%w: name cannot be empty", ErrValidation)
		}
		// Bug 137: name varchar(255) — gate length em Update.
		if len(*req.Name) > 255 {
			return ChannelResponse{}, fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(*req.Name))
		}
		existing.Name = *req.Name
	}
	if req.AgentID != nil {
		existing.AgentID = *req.AgentID
	}
	if len(req.Config) > 0 {
		existing.Config = req.Config
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}

	updated, err := s.repo.Update(ctx, existing)
	if err != nil {
		return ChannelResponse{}, err
	}
	return ResponseFrom(updated), nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}

func (s *service) HandleInbound(ctx context.Context, token string, headers map[string]string, body []byte) (InboundMessage, error) {
	ch, err := s.repo.GetByToken(ctx, token)
	if err != nil {
		return InboundMessage{}, err
	}

	if !ch.Enabled {
		return InboundMessage{}, fmt.Errorf("channel: disabled")
	}

	adapter, ok := s.registry.Lookup(ch.Type)
	if !ok {
		return InboundMessage{}, fmt.Errorf("channel: no adapter for type %s", ch.Type)
	}

	if err := adapter.VerifyRequest(ctx, ch, headers, body); err != nil {
		return InboundMessage{}, fmt.Errorf("channel: verify: %w", err)
	}

	msg, err := adapter.ParseMessage(ctx, ch, body)
	if err != nil {
		return InboundMessage{}, fmt.Errorf("channel: parse: %w", err)
	}

	// URL-verification challenge or empty/bot message — return without dispatching.
	if msg.Challenge != "" || msg.Text == "" {
		return msg, nil
	}

	if s.dispatcher == nil {
		return msg, fmt.Errorf("channel: dispatcher not configured")
	}

	tenantID := tenantctx.FromContext(ctx)
	replyText, err := s.dispatcher.Dispatch(ctx, ch.AgentID, msg.Text, tenantID)
	if err != nil {
		return msg, fmt.Errorf("channel: dispatch: %w", err)
	}

	if replyText != "" {
		if sendErr := adapter.SendReply(ctx, ch, OutboundMessage{
			ReplyTo: msg.ReplyTo,
			Text:    replyText,
		}); sendErr != nil {
			return msg, fmt.Errorf("channel: send reply: %w", sendErr)
		}
	}

	return msg, nil
}

// generateToken returns a 32-byte hex-encoded random token.
func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Ensure *service satisfies Service at compile time.
var _ Service = (*service)(nil)
