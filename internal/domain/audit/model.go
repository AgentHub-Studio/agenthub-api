// Package audit provides append-only audit logging for tenant operations.
package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when an AuditLog entry is not found.
var ErrNotFound = errors.New("audit log not found")

// ErrValidation é retornado quando RecordRequest falha validação
// server-side (entityType/action vazios).
var ErrValidation = errors.New("audit: validation failed")

const (
	// AuditRetentionSettingKey is the ah_core platform setting that owns the policy.
	AuditRetentionSettingKey = "security.audit_retention_days"
	// DefaultAuditRetentionDays mirrors ah_core security.audit_retention_days.
	DefaultAuditRetentionDays = 365
	// MinAuditRetentionDays is the platform floor tenants cannot lower.
	MinAuditRetentionDays = 365
)

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
func ResponseFrom(l AuditLog) AuditLogResponse {
	return AuditLogResponse{
		ID:         l.ID,
		EntityType: l.EntityType,
		EntityID:   l.EntityID,
		Action:     l.Action,
		ActorID:    l.ActorID,
		ActorEmail: l.ActorEmail,
		OldValue:   redactPublicAuditField(l.OldValue),
		NewValue:   redactPublicAuditField(l.NewValue),
		Metadata:   redactPublicAuditField(l.Metadata),
		IPAddress:  l.IPAddress,
		CreatedAt:  l.CreatedAt,
	}
}

var sensitiveAuditDiagnosticPattern = regexp.MustCompile(`(?im)\b(?:authorization|proxy-authorization|cookie|set-cookie|x-api-key|api-key|x-api-token|x-auth-token|x-access-token|x-secret|api[_-]?key|client[_-]?secret|password|secret|[a-z][a-z0-9_.-]*(?:token|credential))\b\s*[:=]\s*(?:bearer[ \t]+)?[^\s,;"}\]]+`)

// redactPublicAuditField creates a response-only copy of audit data without
// credentials. Audit rows remain immutable so operators can retain the
// original record under the database access boundary.
func redactPublicAuditField(raw string) string {
	if raw == "" {
		return raw
	}

	var value any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return redactPublicAuditText(raw)
	}

	var redacted bytes.Buffer
	encoder := json.NewEncoder(&redacted)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(redactPublicAuditValue(value)); err != nil {
		return redactPublicAuditText(raw)
	}
	return redactPublicAuditText(strings.TrimSuffix(redacted.String(), "\n"))
}

func redactPublicAuditValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if isSensitiveAuditKey(key) {
				continue
			}
			out[key] = redactPublicAuditValue(child)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, child := range v {
			out[i] = redactPublicAuditValue(child)
		}
		return out
	case string:
		return redactPublicAuditText(v)
	default:
		return value
	}
}

func isSensitiveAuditKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(key))
	if normalized == "authorization" || normalized == "proxyauthorization" ||
		normalized == "cookie" || normalized == "setcookie" ||
		normalized == "xapikey" || normalized == "xapitoken" ||
		normalized == "xauthtoken" || normalized == "xaccesstoken" ||
		normalized == "xsecret" {
		return true
	}
	for _, suffix := range []string{"apikey", "secret", "password", "token", "credential"} {
		if strings.HasSuffix(normalized, suffix) {
			return true
		}
	}
	return false
}

func redactPublicAuditText(value string) string {
	return sensitiveAuditDiagnosticPattern.ReplaceAllString(value, "[REDACTED]")
}

// AuditExportFilterResponse describes the filters used to build an export.
type AuditExportFilterResponse struct {
	EntityType string     `json:"entityType,omitempty"`
	EntityID   string     `json:"entityId,omitempty"`
	Action     string     `json:"action,omitempty"`
	ActorID    string     `json:"actorId,omitempty"`
	DateFrom   *time.Time `json:"dateFrom,omitempty"`
	DateTo     *time.Time `json:"dateTo,omitempty"`
}

// AuditExportIntegrity carries the checksum for exported records.
type AuditExportIntegrity struct {
	Algorithm string `json:"algorithm"`
	Checksum  string `json:"checksum"`
}

// AuditExportResponse is the public JSON bundle for operational audit logs.
type AuditExportResponse struct {
	TenantID      string                    `json:"tenantId"`
	Format        string                    `json:"format"`
	GeneratedAt   time.Time                 `json:"generatedAt"`
	Filters       AuditExportFilterResponse `json:"filters"`
	Page          int                       `json:"page"`
	Size          int                       `json:"size"`
	TotalElements int64                     `json:"totalElements"`
	Records       []AuditLogResponse        `json:"records"`
	Integrity     AuditExportIntegrity      `json:"integrity"`
}

// AuditExportBundleFile describes one file inside a bundled export.
type AuditExportBundleFile struct {
	Path      string `json:"path"`
	Format    string `json:"format"`
	Bytes     int64  `json:"bytes"`
	Algorithm string `json:"algorithm"`
	Checksum  string `json:"checksum"`
}

// AuditExportBundleManifest describes a multi-file export bundle.
type AuditExportBundleManifest struct {
	TenantID      string                    `json:"tenantId"`
	Format        string                    `json:"format"`
	GeneratedAt   time.Time                 `json:"generatedAt"`
	Filters       AuditExportFilterResponse `json:"filters"`
	Page          int                       `json:"page"`
	Size          int                       `json:"size"`
	TotalElements int64                     `json:"totalElements"`
	Files         []AuditExportBundleFile   `json:"files"`
	Integrity     AuditExportIntegrity      `json:"integrity"`
}

// AuditRetentionRequest asks the audit service to apply the platform retention policy.
type AuditRetentionRequest struct {
	RetentionDays int  `json:"retentionDays"`
	DryRun        bool `json:"dryRun"`
}

// AuditRetentionResponse reports the result of applying operational retention.
type AuditRetentionResponse struct {
	TenantID      string    `json:"tenantId"`
	RetentionDays int       `json:"retentionDays"`
	Cutoff        time.Time `json:"cutoff"`
	DryRun        bool      `json:"dryRun"`
	Deleted       int       `json:"deleted"`
}

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

// ExportFilterResponseFrom converts internal list filters to the export DTO.
func ExportFilterResponseFrom(f ListFilter) AuditExportFilterResponse {
	return AuditExportFilterResponse(f)
}
