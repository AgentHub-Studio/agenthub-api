// Package datasource manages database connection configurations per tenant.
package datasource

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a DataSource is not found.
var ErrNotFound = errors.New("datasource not found")

// DataSourceType represents the database engine type.
type DataSourceType string

const (
	DataSourceTypePostgreSQL DataSourceType = "POSTGRESQL"
	DataSourceTypeMySQL      DataSourceType = "MYSQL"
	DataSourceTypeSQLServer  DataSourceType = "SQL_SERVER"
)

// DataSource stores connection parameters for a tenant database.
type DataSource struct {
	ID            uuid.UUID      `db:"id"`
	Name          string         `db:"name"`
	Type          DataSourceType `db:"type"`
	Host          string         `db:"host"`
	Port          int            `db:"port"`
	Database      string         `db:"database"`
	DBUser        string         `db:"db_user"`
	DBPassword    string         `db:"db_password"`
	VpnResourceID *uuid.UUID     `db:"vpn_resource_id"`
	CreatedAt     time.Time      `db:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at"`
}

// DataSourceResponse is the public DTO — never exposes DBPassword.
type DataSourceResponse struct {
	ID            uuid.UUID      `json:"id"`
	Name          string         `json:"name"`
	Type          DataSourceType `json:"type"`
	Host          string         `json:"host"`
	Port          int            `json:"port"`
	Database      string         `json:"database"`
	DBUser        string         `json:"dbUser"`
	VpnResourceID *uuid.UUID     `json:"vpnResourceId"`
	CreatedAt     time.Time      `json:"createdAt"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

// ResponseFrom converts a DataSource to its public DTO, omitting the password.
func ResponseFrom(d DataSource) DataSourceResponse {
	return DataSourceResponse{
		ID:            d.ID,
		Name:          d.Name,
		Type:          d.Type,
		Host:          d.Host,
		Port:          d.Port,
		Database:      d.Database,
		DBUser:        d.DBUser,
		VpnResourceID: d.VpnResourceID,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

// DataSourceCredentials is the internal DTO used by the VPN proxy service.
// NEVER expose via public API.
type DataSourceCredentials struct {
	ID       uuid.UUID      `json:"id"`
	Name     string         `json:"name"`
	Type     DataSourceType `json:"type"`
	Host     string         `json:"host"`
	Port     int            `json:"port"`
	Database string         `json:"database"`
	User     string         `json:"user"`
	Password string         `json:"password"`
}

// CreateRequest is the payload for creating or updating a DataSource.
type CreateRequest struct {
	Name          string         `json:"name"`
	Type          DataSourceType `json:"type"`
	Host          string         `json:"host"`
	Port          int            `json:"port"`
	Database      string         `json:"database"`
	DBUser        string         `json:"dbUser"`
	DBPassword    string         `json:"dbPassword"`
	VpnResourceID *uuid.UUID     `json:"vpnResourceId"`
}
