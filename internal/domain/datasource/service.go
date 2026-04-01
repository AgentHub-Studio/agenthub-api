package datasource

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// DataSourceRepository defines the persistence interface for DataSource.
type DataSourceRepository interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]DataSource, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (DataSource, error)
	Create(ctx context.Context, tenantID string, d DataSource) (DataSource, error)
	Update(ctx context.Context, tenantID string, id uuid.UUID, d DataSource) (DataSource, error)
	Delete(ctx context.Context, tenantID string, id uuid.UUID) error
}

// Service implements business logic for datasources.
type Service struct {
	repo DataSourceRepository
}

// NewService creates a new Service.
func NewService(repo DataSourceRepository) *Service {
	return &Service{repo: repo}
}

// ListAll returns a paginated list of datasources.
func (s *Service) ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]DataSource, int, error) {
	return s.repo.ListAll(ctx, tenantID, pr)
}

// GetByID retrieves a datasource by ID.
func (s *Service) GetByID(ctx context.Context, tenantID string, id uuid.UUID) (DataSource, error) {
	return s.repo.GetByID(ctx, tenantID, id)
}

// defaultPort returns the conventional port number for the given DataSourceType.
func defaultPort(t DataSourceType) int {
	switch t {
	case DataSourceTypeMySQL:
		return 3306
	case DataSourceTypeSQLServer:
		return 1433
	default: // POSTGRESQL
		return 5432
	}
}

// applyDefaults fills Port with the type default when zero.
func applyDefaults(req *CreateRequest) {
	if req.Port == 0 {
		req.Port = defaultPort(req.Type)
	}
}

// validateRequest checks required fields and type constraints.
func validateRequest(req CreateRequest) error {
	if req.Name == "" {
		return fmt.Errorf("datasource: name is required")
	}
	switch req.Type {
	case DataSourceTypePostgreSQL, DataSourceTypeMySQL, DataSourceTypeSQLServer:
	default:
		return fmt.Errorf("datasource: unsupported type: %s", req.Type)
	}
	if req.Host == "" {
		return fmt.Errorf("datasource: host is required")
	}
	return nil
}

// Create creates a new datasource.
func (s *Service) Create(ctx context.Context, tenantID string, req CreateRequest) (DataSource, error) {
	applyDefaults(&req)
	if err := validateRequest(req); err != nil {
		return DataSource{}, err
	}
	d := DataSource{
		Name:          req.Name,
		Type:          req.Type,
		Host:          req.Host,
		Port:          req.Port,
		Database:      req.Database,
		DBUser:        req.DBUser,
		DBPassword:    req.DBPassword,
		VpnResourceID: req.VpnResourceID,
	}
	return s.repo.Create(ctx, tenantID, d)
}

// Update updates an existing datasource.
func (s *Service) Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (DataSource, error) {
	applyDefaults(&req)
	if err := validateRequest(req); err != nil {
		return DataSource{}, err
	}
	d := DataSource{
		Name:          req.Name,
		Type:          req.Type,
		Host:          req.Host,
		Port:          req.Port,
		Database:      req.Database,
		DBUser:        req.DBUser,
		DBPassword:    req.DBPassword,
		VpnResourceID: req.VpnResourceID,
	}
	return s.repo.Update(ctx, tenantID, id, d)
}

// Delete removes a datasource.
func (s *Service) Delete(ctx context.Context, tenantID string, id uuid.UUID) error {
	return s.repo.Delete(ctx, tenantID, id)
}

// GetCredentials returns the full datasource including password.
// This is for internal/proxy use only and must never be exposed publicly.
func (s *Service) GetCredentials(ctx context.Context, tenantID string, id uuid.UUID) (DataSourceCredentials, error) {
	d, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return DataSourceCredentials{}, err
	}
	return DataSourceCredentials{
		ID:       d.ID,
		Name:     d.Name,
		Type:     d.Type,
		Host:     d.Host,
		Port:     d.Port,
		Database: d.Database,
		User:     d.DBUser,
		Password: d.DBPassword,
	}, nil
}
