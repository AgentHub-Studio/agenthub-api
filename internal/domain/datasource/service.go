package datasource

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

// DataSourceRepository defines the persistence interface for DataSource.
type DataSourceRepository interface {
	ListAll(ctx context.Context, tenantID string, pr pagination.PageRequest) ([]DataSource, int, error)
	GetByID(ctx context.Context, tenantID string, id uuid.UUID) (DataSource, error)
	ExistsByName(ctx context.Context, tenantID, name string) (bool, error)
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
// Erros são wrapped em ErrValidation pra que o handler possa
// mappear para 422 (em vez do 500 genérico). Sem esse wrap, o
// usuário recebia 500 silencioso na UI quando esquecia preencher
// um campo — feedback errado ("erro do servidor" em vez de
// "corrija seu input").
func validateRequest(req CreateRequest) error {
	if req.Name == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	// Bug 129: name varchar(255) — sem este gate o INSERT vazava SQL
	// error 22001 (500) para o cliente quando admin enviava string longa.
	if len(req.Name) > 255 {
		return fmt.Errorf("%w: name exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Name))
	}
	switch req.Type {
	case DataSourceTypePostgreSQL, DataSourceTypeMySQL, DataSourceTypeSQLServer:
	default:
		return fmt.Errorf("%w: unsupported type: %s", ErrValidation, req.Type)
	}
	if req.Host == "" {
		return fmt.Errorf("%w: host is required", ErrValidation)
	}
	// Bug 103: SQL tool executor abre conexão para data_source.host.
	// Sem esta gate, admin podia apontar para localhost / 169.254 (AWS meta) /
	// k8s service DNS e acessar serviços internos do cluster. RFC1918 (10.x,
	// 172.16-31.x, 192.168.x) é permitido — DBs on-prem alcançados via VPN
	// usam essas faixas legitimamente.
	if err := ssrf.ValidateHost(req.Host); err != nil {
		return fmt.Errorf("%w: host invalid (%v)", ErrValidation, err)
	}
	if req.Port < 1 || req.Port > 65535 {
		return fmt.Errorf("%w: port must be between 1 and 65535 (got %d)", ErrValidation, req.Port)
	}
	// Bug 124: SQL tool executor abre conexão para esses campos. Sem
	// estes gates, admin podia criar datasource com database/username
	// vazios e o erro ("role \"\" does not exist") só aparecia em runtime
	// quando o agent disparava a tool — feedback errado (parecia bug
	// do agent, não config inválida).
	// Password é opcional (trust auth do Postgres / IAM auth).
	if req.Database == "" {
		return fmt.Errorf("%w: database is required", ErrValidation)
	}
	// Bug 132: database/dbUser varchar(255) — gate length.
	if len(req.Database) > 255 {
		return fmt.Errorf("%w: database exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Database))
	}
	if req.DBUser == "" {
		return fmt.Errorf("%w: dbUser is required", ErrValidation)
	}
	if len(req.DBUser) > 255 {
		return fmt.Errorf("%w: dbUser exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.DBUser))
	}
	if len(req.Host) > 255 {
		return fmt.Errorf("%w: host exceeds maximum length of 255 chars (got %d)", ErrValidation, len(req.Host))
	}
	return nil
}

// Create creates a new datasource.
func (s *Service) Create(ctx context.Context, tenantID string, req CreateRequest) (DataSource, error) {
	applyDefaults(&req)
	// Bug 182: strip HTML do name (XSS prevention).
	req.Name = sanitize.StripHTML(req.Name)
	if err := validateRequest(req); err != nil {
		return DataSource{}, err
	}
	exists, err := s.repo.ExistsByName(ctx, tenantID, req.Name)
	if err != nil {
		return DataSource{}, fmt.Errorf("datasource: check duplicate name: %w", err)
	}
	if exists {
		return DataSource{}, ErrDuplicateName
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
	created, err := s.repo.Create(ctx, tenantID, d)
	if err != nil {
		// Bug 232: traduzir FK violation de vpn_resource_id em ErrValidation
		// para o handler retornar 422 (não 500 silencioso).
		if strings.Contains(err.Error(), "data_source_vpn_resource_id_fkey") {
			return DataSource{}, fmt.Errorf("%w: vpnResourceId not found", ErrValidation)
		}
		return DataSource{}, err
	}
	return created, nil
}

// Update updates an existing datasource. PATCH-friendly: campos vazios
// no request são preservados do estado atual (true partial update).
// Backlog #186: handler PATCH delega para Update; sem este merge,
// PATCH {"name":"x"} falhava por type/host required.
func (s *Service) Update(ctx context.Context, tenantID string, id uuid.UUID, req CreateRequest) (DataSource, error) {
	existing, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		return DataSource{}, err
	}
	// Merge: campos vazios no request mantêm o valor atual.
	if req.Name == "" {
		req.Name = existing.Name
	} else {
		// Bug 182: strip HTML (XSS prevention).
		req.Name = sanitize.StripHTML(req.Name)
	}
	if req.Type == "" {
		req.Type = existing.Type
	}
	if req.Host == "" {
		req.Host = existing.Host
	}
	if req.Port == 0 {
		req.Port = existing.Port
	}
	if req.Database == "" {
		req.Database = existing.Database
	}
	if req.DBUser == "" {
		req.DBUser = existing.DBUser
	}
	password := req.DBPassword
	if password == "" {
		password = existing.DBPassword
	}
	if req.VpnResourceID == nil {
		req.VpnResourceID = existing.VpnResourceID
	}
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
		DBPassword:    password,
		VpnResourceID: req.VpnResourceID,
	}
	updated, err := s.repo.Update(ctx, tenantID, id, d)
	if err != nil {
		// Bug 232: traduzir FK violation no PATCH/PUT também.
		if strings.Contains(err.Error(), "data_source_vpn_resource_id_fkey") {
			return DataSource{}, fmt.Errorf("%w: vpnResourceId not found", ErrValidation)
		}
		return DataSource{}, err
	}
	return updated, nil
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
