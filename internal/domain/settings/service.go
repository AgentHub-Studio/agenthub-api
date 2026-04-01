package settings

import (
	"context"
	"fmt"
)

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
		return SettingResponse{}, fmt.Errorf("key is required")
	}
	if len(req.Value) == 0 {
		return SettingResponse{}, fmt.Errorf("value is required")
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
