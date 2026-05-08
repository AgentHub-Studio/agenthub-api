package user

import (
	"context"
	"fmt"

	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

// Service defines business logic operations for User management.
type Service interface {
	List(ctx context.Context) ([]UserResponse, error)
	Get(ctx context.Context, userID string) (UserResponse, error)
	Create(ctx context.Context, req CreateUserRequest) (UserResponse, error)
	Update(ctx context.Context, userID string, req UpdateUserRequest) (UserResponse, error)
	Delete(ctx context.Context, userID string) error
	AssignRole(ctx context.Context, userID string, role string) error
	RemoveRole(ctx context.Context, userID string, role string) error
	ResetPassword(ctx context.Context, userID string) error
	ListRoles(ctx context.Context) ([]string, error)
}

type service struct {
	client KeycloakUserClient
}

// NewService creates a new user Service.
func NewService(client KeycloakUserClient) Service {
	return &service{client: client}
}

func (s *service) tenantID(ctx context.Context) (string, error) {
	id := tenant.FromContext(ctx)
	if id == "" {
		return "", fmt.Errorf("user: tenantID not found in context")
	}
	return id, nil
}

func (s *service) List(ctx context.Context) ([]UserResponse, error) {
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	users, err := s.client.ListUsers(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	responses := make([]UserResponse, len(users))
	for i, u := range users {
		responses[i] = ResponseFrom(u)
	}
	return responses, nil
}

func (s *service) Get(ctx context.Context, userID string) (UserResponse, error) {
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return UserResponse{}, err
	}
	u, err := s.client.GetUser(ctx, tenantID, userID)
	if err != nil {
		return UserResponse{}, err
	}
	return ResponseFrom(u), nil
}

func (s *service) Create(ctx context.Context, req CreateUserRequest) (UserResponse, error) {
	if req.Username == "" {
		return UserResponse{}, fmt.Errorf("%w: username is required", ErrValidation)
	}
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return UserResponse{}, err
	}
	u, err := s.client.CreateUser(ctx, tenantID, req)
	if err != nil {
		return UserResponse{}, err
	}
	return ResponseFrom(u), nil
}

func (s *service) Update(ctx context.Context, userID string, req UpdateUserRequest) (UserResponse, error) {
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return UserResponse{}, err
	}
	u, err := s.client.UpdateUser(ctx, tenantID, userID, req)
	if err != nil {
		return UserResponse{}, err
	}
	return ResponseFrom(u), nil
}

func (s *service) Delete(ctx context.Context, userID string) error {
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return err
	}
	return s.client.DeleteUser(ctx, tenantID, userID)
}

func (s *service) AssignRole(ctx context.Context, userID string, role string) error {
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return err
	}
	return s.client.AssignRole(ctx, tenantID, userID, role)
}

func (s *service) RemoveRole(ctx context.Context, userID string, role string) error {
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return err
	}
	return s.client.RemoveRole(ctx, tenantID, userID, role)
}

func (s *service) ResetPassword(ctx context.Context, userID string) error {
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return err
	}
	return s.client.ResetPassword(ctx, tenantID, userID)
}

func (s *service) ListRoles(ctx context.Context) ([]string, error) {
	tenantID, err := s.tenantID(ctx)
	if err != nil {
		return nil, err
	}
	return s.client.ListRoles(ctx, tenantID)
}
