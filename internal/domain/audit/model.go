// Package audit provides append-only audit logging for tenant operations.
package audit

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when an AuditLog entry is not found.
var ErrNotFound = errors.New("audit log not found")

// AuditAction represents the type of operation recorded in the audit log.
type AuditAction string

const (
	AuditActionCreate  AuditAction = "CREATE"
	AuditActionUpdate  AuditAction = "UPDATE"
	AuditActionDelete  AuditAction = "DELETE"
	AuditActionExecute AuditAction = "EXECUTE"
)

// AuditLog is an immutable record of a tenant operation.
type AuditLog struct {
	ID         uuid.UUID   `db:"id"`
	EntityType string      `db:"entity_type"`
	EntityID   string      `db:"entity_id"`
	Action     AuditAction `db:"action"`
	ActorID    string      `db:"actor_id"`
	ActorEmail string      `db:"actor_email"`
	OldValue   string      `db:"old_value"`
	NewValue   string      `db:"new_value"`
	Metadata   string      `db:"metadata"`
	IPAddress  string      `db:"ip_address"`
	CreatedAt  time.Time   `db:"created_at"`
}

// AuditLogResponse is the public DTO for AuditLog.
type AuditLogResponse struct {
	ID         uuid.UUID   `json:"id"`
	EntityType string      `json:"entityType"`
	EntityID   string      `json:"entityId"`
	Action     AuditAction `json:"action"`
	ActorID    string      `json:"actorId"`
	ActorEmail string      `json:"actorEmail"`
	OldValue   string      `json:"oldValue"`
	NewValue   string      `json:"newValue"`
	Metadata   string      `json:"metadata"`
	IPAddress  string      `json:"ipAddress"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// ResponseFrom converts an AuditLog to its public DTO.
func ResponseFrom(l AuditLog) AuditLogResponse { return AuditLogResponse(l) }

// RecordRequest is the payload for recording an audit log entry.
type RecordRequest struct {
	EntityType string      `json:"entityType"`
	EntityID   string      `json:"entityId"`
	Action     AuditAction `json:"action"`
	ActorID    string      `json:"actorId"`
	ActorEmail string      `json:"actorEmail"`
	OldValue   string      `json:"oldValue"`
	NewValue   string      `json:"newValue"`
	Metadata   string      `json:"metadata"`
	IPAddress  string      `json:"ipAddress"`
}

// ListFilter holds optional filters for listing audit logs.
type ListFilter struct {
	EntityType string
	EntityID   string
	Action     string
	ActorID    string
	DateFrom   *time.Time
	DateTo     *time.Time
}
