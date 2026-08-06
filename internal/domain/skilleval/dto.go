package skilleval

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SuiteResponse is the JSON representation of an EvalSuite.
type SuiteResponse struct {
	ID          uuid.UUID `json:"id"`
	SkillID     uuid.UUID `json:"skillId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func SuiteResponseFrom(s EvalSuite) SuiteResponse {
	return SuiteResponse(s)
}

// CaseResponse is the JSON representation of an EvalCase.
type CaseResponse struct {
	ID             uuid.UUID       `json:"id"`
	SuiteID        uuid.UUID       `json:"suiteId"`
	Description    string          `json:"description"`
	InputText      string          `json:"inputText"`
	ExpectedTool   string          `json:"expectedTool,omitempty"`
	ExpectedOutput string          `json:"expectedOutput,omitempty"`
	GraderType     GraderType      `json:"graderType"`
	GraderConfig   json.RawMessage `json:"graderConfig,omitempty"`
	ShouldTrigger  bool            `json:"shouldTrigger"`
	CreatedAt      time.Time       `json:"createdAt"`
}

func CaseResponseFrom(ec EvalCase) CaseResponse {
	return CaseResponse(ec)
}

// RunResponse is the JSON representation of an EvalRun, optionally with case results.
type RunResponse struct {
	ID          uuid.UUID        `json:"id"`
	SuiteID     uuid.UUID        `json:"suiteId"`
	Status      RunStatus        `json:"status"`
	TotalCases  int              `json:"totalCases"`
	PassedCases int              `json:"passedCases"`
	FailedCases int              `json:"failedCases"`
	StartedAt   time.Time        `json:"startedAt"`
	FinishedAt  *time.Time       `json:"finishedAt,omitempty"`
	Results     []ResultResponse `json:"results,omitempty"`
}

func RunResponseFrom(run EvalRun, results []CaseResult) RunResponse {
	resp := RunResponse{
		ID:          run.ID,
		SuiteID:     run.SuiteID,
		Status:      run.Status,
		TotalCases:  run.TotalCases,
		PassedCases: run.PassedCases,
		FailedCases: run.FailedCases,
		StartedAt:   run.StartedAt,
		FinishedAt:  run.FinishedAt,
	}
	if len(results) > 0 {
		resp.Results = make([]ResultResponse, len(results))
		for i, r := range results {
			resp.Results[i] = ResultResponseFrom(r)
		}
	}
	return resp
}

// ResultResponse is the JSON representation of a CaseResult.
type ResultResponse struct {
	ID           uuid.UUID `json:"id"`
	RunID        uuid.UUID `json:"runId"`
	CaseID       uuid.UUID `json:"caseId"`
	Passed       bool      `json:"passed"`
	ActualOutput string    `json:"actualOutput,omitempty"`
	Score        *float64  `json:"score,omitempty"`
	ErrorMsg     string    `json:"errorMsg,omitempty"`
	DurationMs   int       `json:"durationMs"`
	CreatedAt    time.Time `json:"createdAt"`
}

var sensitiveResultDiagnosticLinePattern = regexp.MustCompile(`(?im)(^|:[\t ]+)[\t ]*(?:authorization|proxy-authorization|cookie|set-cookie|x-api-key|api-key|x-auth-token|password|api[_-]?key|secret|client[_-]?secret)[\t ]*[:=][^\r\n]*`)

func ResultResponseFrom(r CaseResult) ResultResponse {
	response := ResultResponse(r)
	if r.ErrorMsg != "" {
		response.ErrorMsg = redactResultDiagnostic(r.ErrorMsg)
	}
	return response
}

func redactResultDiagnostic(value string) string {
	var payload any
	if err := json.Unmarshal([]byte(value), &payload); err != nil {
		return sensitiveResultDiagnosticLinePattern.ReplaceAllString(value, "$1[REDACTED]")
	}

	redacted, err := json.Marshal(redactResultDiagnosticValue(payload))
	if err != nil {
		return sensitiveResultDiagnosticLinePattern.ReplaceAllString(value, "$1[REDACTED]")
	}
	return sensitiveResultDiagnosticLinePattern.ReplaceAllString(string(redacted), "$1[REDACTED]")
}

func redactResultDiagnosticValue(value any) any {
	switch current := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(current))
		for key, child := range current {
			if isSensitiveResultDiagnosticKey(key) {
				continue
			}
			out[key] = redactResultDiagnosticValue(child)
		}
		return out
	case []any:
		out := make([]any, len(current))
		for index, child := range current {
			out[index] = redactResultDiagnosticValue(child)
		}
		return out
	case string:
		return sensitiveResultDiagnosticLinePattern.ReplaceAllString(current, "$1[REDACTED]")
	default:
		return value
	}
}

func isSensitiveResultDiagnosticKey(key string) bool {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "", ".", "").Replace(key))
	switch normalized {
	case "authorization", "proxyauthorization", "cookie", "setcookie", "authtoken", "xauthtoken", "accesstoken", "refreshtoken", "apikey", "xapikey", "password", "secret", "clientsecret", "bearertoken":
		return true
	default:
		return false
	}
}

// --- request types ---

// CreateSuiteRequest is the JSON body for creating an eval suite.
type CreateSuiteRequest struct {
	SkillID     uuid.UUID `json:"skillId"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
}

// CreateCaseRequest is the JSON body for adding a test case to a suite.
type CreateCaseRequest struct {
	Description    string          `json:"description,omitempty"`
	InputText      string          `json:"inputText"`
	ExpectedTool   string          `json:"expectedTool,omitempty"`
	ExpectedOutput string          `json:"expectedOutput,omitempty"`
	GraderType     GraderType      `json:"graderType,omitempty"`
	GraderConfig   json.RawMessage `json:"graderConfig,omitempty"`
	ShouldTrigger  *bool           `json:"shouldTrigger,omitempty"`
}
