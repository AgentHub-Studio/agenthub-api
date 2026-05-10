package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

// settingsKeyPattern restringe key a alphanumeric + dot-notation. Bug 249:
// sem este gate, keys com HTML/espaços/slash eram aceitas e renderizadas
// na UI Settings, virando vetor de XSS.
var settingsKeyPattern = regexp.MustCompile(`^[A-Za-z0-9_\-.]{1,255}$`)

// Service defines business logic operations for Setting.
type Service interface {
	List(ctx context.Context) ([]SettingResponse, error)
	Get(ctx context.Context, key string) (SettingResponse, error)
	Upsert(ctx context.Context, key string, req UpdateSettingRequest) (SettingResponse, error)
	Delete(ctx context.Context, key string) error
}

type service struct {
	repo Repository
}

// NewService creates a new settings Service.
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) List(ctx context.Context) ([]SettingResponse, error) {
	settings, err := s.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	responses := make([]SettingResponse, len(settings))
	for i, setting := range settings {
		responses[i] = ResponseFrom(setting)
	}
	return responses, nil
}

func (s *service) Get(ctx context.Context, key string) (SettingResponse, error) {
	setting, err := s.repo.FindByKey(ctx, key)
	if err != nil {
		return SettingResponse{}, err
	}
	return ResponseFrom(setting), nil
}

func (s *service) Upsert(ctx context.Context, key string, req UpdateSettingRequest) (SettingResponse, error) {
	if key == "" {
		return SettingResponse{}, fmt.Errorf("%w: key is required", ErrValidation)
	}
	// Bug 134: key varchar(255) — gate length antes do INSERT.
	if len(key) > 255 {
		return SettingResponse{}, fmt.Errorf("%w: key exceeds maximum length of 255 chars (got %d)", ErrValidation, len(key))
	}
	// Bug 249: gate restritivo no key (alphanumeric + ._-). Settings usam
	// dot-notation (ex: "openrouter.apiKey", "agent.defaultModel"). Sem
	// este gate, keys com HTML/espaços/path-traversal eram silenciosamente
	// aceitas — vetor XSS quando a UI Settings renderiza key sem escape.
	if !settingsKeyPattern.MatchString(key) {
		return SettingResponse{}, fmt.Errorf("%w: key must match pattern [A-Za-z0-9_\\-.]{1,255} (got %q)", ErrValidation, key)
	}
	if len(req.Value) == 0 {
		return SettingResponse{}, fmt.Errorf("%w: value is required", ErrValidation)
	}
	// Bug 165: cap value em 64KB. Settings armazenam URLs/api keys/JSON
	// configs — todos cabem confortavelmente; 200KB+ é storage waste.
	if len(req.Value) > 64*1024 {
		return SettingResponse{}, fmt.Errorf("%w: value exceeds maximum size of 65536 bytes (got %d)", ErrValidation, len(req.Value))
	}

	// Bug 102 (CRÍTICO): qualquer chave terminando em ".baseUrl" é consumida
	// pelo model_factory para construir o cliente LLM. Sem SSRF, um admin
	// malicioso (ou comprometido) poderia redirecionar todas as chamadas
	// LLM para localhost / 169.254 / cluster DNS e exfiltrar prompts.
	if strings.HasSuffix(strings.ToLower(key), ".baseurl") {
		var url string
		if err := json.Unmarshal(req.Value, &url); err != nil {
			return SettingResponse{}, fmt.Errorf("%w: %s must be a JSON string URL", ErrValidation, key)
		}
		if err := ssrf.ValidateURL(url); err != nil {
			return SettingResponse{}, fmt.Errorf("%w: %s invalid (%v)", ErrValidation, key, err)
		}
	}

	var description string
	if req.Description != nil {
		description = *req.Description
	}

	setting := Setting{
		Key:         key,
		Value:       req.Value,
		Description: description,
	}
	created, err := s.repo.Upsert(ctx, setting)
	if err != nil {
		return SettingResponse{}, err
	}
	return ResponseFrom(created), nil
}

func (s *service) Delete(ctx context.Context, key string) error {
	return s.repo.Delete(ctx, key)
}
