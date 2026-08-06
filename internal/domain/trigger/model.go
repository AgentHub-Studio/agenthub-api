package trigger

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when a trigger cannot be found.
var ErrNotFound = errors.New("trigger: not found")

// ErrAgentNotFound é retornado quando o agentId em
// POST /api/agents/{agentId}/triggers não existe (FK 23503).
var ErrAgentNotFound = errors.New("trigger: agent not found")

// ErrDuplicateName é retornado quando já existe um trigger com o
// mesmo name para o agente. Mapeado para 409 no handler.
var ErrDuplicateName = errors.New("trigger: a trigger with this name already exists for the agent")

// RunStatus represents the state of a trigger run.
type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
)

// AgentTrigger represents a scheduled execution of an agent.
type AgentTrigger struct {
	ID             uuid.UUID       `json:"id"`
	AgentID        uuid.UUID       `json:"agentId"`
	Name           string          `json:"name"`
	CronExpression string          `json:"cronExpression"`
	Enabled        bool            `json:"enabled"`
	InputTemplate  json.RawMessage `json:"inputTemplate,omitempty"`
	LastRunAt      *time.Time      `json:"lastRunAt,omitempty"`
	NextRunAt      *time.Time      `json:"nextRunAt,omitempty"`
	RunCount       int             `json:"runCount"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// AgentTriggerRun represents a single execution of a trigger.
type AgentTriggerRun struct {
	ID          uuid.UUID  `json:"id"`
	TriggerID   uuid.UUID  `json:"triggerId"`
	SessionID   uuid.UUID  `json:"sessionId"`
	Status      RunStatus  `json:"status"`
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	TotalTurns  *int       `json:"totalTurns,omitempty"`
	TotalTokens *int       `json:"totalTokens,omitempty"`
	Error       *string    `json:"error,omitempty"`
}

var sensitiveTriggerRunDiagnosticLinePattern = regexp.MustCompile(`(?im)(^|:[\t ]+)[\t ]*(?:authorization|proxy-authorization|cookie|set-cookie|x-api-key|api-key|x-auth-token|password|api[_-]?key|secret|client[_-]?secret)[\t ]*[:=][^\r\n]*`)

// PublicRunResponseFrom returns a copy safe to serialize at the trigger history boundary.
func PublicRunResponseFrom(run AgentTriggerRun) AgentTriggerRun {
	public := run
	if run.Error != nil {
		redacted := redactTriggerRunDiagnostic(*run.Error)
		public.Error = &redacted
	}
	return public
}

func redactTriggerRunDiagnostic(value string) string {
	var payload any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return sensitiveTriggerRunDiagnosticLinePattern.ReplaceAllString(value, "$1[REDACTED]")
	}

	redacted, err := json.Marshal(redactTriggerRunDiagnosticValue(payload))
	if err != nil {
		return sensitiveTriggerRunDiagnosticLinePattern.ReplaceAllString(value, "$1[REDACTED]")
	}
	return sensitiveTriggerRunDiagnosticLinePattern.ReplaceAllString(string(redacted), "$1[REDACTED]")
}

func redactTriggerRunDiagnosticValue(value any) any {
	switch current := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(current))
		for key, child := range current {
			if isSensitiveTriggerRunDiagnosticKey(key) {
				continue
			}
			out[key] = redactTriggerRunDiagnosticValue(child)
		}
		return out
	case []any:
		out := make([]any, len(current))
		for index, child := range current {
			out[index] = redactTriggerRunDiagnosticValue(child)
		}
		return out
	case string:
		return sensitiveTriggerRunDiagnosticLinePattern.ReplaceAllString(current, "$1[REDACTED]")
	default:
		return value
	}
}

func isSensitiveTriggerRunDiagnosticKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(key))
	switch normalized {
	case "authorization", "proxyauthorization", "cookie", "setcookie", "authtoken", "xauthtoken", "accesstoken", "refreshtoken", "apikey", "xapikey", "password", "secret", "clientsecret", "bearertoken":
		return true
	default:
		return false
	}
}

// CreateTriggerRequest is the payload for creating a trigger.
type CreateTriggerRequest struct {
	Name           string          `json:"name"`
	CronExpression string          `json:"cronExpression"`
	Enabled        *bool           `json:"enabled,omitempty"`
	InputTemplate  json.RawMessage `json:"inputTemplate,omitempty"`
}

// UpdateTriggerRequest is the payload for updating a trigger.
type UpdateTriggerRequest struct {
	Name           *string          `json:"name,omitempty"`
	CronExpression *string          `json:"cronExpression,omitempty"`
	Enabled        *bool            `json:"enabled,omitempty"`
	InputTemplate  *json.RawMessage `json:"inputTemplate,omitempty"`
}
