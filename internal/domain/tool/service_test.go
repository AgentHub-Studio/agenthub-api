package tool_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/tool"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

type mockToolRepo struct {
	data     map[uuid.UUID]tool.Tool
	bindings []tool.SkillTool
}

type mockToolRepoWithSkillInstructionRefs struct {
	*mockToolRepo
	refs []tool.SkillInstructionReference
}

func newMockRepo() *mockToolRepo {
	return &mockToolRepo{data: make(map[uuid.UUID]tool.Tool)}
}

func (m *mockToolRepo) List(_ context.Context, _ pagination.PageRequest, toolType string) ([]tool.Tool, int64, error) {
	var out []tool.Tool
	for _, t := range m.data {
		if toolType == "" || t.Type == toolType {
			out = append(out, t)
		}
	}
	return out, int64(len(out)), nil
}

func (m *mockToolRepo) Create(_ context.Context, t tool.Tool) (tool.Tool, error) {
	t.ID = uuid.New()
	m.data[t.ID] = t
	return t, nil
}

func (m *mockToolRepo) GetByID(_ context.Context, id uuid.UUID) (tool.Tool, error) {
	t, ok := m.data[id]
	if !ok {
		return tool.Tool{}, tool.ErrNotFound
	}
	return t, nil
}

func (m *mockToolRepo) Update(_ context.Context, id uuid.UUID, t tool.Tool) (tool.Tool, error) {
	if _, ok := m.data[id]; !ok {
		return tool.Tool{}, tool.ErrNotFound
	}
	m.data[id] = t
	return t, nil
}

func (m *mockToolRepo) Delete(_ context.Context, id uuid.UUID) error {
	if _, ok := m.data[id]; !ok {
		return tool.ErrNotFound
	}
	delete(m.data, id)
	return nil
}

func (m *mockToolRepo) BindToSkill(_ context.Context, skillID uuid.UUID, req tool.BindRequest) (tool.SkillTool, error) {
	for _, b := range m.bindings {
		if b.SkillID == skillID && b.ToolID == req.ToolID {
			return tool.SkillTool{}, tool.ErrAlreadyBound
		}
	}
	active := true
	if req.IsActive != nil {
		active = *req.IsActive
	}
	st := tool.SkillTool{
		ID:       uuid.New(),
		SkillID:  skillID,
		ToolID:   req.ToolID,
		Priority: req.Priority,
		IsActive: active,
	}
	m.bindings = append(m.bindings, st)
	return st, nil
}

func (m *mockToolRepo) UnbindFromSkill(_ context.Context, skillID, toolID uuid.UUID) error {
	for i, b := range m.bindings {
		if b.SkillID == skillID && b.ToolID == toolID {
			m.bindings = append(m.bindings[:i], m.bindings[i+1:]...)
			return nil
		}
	}
	return tool.ErrNotFound
}

func (m *mockToolRepo) ListLabels(_ context.Context) ([]string, error) {
	seen := map[string]bool{}
	var labels []string
	for _, t := range m.data {
		for _, l := range t.Labels {
			if !seen[l] {
				seen[l] = true
				labels = append(labels, l)
			}
		}
	}
	return labels, nil
}

func (m *mockToolRepo) ListBySkill(_ context.Context, skillID uuid.UUID) ([]tool.SkillTool, []tool.Tool, error) {
	var bindings []tool.SkillTool
	var tools []tool.Tool
	for _, b := range m.bindings {
		if b.SkillID == skillID {
			bindings = append(bindings, b)
			if t, ok := m.data[b.ToolID]; ok {
				tools = append(tools, t)
			}
		}
	}
	return bindings, tools, nil
}

func (m *mockToolRepoWithSkillInstructionRefs) ListSkillInstructionReferencesByTool(_ context.Context, _ uuid.UUID) ([]tool.SkillInstructionReference, error) {
	return m.refs, nil
}

func TestToolService_Create_Success(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "HTTP POST",
		Type:   "HTTP",
		Config: json.RawMessage(`{"url":"https://api.example.com/endpoint"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "HTTP POST", created.Name)
	assert.NotEqual(t, uuid.Nil, created.ID)
}

func TestToolService_GetByID_NotFound(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.GetByID(context.Background(), uuid.New())
	require.ErrorIs(t, err, tool.ErrNotFound)
}

func TestToolService_BindToSkill_Success(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{Name: "SQL Query", Type: "SQL", Config: json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`)})
	require.NoError(t, err)
	skillID := uuid.New()
	binding, err := svc.BindToSkill(context.Background(), skillID, tool.BindRequest{ToolID: created.ID, Priority: 1})
	require.NoError(t, err)
	assert.Equal(t, skillID, binding.SkillID)
}

func TestToolService_BindToSkill_AlreadyBound(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{Name: "SQL Query", Type: "SQL", Config: json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`)})
	require.NoError(t, err)
	skillID := uuid.New()
	_, err = svc.BindToSkill(context.Background(), skillID, tool.BindRequest{ToolID: created.ID, Priority: 1})
	require.NoError(t, err)
	_, err = svc.BindToSkill(context.Background(), skillID, tool.BindRequest{ToolID: created.ID, Priority: 1})
	require.ErrorIs(t, err, tool.ErrAlreadyBound)
}

func TestToolService_Delete_WarnsWhenSkillInstructionsReferenceDeletedTool(t *testing.T) {
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	toolID := uuid.New()
	skillID := uuid.New()
	repo := &mockToolRepoWithSkillInstructionRefs{
		mockToolRepo: newMockRepo(),
		refs: []tool.SkillInstructionReference{
			{
				SkillID:      skillID,
				SkillName:    "Customer Support Skill",
				Instructions: "Always use customer_lookup before answering. private-token-should-not-log",
			},
		},
	}
	repo.data[toolID] = tool.Tool{
		ID:     toolID,
		Name:   "Customer Lookup",
		Slug:   "customer_lookup",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/customer"}`),
	}

	svc := tool.NewService(repo)
	err := svc.Delete(context.Background(), toolID)

	require.NoError(t, err)
	assert.NotContains(t, repo.data, toolID)
	gotLogs := logs.String()
	assert.Contains(t, gotLogs, "referenced by skill instructions")
	assert.Contains(t, gotLogs, toolID.String())
	assert.Contains(t, gotLogs, skillID.String())
	assert.Contains(t, gotLogs, "Customer Lookup")
	assert.Contains(t, gotLogs, "Customer Support Skill")
	assert.NotContains(t, gotLogs, "private-token-should-not-log")
}

func TestToolService_Create_InvalidType(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "Bad Tool",
		Type: "INVALID_TYPE",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "unsupported type")
}

func TestToolService_Create_MissingName(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{Type: tool.ToolTypeHTTP})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "name")
}

func TestToolService_Create_HTTPRejectsSSRFURL(t *testing.T) {
	tests := []struct {
		name   string
		config json.RawMessage
	}{
		{
			name:   "metadata literal",
			config: json.RawMessage(`{"url":"http://169.254.169.254/latest/meta-data/"}`),
		},
		{
			name:   "localhost fqdn",
			config: json.RawMessage(`{"url":"http://localhost./api"}`),
		},
		{
			name:   "cluster local fqdn",
			config: json.RawMessage(`{"url":"http://keycloak.agenthub.svc.cluster.local./auth"}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			_, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "SSRF Tool",
				Type:   tool.ToolTypeHTTP,
				Config: tc.config,
			})
			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), "invalid URL")
		})
	}
}

func TestToolService_Create_HTTPDoesNotExposeCredentialsFromMalformedURL(t *testing.T) {
	const username = "malformed-url-user"
	const password = "malformed-url-password"
	const apiKey = "malformed-url-api-key"

	rawURL := fmt.Sprintf("https://%s:%s@api.example.com/%%zz?api_key=%s", username, password, apiKey)
	svc := tool.NewService(newMockRepo())

	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "Malformed URL Redaction Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET"
		}`, rawURL)),
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "invalid URL")
	assert.NotContains(t, err.Error(), username)
	assert.NotContains(t, err.Error(), password)
	assert.NotContains(t, err.Error(), apiKey)
}

func TestToolService_Create_RejectsHTMLName(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   `<svg onload="alert(1)">Tool`,
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/endpoint"}`),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "HTML")
}

func TestToolService_Create_RejectsInvalidCanonicalSlug(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "HTTP Tool",
		Slug:   "http_tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/endpoint"}`),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "slug must match")
}

func TestToolService_Create_HTTPRejectsURLTemplateTraversal(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "Traversal Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"urlTemplate":"https://api.example.com/files/../secret"}`),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "path traversal")
}

func TestToolService_Create_HTTPRejectsEncodedSlashURLTemplate(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "Encoded Slash Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"urlTemplate":"https://api.example.com/files/..%2Fsecret"}`),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "encoded slashes")
}

func TestToolService_Create_HTTPRejectsNonStringHeaderValueForRuntime(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "Numeric Header Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/endpoint","headers":{"X-Trace":123}}`),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "header value")
	assert.Contains(t, err.Error(), "X-Trace")
	assert.Contains(t, err.Error(), "string")
}

func TestToolService_Create_HTTPRejectsRuntimeIncompatibleTimeoutAliases(t *testing.T) {
	tests := []struct {
		name   string
		config json.RawMessage
	}{
		{
			name:   "fractional timeoutSeconds",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","timeoutSeconds":1.5}`),
		},
		{
			name:   "snake case timeout too large",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","timeout_seconds":99999}`),
		},
		{
			name:   "timeoutMs too large",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","timeoutMs":600001}`),
		},
		{
			name:   "timeoutMs string",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","timeoutMs":"1000"}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			_, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "HTTP Timeout Tool",
				Type:   tool.ToolTypeHTTP,
				Config: tc.config,
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), "timeout")
		})
	}
}

func TestToolService_Create_HTTPAcceptsSafeURLTemplate(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	resp, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "Safe Template Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"urlTemplate":"https://api.example.com/users/{id}"}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "safe-template-tool", resp.Slug)
}

func TestToolService_Create_HTTPNormalizesMethodForRuntime(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	resp, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "HTTP Method Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/endpoint","method":" post "}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "POST", configString(t, resp.Config, "method"))
}

func TestToolService_Create_HTTPRejectsNonStringMethodForRuntime(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "HTTP Numeric Method Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/endpoint","method":123}`),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "method")
	assert.Contains(t, err.Error(), "string")
}

func TestToolService_Create_HTTPRejectsRuntimeIncompatibleScalarFields(t *testing.T) {
	tests := []struct {
		name   string
		field  string
		config json.RawMessage
	}{
		{
			name:   "authType must be string",
			field:  "authType",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","authType":123}`),
		},
		{
			name:   "auth_token must be string",
			field:  "auth_token",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","auth_type":"bearer","auth_token":{"secret":"x"}}`),
		},
		{
			name:   "useCallerToken must be bool",
			field:  "useCallerToken",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","useCallerToken":"true"}`),
		},
		{
			name:   "use_caller_token must be bool",
			field:  "use_caller_token",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","use_caller_token":"true"}`),
		},
		{
			name:   "baseUrl must be string",
			field:  "baseUrl",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","baseUrl":123}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			_, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "HTTP Scalar Tool",
				Type:   tool.ToolTypeHTTP,
				Config: tc.config,
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), tc.field)
		})
	}
}

func TestToolService_Create_HTTPRejectsInvalidAuthTypeForRuntime(t *testing.T) {
	tests := []struct {
		name   string
		config json.RawMessage
	}{
		{
			name:   "unknown camel case auth type",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","authType":"api_key","authToken":"secret"}`),
		},
		{
			name:   "unknown snake case auth type",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","auth_type":"oauth","auth_token":"secret"}`),
		},
		{
			name:   "auth type with whitespace",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","authType":" bearer ","authToken":"secret"}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			_, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "HTTP Auth Type Tool",
				Type:   tool.ToolTypeHTTP,
				Config: tc.config,
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), "auth")
		})
	}
}

func TestToolService_HTTPRejectsConflictingRuntimeAliases(t *testing.T) {
	tests := []struct {
		name   string
		config json.RawMessage
		field  string
	}{
		{
			name:   "body aliases",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","bodyTemplate":"{\"source\":\"camel\"}","body":"{\"source\":\"legacy\"}"}`),
			field:  "bodyTemplate",
		},
		{
			name:   "URL aliases",
			config: json.RawMessage(`{"url":"https://api.example.com/primary","urlTemplate":"https://api.example.com/legacy"}`),
			field:  "url",
		},
		{
			name:   "timeout aliases",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","timeoutSeconds":5,"timeout_seconds":10}`),
			field:  "timeoutSeconds",
		},
		{
			name:   "authentication type aliases",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","authType":"bearer","auth_type":"none"}`),
			field:  "authType",
		},
		{
			name:   "authentication token aliases",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","authToken":"configured-token-a","auth_token":"configured-token-b"}`),
			field:  "authToken",
		},
		{
			name:   "caller token aliases",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","useCallerToken":false,"use_caller_token":true}`),
			field:  "useCallerToken",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name+" on create", func(t *testing.T) {
			repo := newMockRepo()
			svc := tool.NewService(repo)

			_, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "Conflicting HTTP aliases",
				Type:   tool.ToolTypeHTTP,
				Config: tc.config,
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), tc.field)
			assert.Empty(t, repo.data, "invalid configuration must not be persisted")
		})

		t.Run(tc.name+" on update", func(t *testing.T) {
			repo := newMockRepo()
			svc := tool.NewService(repo)
			created, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "Stable HTTP aliases",
				Type:   tool.ToolTypeHTTP,
				Config: json.RawMessage(`{"url":"https://api.example.com/endpoint","method":"POST"}`),
			})
			require.NoError(t, err)
			persistedConfig := append(json.RawMessage(nil), repo.data[created.ID].Config...)

			_, err = svc.Update(context.Background(), created.ID, tool.UpdateRequest{Config: tc.config})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), tc.field)
			assert.JSONEq(t, string(persistedConfig), string(repo.data[created.ID].Config), "invalid update must not change persisted configuration")
		})
	}
}

func TestToolService_HTTPAllowsEquivalentRuntimeAliases(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "Equivalent HTTP aliases",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(`{
			"url":"https://api.example.com/endpoint",
			"urlTemplate":"https://api.example.com/endpoint",
			"bodyTemplate":"{\"source\":\"shared\"}",
			"body":{"source":"shared"},
			"timeoutSeconds":2,
			"timeoutMs":1500,
			"authType":"Bearer",
			"auth_type":"bearer",
			"authToken":"configured-token",
			"auth_token":"configured-token",
			"useCallerToken":true,
			"use_caller_token":true
		}`),
	})

	require.NoError(t, err)
}

func TestToolService_Create_HTTPRejectsOversizedRuntimeConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  json.RawMessage
		message string
	}{
		{
			name:    "url exceeds 2048 characters",
			config:  json.RawMessage(fmt.Sprintf(`{"url":"https://api.example.com/%s"}`, strings.Repeat("a", 2049))),
			message: "url exceeds maximum length",
		},
		{
			name:    "bodyTemplate exceeds 32000 characters",
			config:  json.RawMessage(fmt.Sprintf(`{"url":"https://api.example.com/endpoint","method":"POST","bodyTemplate":%q}`, strings.Repeat("x", 32001))),
			message: "bodyTemplate exceeds maximum length",
		},
		{
			name:    "headers exceeds 50 entries",
			config:  oversizedHeadersConfig(t, "https://api.example.com/endpoint"),
			message: "headers exceeds maximum",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			_, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "HTTP Oversized Config Tool",
				Type:   tool.ToolTypeHTTP,
				Config: tc.config,
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}

func TestToolService_HTTPRejectsSensitiveBodyTemplatePlaceholders(t *testing.T) {
	tests := []struct {
		name   string
		config json.RawMessage
	}{
		{
			name:   "snake_case body template auth token",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","method":"POST","body_template":"{\"token\":\"{{auth_token}}\"}"}`),
		},
		{
			name:   "camelCase body template namespaced password",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","method":"POST","bodyTemplate":"{\"password\":\"{{input.password}}\"}"}`),
		},
		{
			name:   "body alias args api key",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","method":"POST","body":"{\"key\":\"{{args.api_key}}\"}"}`),
		},
		{
			name:   "object body template secret",
			config: json.RawMessage(`{"url":"https://api.example.com/endpoint","method":"POST","bodyTemplate":{"secret":"{{secret}}"}}`),
		},
	}

	for _, tc := range tests {
		t.Run("create "+tc.name, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())

			_, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "HTTP Sensitive Body Tool",
				Type:   tool.ToolTypeHTTP,
				Config: tc.config,
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), "body")
			assert.Contains(t, err.Error(), "sensitive")
		})

		t.Run("update "+tc.name, func(t *testing.T) {
			repo := newMockRepo()
			toolID := uuid.New()
			repo.data[toolID] = tool.Tool{
				ID:     toolID,
				Name:   "HTTP Existing Tool",
				Type:   tool.ToolTypeHTTP,
				Config: json.RawMessage(`{"url":"https://api.example.com/endpoint","method":"POST"}`),
			}
			svc := tool.NewService(repo)

			_, err := svc.Update(context.Background(), toolID, tool.UpdateRequest{
				Config: tc.config,
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), "body")
			assert.Contains(t, err.Error(), "sensitive")
		})
	}
}

func TestToolService_HTTPRejectsUnsupportedURLScheme(t *testing.T) {
	config := json.RawMessage(`{"url":"ftp://[2606:4700:4700::1111]/resource","method":"GET"}`)

	t.Run("create", func(t *testing.T) {
		svc := tool.NewService(newMockRepo())

		_, err := svc.Create(context.Background(), tool.CreateRequest{
			Name:   "HTTP Unsupported Scheme Tool",
			Type:   tool.ToolTypeHTTP,
			Config: config,
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, tool.ErrValidation)
		assert.Contains(t, err.Error(), "scheme")
	})

	t.Run("update", func(t *testing.T) {
		repo := newMockRepo()
		toolID := uuid.New()
		repo.data[toolID] = tool.Tool{
			ID:     toolID,
			Name:   "HTTP Existing Tool",
			Type:   tool.ToolTypeHTTP,
			Config: json.RawMessage(`{"url":"https://api.example.com/endpoint","method":"GET"}`),
		}
		svc := tool.NewService(repo)

		_, err := svc.Update(context.Background(), toolID, tool.UpdateRequest{
			Config: config,
		})

		require.Error(t, err)
		assert.ErrorIs(t, err, tool.ErrValidation)
		assert.Contains(t, err.Error(), "scheme")
	})
}

func TestToolService_Create_HTTPAcceptsBackendRelativeURLForRuntime(t *testing.T) {
	t.Setenv("BACKEND_BASE_URL", "http://agenthub-api:8081/")
	svc := tool.NewService(newMockRepo())

	resp, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "Core Relative Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"/api/agents","method":"GET","useCallerToken":true}`),
	})

	require.NoError(t, err)
	assert.Equal(t, "/api/agents", configString(t, resp.Config, "url"))
}

func TestToolService_Create_WithLabels(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "Tagged Tool",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/v1"}`),
		Labels: []string{"prod", "external"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"prod", "external"}, created.Labels)
}

func TestToolService_Create_ValidTypes(t *testing.T) {
	httpConfig := json.RawMessage(`{"url":"https://api.example.com/endpoint"}`)
	sqlConfig := json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`)
	docConfig := json.RawMessage(`{"kbId":"00000000-0000-0000-0000-000000000002"}`)
	validTypes := []struct {
		typ    string
		config json.RawMessage
	}{
		{tool.ToolTypeHTTP, httpConfig},
		{tool.ToolTypeSQL, sqlConfig},
		{tool.ToolTypeDocumentSearch, docConfig},
		{tool.ToolTypeCustom, nil},
		{tool.ToolTypeBlockly, nil},
		{tool.ToolTypeComposite, nil},
		{tool.ToolTypeCode, nil},
		{tool.ToolTypeDatabase, sqlConfig},
		{tool.ToolTypeDocuments, docConfig},
	}
	for _, tc := range validTypes {
		t.Run(tc.typ, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			_, err := svc.Create(context.Background(), tool.CreateRequest{Name: "T", Type: tc.typ, Config: tc.config})
			require.NoError(t, err)
		})
	}
}

// --- TR-01-TASK-19: PATCH preserves unset fields (P-C196-1) ---

func TestPatchTool_OnlyDescriptionSent_TypePreserved(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:        "my-tool",
		Type:        tool.ToolTypeHTTP,
		Description: "original",
		Config:      json.RawMessage(`{"url":"https://api.example.com/endpoint"}`),
	})
	require.NoError(t, err)

	newDesc := "updated description"
	updated, err := svc.Update(context.Background(), created.ID, tool.UpdateRequest{
		Description: &newDesc,
	})

	require.NoError(t, err)
	assert.Equal(t, "updated description", updated.Description)
	assert.Equal(t, tool.ToolType(tool.ToolTypeHTTP), updated.Type, "type must be preserved")
	assert.Equal(t, "my-tool", updated.Name, "name must be preserved")
}

func TestPatchTool_OnlyNameSent_TypeAndDescPreserved(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "old-name", Type: tool.ToolTypeSQL, Description: "keep me",
		Config: json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`),
	})
	require.NoError(t, err)

	newName := "new-name"
	updated, err := svc.Update(context.Background(), created.ID, tool.UpdateRequest{
		Name: &newName,
	})

	require.NoError(t, err)
	assert.Equal(t, "new-name", updated.Name)
	assert.Equal(t, tool.ToolType(tool.ToolTypeSQL), updated.Type, "type must be preserved")
	assert.Equal(t, "keep me", updated.Description, "description must be preserved")
}

func TestPatchTool_EmptyBody_NoChanges(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "stable-tool", Type: tool.ToolTypeCustom, Description: "unchanged",
	})
	require.NoError(t, err)

	// Empty UpdateRequest — nothing should change.
	updated, err := svc.Update(context.Background(), created.ID, tool.UpdateRequest{})

	require.NoError(t, err)
	assert.Equal(t, "stable-tool", updated.Name)
	assert.Equal(t, "unchanged", updated.Description)
}

func TestToolService_List_FilterByType(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{Name: "HTTP", Type: "HTTP", Config: json.RawMessage(`{"url":"https://api.example.com/v1"}`)})
	require.NoError(t, err)
	_, err = svc.Create(context.Background(), tool.CreateRequest{Name: "SQL", Type: "SQL", Config: json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`)})
	require.NoError(t, err)
	page, err := svc.List(context.Background(), pagination.PageRequest{Page: 0, Size: 20}, "HTTP")
	require.NoError(t, err)
	assert.Equal(t, int64(1), page.TotalElements)
}

// --- TR-01-TASK-28: URL obrigatória em HTTP tools (P-C254-1) ---

func TestToolService_Create_HTTPMissingURL_Rejected(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "No URL",
		Type: tool.ToolTypeHTTP,
		// Config has no "url" field.
		Config: json.RawMessage(`{"method":"GET"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url")
	assert.Contains(t, err.Error(), "required")
}

func TestToolService_Create_HTTPEmptyConfig_Rejected(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "No Config",
		Type: tool.ToolTypeHTTP,
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url")
}

func TestToolService_Update_HTTPRemoveURL_Rejected(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "With URL",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/v1"}`),
	})
	require.NoError(t, err)

	// Update to remove the URL from config.
	_, err = svc.Update(context.Background(), created.ID, tool.UpdateRequest{
		Config: json.RawMessage(`{"method":"POST"}`),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "url")
}

func TestToolService_Update_HTTPRejectsTrailingDotSSRFURL(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "With URL",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/v1"}`),
	})
	require.NoError(t, err)

	_, err = svc.Update(context.Background(), created.ID, tool.UpdateRequest{
		Config: json.RawMessage(`{"url":"http://minio.cluster.local./buckets"}`),
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "invalid URL")
}

func TestToolService_Update_HTTPNormalizesMethodForRuntime(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "With URL",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/v1","method":"GET"}`),
	})
	require.NoError(t, err)

	updated, err := svc.Update(context.Background(), created.ID, tool.UpdateRequest{
		Config: json.RawMessage(`{"url":"https://api.example.com/v1","method":" patch "}`),
	})
	require.NoError(t, err)
	assert.Equal(t, "PATCH", configString(t, updated.Config, "method"))
}

func TestToolService_Update_HTTPRejectsNonStringMethodForRuntime(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "With URL",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/v1","method":"GET"}`),
	})
	require.NoError(t, err)

	_, err = svc.Update(context.Background(), created.ID, tool.UpdateRequest{
		Config: json.RawMessage(`{"url":"https://api.example.com/v1","method":123}`),
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, tool.ErrValidation)
	assert.Contains(t, err.Error(), "method")
	assert.Contains(t, err.Error(), "string")
}

func TestToolService_Update_HTTPRejectsRuntimeIncompatibleScalarFields(t *testing.T) {
	tests := []struct {
		name   string
		field  string
		config json.RawMessage
	}{
		{
			name:   "authType must be string",
			field:  "authType",
			config: json.RawMessage(`{"url":"https://api.example.com/v1","authType":123}`),
		},
		{
			name:   "auth_token must be string",
			field:  "auth_token",
			config: json.RawMessage(`{"url":"https://api.example.com/v1","auth_type":"bearer","auth_token":{"secret":"x"}}`),
		},
		{
			name:   "useCallerToken must be bool",
			field:  "useCallerToken",
			config: json.RawMessage(`{"url":"https://api.example.com/v1","useCallerToken":"true"}`),
		},
		{
			name:   "use_caller_token must be bool",
			field:  "use_caller_token",
			config: json.RawMessage(`{"url":"https://api.example.com/v1","use_caller_token":"true"}`),
		},
		{
			name:   "baseUrl must be string",
			field:  "baseUrl",
			config: json.RawMessage(`{"url":"https://api.example.com/v1","baseUrl":123}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			created, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "With URL",
				Type:   tool.ToolTypeHTTP,
				Config: json.RawMessage(`{"url":"https://api.example.com/v1","method":"GET"}`),
			})
			require.NoError(t, err)

			_, err = svc.Update(context.Background(), created.ID, tool.UpdateRequest{
				Config: tc.config,
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), tc.field)
		})
	}
}

func TestToolService_Update_HTTPRejectsInvalidAuthTypeForRuntime(t *testing.T) {
	tests := []struct {
		name   string
		config json.RawMessage
	}{
		{
			name:   "unknown camel case auth type",
			config: json.RawMessage(`{"url":"https://api.example.com/v1","authType":"api_key","authToken":"secret"}`),
		},
		{
			name:   "unknown snake case auth type",
			config: json.RawMessage(`{"url":"https://api.example.com/v1","auth_type":"oauth","auth_token":"secret"}`),
		},
		{
			name:   "auth type with whitespace",
			config: json.RawMessage(`{"url":"https://api.example.com/v1","authType":" bearer ","authToken":"secret"}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			created, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "With URL",
				Type:   tool.ToolTypeHTTP,
				Config: json.RawMessage(`{"url":"https://api.example.com/v1","method":"GET"}`),
			})
			require.NoError(t, err)

			_, err = svc.Update(context.Background(), created.ID, tool.UpdateRequest{
				Config: tc.config,
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), "auth")
		})
	}
}

func TestToolService_Update_HTTPRejectsOversizedRuntimeConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  json.RawMessage
		message string
	}{
		{
			name:    "url exceeds 2048 characters",
			config:  json.RawMessage(fmt.Sprintf(`{"url":"https://api.example.com/%s"}`, strings.Repeat("a", 2049))),
			message: "url exceeds maximum length",
		},
		{
			name:    "bodyTemplate exceeds 32000 characters",
			config:  json.RawMessage(fmt.Sprintf(`{"url":"https://api.example.com/endpoint","method":"POST","bodyTemplate":%q}`, strings.Repeat("x", 32001))),
			message: "bodyTemplate exceeds maximum length",
		},
		{
			name:    "headers exceeds 50 entries",
			config:  oversizedHeadersConfig(t, "https://api.example.com/endpoint"),
			message: "headers exceeds maximum",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			created, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "With URL",
				Type:   tool.ToolTypeHTTP,
				Config: json.RawMessage(`{"url":"https://api.example.com/v1","method":"GET"}`),
			})
			require.NoError(t, err)

			_, err = svc.Update(context.Background(), created.ID, tool.UpdateRequest{
				Config: tc.config,
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), tc.message)
		})
	}
}

func TestToolService_Update_HTTPAcceptsBackendRelativeURLForRuntime(t *testing.T) {
	t.Setenv("BACKEND_BASE_URL", "http://agenthub-api:8081/")
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "With URL",
		Type:   tool.ToolTypeHTTP,
		Config: json.RawMessage(`{"url":"https://api.example.com/v1","method":"GET"}`),
	})
	require.NoError(t, err)

	updated, err := svc.Update(context.Background(), created.ID, tool.UpdateRequest{
		Config: json.RawMessage(`{"url":"/api/skills","method":"GET","useCallerToken":true}`),
	})

	require.NoError(t, err)
	assert.Equal(t, "/api/skills", configString(t, updated.Config, "url"))
}

func TestToolService_Update_HTTPRejectsRuntimeIncompatibleTimeoutAliases(t *testing.T) {
	tests := []struct {
		name   string
		config json.RawMessage
	}{
		{
			name:   "fractional timeoutSeconds",
			config: json.RawMessage(`{"url":"https://api.example.com/v1","timeoutSeconds":1.5}`),
		},
		{
			name:   "snake case timeout too large",
			config: json.RawMessage(`{"url":"https://api.example.com/v1","timeout_seconds":99999}`),
		},
		{
			name:   "timeoutMs too large",
			config: json.RawMessage(`{"url":"https://api.example.com/v1","timeoutMs":600001}`),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := tool.NewService(newMockRepo())
			created, err := svc.Create(context.Background(), tool.CreateRequest{
				Name:   "With URL",
				Type:   tool.ToolTypeHTTP,
				Config: json.RawMessage(`{"url":"https://api.example.com/v1","method":"GET"}`),
			})
			require.NoError(t, err)

			_, err = svc.Update(context.Background(), created.ID, tool.UpdateRequest{
				Config: tc.config,
			})

			require.Error(t, err)
			assert.ErrorIs(t, err, tool.ErrValidation)
			assert.Contains(t, err.Error(), "timeout")
		})
	}
}

func TestToolService_NonHTTP_NoURLRequired(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "SQL Tool",
		Type:   tool.ToolTypeSQL,
		Config: json.RawMessage(`{"query":"SELECT 1","datasourceId":"00000000-0000-0000-0000-000000000001"}`),
	})
	require.NoError(t, err)
}

func TestToolService_TestHTTPTool_RendersRuntimePlaceholderAliases(t *testing.T) {
	var gotPath, gotQuery, gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("q")
		gotHeader = r.Header.Get("X-Trace")
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET",
			"headers": {"X-Trace": "{{args.trace}}"}
		}`, srv.URL+"/users/{{args.id}}/search/{{q}}?q={{input.query}}")),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, map[string]any{
		"id":    "42",
		"q":     "alpha",
		"query": "hello world & more",
		"trace": "trace-123",
	})

	require.NoError(t, err)
	assert.Contains(t, out, "Status: 200")
	assert.Equal(t, "/users/42/search/alpha", gotPath)
	assert.Equal(t, "hello world & more", gotQuery)
	assert.Equal(t, "trace-123", gotHeader)
}

func TestToolService_TestHTTPTool_RejectsNonStringHeaderValueForRuntime(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Numeric Header Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET",
			"headers": {"X-Trace": 123}
		}`, srv.URL)),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "header value")
	assert.Contains(t, err.Error(), "X-Trace")
	assert.False(t, called, "invalid header config must fail before request")
}

func TestToolService_TestHTTPTool_RejectsNonStringMethodForRuntime(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Numeric Method Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": 123
		}`, srv.URL)),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "method")
	assert.Contains(t, err.Error(), "string")
	assert.False(t, called, "invalid method config must fail before request")
}

func TestToolService_TestHTTPTool_RejectsRuntimeIncompatibleScalarFields(t *testing.T) {
	tests := []struct {
		name   string
		field  string
		config string
	}{
		{
			name:   "authType must be string",
			field:  "authType",
			config: `{"url":%q,"method":"GET","authType":123}`,
		},
		{
			name:   "auth_token must be string",
			field:  "auth_token",
			config: `{"url":%q,"method":"GET","auth_type":"bearer","auth_token":{"secret":"x"}}`,
		},
		{
			name:   "useCallerToken must be bool",
			field:  "useCallerToken",
			config: `{"url":%q,"method":"GET","useCallerToken":"true"}`,
		},
		{
			name:   "use_caller_token must be bool",
			field:  "use_caller_token",
			config: `{"url":%q,"method":"GET","use_caller_token":"true"}`,
		},
		{
			name:   "baseUrl must be string",
			field:  "baseUrl",
			config: `{"url":%q,"method":"GET","baseUrl":123}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				_, _ = w.Write([]byte("ok"))
			}))
			defer srv.Close()

			repo := newMockRepo()
			toolID := uuid.New()
			repo.data[toolID] = tool.Tool{
				ID:     toolID,
				Name:   "HTTP Scalar Test",
				Type:   tool.ToolTypeHTTP,
				Config: json.RawMessage(fmt.Sprintf(tc.config, srv.URL)),
			}
			svc := newHTTPRuntimeTestService(repo)

			out, err := svc.TestTool(context.Background(), toolID, nil)

			require.Error(t, err)
			assert.Empty(t, out)
			assert.Contains(t, err.Error(), tc.field)
			assert.False(t, called, "invalid scalar config must fail before request")
		})
	}
}

func TestToolService_TestHTTPTool_RejectsInvalidAuthTypeForRuntime(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{
			name:   "unknown camel case auth type",
			config: `{"url":%q,"method":"GET","authType":"api_key","authToken":"secret"}`,
		},
		{
			name:   "unknown snake case auth type",
			config: `{"url":%q,"method":"GET","auth_type":"oauth","auth_token":"secret"}`,
		},
		{
			name:   "auth type with whitespace",
			config: `{"url":%q,"method":"GET","authType":" bearer ","authToken":"secret"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				_, _ = w.Write([]byte("ok"))
			}))
			defer srv.Close()

			repo := newMockRepo()
			toolID := uuid.New()
			repo.data[toolID] = tool.Tool{
				ID:     toolID,
				Name:   "HTTP Auth Type Test",
				Type:   tool.ToolTypeHTTP,
				Config: json.RawMessage(fmt.Sprintf(tc.config, srv.URL)),
			}
			svc := newHTTPRuntimeTestService(repo)

			out, err := svc.TestTool(context.Background(), toolID, nil)

			require.Error(t, err)
			assert.Empty(t, out)
			assert.Contains(t, err.Error(), "auth")
			assert.False(t, called, "invalid auth type must fail before request")
		})
	}
}

func TestToolService_TestHTTPTool_RejectsOversizedRuntimeConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  func(serverURL string) json.RawMessage
		message string
	}{
		{
			name: "url exceeds 2048 characters",
			config: func(serverURL string) json.RawMessage {
				return json.RawMessage(fmt.Sprintf(`{"url":%q,"method":"GET"}`, serverURL+"/"+strings.Repeat("a", 2049)))
			},
			message: "url exceeds maximum length",
		},
		{
			name: "bodyTemplate exceeds 32000 characters",
			config: func(serverURL string) json.RawMessage {
				return json.RawMessage(fmt.Sprintf(`{"url":%q,"method":"POST","bodyTemplate":%q}`, serverURL+"/body", strings.Repeat("x", 32001)))
			},
			message: "bodyTemplate exceeds maximum length",
		},
		{
			name: "headers exceeds 50 entries",
			config: func(serverURL string) json.RawMessage {
				return oversizedHeadersConfig(t, serverURL+"/headers")
			},
			message: "headers exceeds maximum",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				_, _ = w.Write([]byte("ok"))
			}))
			defer srv.Close()

			repo := newMockRepo()
			toolID := uuid.New()
			repo.data[toolID] = tool.Tool{
				ID:     toolID,
				Name:   "HTTP Oversized Runtime Test",
				Type:   tool.ToolTypeHTTP,
				Config: tc.config(srv.URL),
			}
			svc := newHTTPRuntimeTestService(repo)

			out, err := svc.TestTool(context.Background(), toolID, nil)

			require.Error(t, err)
			assert.Empty(t, out)
			assert.Contains(t, err.Error(), tc.message)
			assert.False(t, called, "oversized config must fail before request")
		})
	}
}

func TestToolService_TestHTTPTool_RejectsSensitiveBodyTemplatePlaceholders(t *testing.T) {
	tests := []struct {
		name   string
		config func(serverURL string) json.RawMessage
	}{
		{
			name: "body template auth token",
			config: func(serverURL string) json.RawMessage {
				return json.RawMessage(fmt.Sprintf(`{"url":%q,"method":"POST","bodyTemplate":"{\"token\":\"{{auth_token}}\"}"}`, serverURL+"/body"))
			},
		},
		{
			name: "body alias args api key",
			config: func(serverURL string) json.RawMessage {
				return json.RawMessage(fmt.Sprintf(`{"url":%q,"method":"POST","body":"{\"key\":\"{{args.api_key}}\"}"}`, serverURL+"/body"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				_, _ = w.Write([]byte("ok"))
			}))
			defer srv.Close()

			repo := newMockRepo()
			toolID := uuid.New()
			repo.data[toolID] = tool.Tool{
				ID:     toolID,
				Name:   "HTTP Sensitive Body Runtime Test",
				Type:   tool.ToolTypeHTTP,
				Config: tc.config(srv.URL),
			}
			svc := newHTTPRuntimeTestService(repo)

			out, err := svc.TestTool(context.Background(), toolID, nil)

			require.Error(t, err)
			assert.Empty(t, out)
			assert.Contains(t, err.Error(), "body")
			assert.Contains(t, err.Error(), "sensitive")
			assert.False(t, called, "sensitive body placeholders must fail before request")
		})
	}
}

func TestToolService_TestHTTPTool_RejectsRuntimeIncompatibleTimeoutAliases(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{
			name:   "fractional timeoutSeconds",
			config: `{"url":%q,"method":"GET","timeoutSeconds":1.5}`,
		},
		{
			name:   "snake case timeout too large",
			config: `{"url":%q,"method":"GET","timeout_seconds":99999}`,
		},
		{
			name:   "timeoutMs string",
			config: `{"url":%q,"method":"GET","timeoutMs":"1000"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				_, _ = w.Write([]byte("ok"))
			}))
			defer srv.Close()

			repo := newMockRepo()
			toolID := uuid.New()
			repo.data[toolID] = tool.Tool{
				ID:     toolID,
				Name:   "HTTP Timeout Alias Test",
				Type:   tool.ToolTypeHTTP,
				Config: json.RawMessage(fmt.Sprintf(tc.config, srv.URL)),
			}
			svc := newHTTPRuntimeTestService(repo)

			out, err := svc.TestTool(context.Background(), toolID, nil)

			require.Error(t, err)
			assert.Empty(t, out)
			assert.Contains(t, err.Error(), "timeout")
			assert.False(t, called, "invalid timeout config must fail before request")
		})
	}
}

func TestToolService_TestHTTPTool_RendersRuntimeBodyTemplate(t *testing.T) {
	var gotMethod, gotBody, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		if gotBody != `{"name":"Ada","city":"Sao Paulo"}` {
			http.Error(w, "missing rendered body", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Body Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "POST",
			"bodyTemplate": "{\"name\":\"{{args.name}}\",\"city\":\"{{city}}\"}"
		}`, srv.URL+"/users")),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, map[string]any{
		"name": "Ada",
		"city": "Sao Paulo",
	})

	require.NoError(t, err)
	assert.Contains(t, out, "Status: 200")
	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "application/json", gotContentType)
	assert.Equal(t, `{"name":"Ada","city":"Sao Paulo"}`, gotBody)
}

func TestToolService_TestHTTPTool_NormalizesRuntimeMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if gotMethod != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Method Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "post",
			"bodyTemplate": "{\"name\":\"Ada\"}"
		}`, srv.URL+"/users")),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "Status: 200")
	assert.Equal(t, http.MethodPost, gotMethod)
}

func TestToolService_TestHTTPTool_TrimsRuntimeMethod(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		if gotMethod != http.MethodPost {
			http.Error(w, "expected POST", http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Trimmed Method Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": " post ",
			"bodyTemplate": "{\"name\":\"Ada\"}"
		}`, srv.URL+"/users")),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "Status: 200")
	assert.Equal(t, http.MethodPost, gotMethod)
}

func TestToolService_TestHTTPTool_AppendsUnusedInputAsQuery(t *testing.T) {
	var gotCity, gotUnits string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCity = r.URL.Query().Get("city")
		gotUnits = r.URL.Query().Get("units")
		if gotCity != "Sao Paulo" || gotUnits != "metric" {
			http.Error(w, "missing query params", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Query Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET"
		}`, srv.URL+"/weather")),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, map[string]any{
		"city":  "Sao Paulo",
		"units": "metric",
	})

	require.NoError(t, err)
	assert.Contains(t, out, "Status: 200")
	assert.Equal(t, "Sao Paulo", gotCity)
	assert.Equal(t, "metric", gotUnits)
}

func TestToolService_TestHTTPTool_UsesBaseURLForRelativeRuntimeURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if gotPath != "/v1/status" {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Base URL Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"baseUrl": %q,
			"url": "/v1/status",
			"method": "GET"
		}`, srv.URL+"/")),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "Status: 200")
	assert.Equal(t, "/v1/status", gotPath)
}

func TestToolService_TestHTTPTool_UsesBackendBaseURLForRelativeRuntimeURL(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if gotPath != "/api/agents" {
			http.Error(w, "unexpected path", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	t.Setenv("BACKEND_BASE_URL", srv.URL+"/")

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Backend Base URL Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(`{
			"url": "/api/agents",
			"method": "GET"
		}`),
	}
	svc := tool.NewService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "Status: 200")
	assert.Equal(t, "/api/agents", gotPath)
}

func TestToolService_TestHTTPTool_ForwardsCallerTokenWhenConfigured(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if gotAuth != "Bearer caller-token-123" {
			http.Error(w, "missing caller token", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Caller Token Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET",
			"useCallerToken": true
		}`, srv.URL+"/secure")),
	}
	svc := newHTTPRuntimeTestService(repo)

	ctx := tenant.NewContextWithToken(context.Background(), "test", "caller-token-123")
	out, err := svc.TestTool(ctx, toolID, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "Status: 200")
	assert.Equal(t, "Bearer caller-token-123", gotAuth)
}

func TestToolService_TestHTTPTool_ForwardsCallerTokenWhenSnakeCaseConfigured(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if gotAuth != "Bearer caller-token-123" {
			http.Error(w, "missing caller token", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Snake Case Caller Token Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET",
			"use_caller_token": true
		}`, srv.URL+"/secure")),
	}
	svc := newHTTPRuntimeTestService(repo)

	ctx := tenant.NewContextWithToken(context.Background(), "test", "caller-token-123")
	out, err := svc.TestTool(ctx, toolID, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "Status: 200")
	assert.Equal(t, "Bearer caller-token-123", gotAuth)
}

func TestToolService_TestHTTPTool_HonorsRuntimeTimeoutSeconds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Timeout Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET",
			"timeoutSeconds": 1
		}`, srv.URL+"/slow")),
	}
	svc := newHTTPRuntimeTestService(repo)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	_, err := svc.TestTool(ctx, toolID, nil)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP request failed")
	assert.True(t, elapsed < 2500*time.Millisecond, "configured timeout should fire before parent context, elapsed=%s", elapsed)
}

func TestToolService_TestHTTPTool_ReturnsRuntimeErrorForHTTPFailureStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Failure Status Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET"
		}`, srv.URL+"/fail")),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "HTTP 503 Service Unavailable")
	assert.Contains(t, err.Error(), "upstream unavailable")
}

func TestToolService_TestHTTPTool_RedactsConfiguredCredentialsInUpstreamError(t *testing.T) {
	const authToken = "tool-test-secret-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+authToken, r.Header.Get("Authorization"))
		http.Error(w, "upstream echoed Authorization: "+r.Header.Get("Authorization"), http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Credential Redaction Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET",
			"authType": "bearer",
			"authToken": %q
		}`, srv.URL+"/fail", authToken)),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "HTTP 503 Service Unavailable")
	assert.NotContains(t, err.Error(), authToken)
}

func TestToolService_TestHTTPTool_RedactsConfiguredCredentialsInUpstreamSuccess(t *testing.T) {
	const authToken = "tool-test-success-secret"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer "+authToken, r.Header.Get("Authorization"))
		_, _ = w.Write([]byte("safe-success-value; upstream echoed Authorization: " + r.Header.Get("Authorization")))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Successful Credential Redaction Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET",
			"authType": "bearer",
			"authToken": %q
		}`, srv.URL+"/success", authToken)),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "Status: 200")
	assert.Contains(t, out, "safe-success-value")
	assert.NotContains(t, out, authToken)
}

func TestToolService_TestHTTPTool_RedactsSensitiveJSONFromUpstreamSuccess(t *testing.T) {
	const upstreamSecret = "upstream-success-api-key-secret"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"details":{"apiKey":"` + upstreamSecret + `","safe":"visible-success"}}`))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Successful Structured Redaction Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET"
		}`, srv.URL+"/success")),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.NoError(t, err)
	assert.Contains(t, out, `"safe":"visible-success"`)
	assert.Contains(t, out, `"apiKey":"***"`)
	assert.NotContains(t, out, upstreamSecret)
}

func TestToolService_TestHTTPTool_RedactsConfiguredURLUserInfoInUpstreamSuccess(t *testing.T) {
	const username = "tool-response-url-user"
	const password = "tool-response-url-password"
	const apiKey = "tool-response-url-query-secret"

	var configuredURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("safe-success-value; upstream echoed configured URL: " + configuredURL))
	}))
	defer srv.Close()
	configuredURL = strings.Replace(srv.URL, "://", "://"+username+":"+password+"@", 1) + "/success?api_key=" + apiKey

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP URL User Info Redaction Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET"
		}`, configuredURL)),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.NoError(t, err)
	assert.Contains(t, out, "safe-success-value")
	assert.NotContains(t, out, username)
	assert.NotContains(t, out, password)
	assert.NotContains(t, out, apiKey)
}

func TestToolService_TestHTTPTool_RedactsSensitiveJSONFromUpstreamError(t *testing.T) {
	const upstreamSecret = "upstream-api-key-secret"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream failure","details":{"apiKey":"` + upstreamSecret + `","safe":"visible"}}`))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Structured Error Redaction Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET"
		}`, srv.URL+"/fail")),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "HTTP 502 Bad Gateway")
	assert.Contains(t, err.Error(), `"safe":"visible"`)
	assert.Contains(t, err.Error(), `"apiKey":"***"`)
	assert.NotContains(t, err.Error(), upstreamSecret)
}

func TestToolService_TestHTTPTool_RedactsSensitiveURLInTransportError(t *testing.T) {
	const apiKey = "transport-query-api-key-secret"
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	endpoint := srv.URL + "?api_key=" + apiKey
	srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Transport Error Redaction Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET"
		}`, endpoint)),
	}
	svc := newHTTPRuntimeTestService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "HTTP request failed")
	assert.NotContains(t, err.Error(), apiKey)
}

func TestToolService_TestHTTPTool_BlocksRedirectToSSRFURL(t *testing.T) {
	redirectedTargetReached := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirectedTargetReached = true
		_, _ = w.Write([]byte("redirect target must not be reached"))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/metadata", http.StatusFound)
	}))
	defer redirector.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Redirect SSRF Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET"
		}`, redirector.URL+"/redirect")),
	}
	svc := tool.NewService(repo).WithHTTPURLValidator(func(rawURL string) error {
		if strings.HasPrefix(rawURL, redirector.URL) {
			return nil
		}
		return ssrf.ValidateURL(rawURL)
	})

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "blocked redirect URL")
	assert.False(t, redirectedTargetReached, "redirected SSRF target must not be reached")
}

func TestToolService_TestHTTPTool_BlocksRuntimeSSRFURL(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		_, _ = w.Write([]byte("should not be reached"))
	}))
	defer srv.Close()

	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP SSRF Runtime Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(fmt.Sprintf(`{
			"url": %q,
			"method": "GET"
		}`, srv.URL+"/metadata")),
	}
	svc := tool.NewService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "blocked URL")
	assert.False(t, called, "blocked URL must not be requested")
}

func TestToolService_TestHTTPTool_RejectsUnsupportedURLScheme(t *testing.T) {
	repo := newMockRepo()
	toolID := uuid.New()
	repo.data[toolID] = tool.Tool{
		ID:   toolID,
		Name: "HTTP Unsupported Scheme Runtime Test",
		Type: tool.ToolTypeHTTP,
		Config: json.RawMessage(`{
			"url": "ftp://[2606:4700:4700::1111]/resource",
			"method": "GET"
		}`),
	}
	svc := tool.NewService(repo)

	out, err := svc.TestTool(context.Background(), toolID, nil)

	require.Error(t, err)
	assert.Empty(t, out)
	assert.Contains(t, err.Error(), "blocked URL")
	assert.Contains(t, err.Error(), "scheme")
}

func TestToolService_TestHTTPTool_RejectsURLPathTraversalBeforeTrustedBackendRequest(t *testing.T) {
	tests := []struct {
		name   string
		config string
		inputs map[string]any
	}{
		{
			name:   "literal traversal in relative backend URL",
			config: `{"url":"/api/files/../admin","method":"GET"}`,
		},
		{
			name:   "encoded slash traversal rendered from placeholder",
			config: `{"urlTemplate":"/api/files/{{input.path}}","method":"GET"}`,
			inputs: map[string]any{"path": "../secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				_, _ = w.Write([]byte("should not be reached"))
			}))
			defer srv.Close()

			repo := newMockRepo()
			toolID := uuid.New()
			repo.data[toolID] = tool.Tool{
				ID:     toolID,
				Name:   "HTTP Path Traversal Runtime Test",
				Type:   tool.ToolTypeHTTP,
				Config: json.RawMessage(tt.config),
			}
			svc := tool.NewService(repo).WithHTTPBackendBaseURL(srv.URL)

			out, err := svc.TestTool(context.Background(), toolID, tt.inputs)

			require.Error(t, err)
			assert.Empty(t, out)
			assert.Contains(t, err.Error(), "path traversal")
			assert.False(t, called, "path traversal must fail before trusted backend request")
		})
	}
}

func newHTTPRuntimeTestService(repo *mockToolRepo) *tool.Service {
	return tool.NewService(repo).WithHTTPURLValidator(func(string) error { return nil })
}

// --- TR-01-TASK-30: normalizar dataSourceId -> datasource_id (P-C249-1) ---

// configKeys returns the top-level keys of a tool's Config (which is type any).
func configKeys(t *testing.T, cfg any) map[string]bool {
	t.Helper()
	b, err := json.Marshal(cfg)
	require.NoError(t, err)
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(b, &m))
	keys := make(map[string]bool, len(m))
	for k := range m {
		keys[k] = true
	}
	return keys
}

func configString(t *testing.T, cfg any, key string) string {
	t.Helper()
	b, err := json.Marshal(cfg)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(b, &m))
	v, _ := m[key].(string)
	return v
}

func oversizedHeadersConfig(t *testing.T, targetURL string) json.RawMessage {
	t.Helper()
	headers := make(map[string]string, 51)
	for i := 0; i < 51; i++ {
		headers[fmt.Sprintf("X-Test-%02d", i)] = "value"
	}
	cfg := map[string]any{
		"url":     targetURL,
		"method":  "GET",
		"headers": headers,
	}
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	return data
}

func TestNormalizeDataSourceID_CamelCaseConvertedToSnakeCase(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "SQL Query",
		Type:   tool.ToolTypeSQL,
		Config: json.RawMessage(`{"dataSourceId":"00000000-0000-0000-0000-000000000001","query":"SELECT 1"}`),
	})
	require.NoError(t, err)

	keys := configKeys(t, created.Config)
	assert.True(t, keys["datasource_id"], "snake_case key should be present")
	assert.False(t, keys["dataSourceId"], "camelCase key should be removed")
}

func TestNormalizeDataSourceID_AlreadySnakeCase_Unchanged(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "SQL Query 2",
		Type:   tool.ToolTypeSQL,
		Config: json.RawMessage(`{"datasource_id":"00000000-0000-0000-0000-000000000002","query":"SELECT 1"}`),
	})
	require.NoError(t, err)

	keys := configKeys(t, created.Config)
	assert.True(t, keys["datasource_id"])
}

func TestNormalizeDataSourceID_NonSQLToolNotTouched(t *testing.T) {
	svc := tool.NewService(newMockRepo())
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:   "Custom Tool",
		Type:   tool.ToolTypeCustom,
		Config: json.RawMessage(`{"datasourceId":"abc-123"}`),
	})
	require.NoError(t, err)

	// Non-SQL tools should NOT be normalised — config returned as-is.
	keys := configKeys(t, created.Config)
	assert.True(t, keys["datasourceId"], "non-SQL tool config should be untouched")
}

func TestToolService_SQLRejectsConflictingDatasourceIDAliases(t *testing.T) {
	const snakeID = "00000000-0000-0000-0000-000000000001"
	const camelID = "00000000-0000-0000-0000-000000000002"

	t.Run("create does not persist a conflict", func(t *testing.T) {
		repo := newMockRepo()
		svc := tool.NewService(repo)

		_, err := svc.Create(context.Background(), tool.CreateRequest{
			Name: "Conflicting SQL datasource",
			Type: tool.ToolTypeSQL,
			Config: json.RawMessage(`{
				"datasource_id":"` + snakeID + `",
				"dataSourceId":"` + camelID + `",
				"query":"SELECT 1"
			}`),
		})

		require.ErrorIs(t, err, tool.ErrValidation)
		assert.Empty(t, repo.data)
	})

	t.Run("update preserves the existing datasource", func(t *testing.T) {
		repo := newMockRepo()
		existing := tool.Tool{
			ID:     uuid.New(),
			Name:   "Existing SQL datasource",
			Type:   tool.ToolTypeSQL,
			Config: json.RawMessage(`{"datasource_id":"` + snakeID + `","query":"SELECT 1"}`),
		}
		repo.data[existing.ID] = existing
		svc := tool.NewService(repo)

		_, err := svc.Update(context.Background(), existing.ID, tool.UpdateRequest{
			Config: json.RawMessage(`{
				"datasource_id":"` + snakeID + `",
				"dataSourceId":"` + camelID + `",
				"query":"SELECT 1"
			}`),
		})

		require.ErrorIs(t, err, tool.ErrValidation)
		assert.Equal(t, string(existing.Config), string(repo.data[existing.ID].Config))
	})
}

func TestToolService_SQLAcceptsEquivalentDatasourceIDAliases(t *testing.T) {
	repo := newMockRepo()
	svc := tool.NewService(repo)
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name: "Equivalent SQL datasource",
		Type: tool.ToolTypeSQL,
		Config: json.RawMessage(`{
			"datasource_id":"AAAAAAAA-0000-0000-0000-000000000001",
			"dataSourceId":"aaaaaaaa-0000-0000-0000-000000000001",
			"query":"SELECT 1"
		}`),
	})
	require.NoError(t, err)
	keys := configKeys(t, created.Config)
	assert.True(t, keys["datasource_id"])
	assert.False(t, keys["dataSourceId"])
}

// --- TR-QA-LOOP: skillId auto-bind on create (Cenário A) ---

func TestToolService_Create_WithSkillID_AutoBinds(t *testing.T) {
	repo := newMockRepo()
	svc := tool.NewService(repo)

	skillID := uuid.New()
	created, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:    "auto-bind-tool",
		Type:    tool.ToolTypeCustom,
		SkillID: &skillID,
	})
	require.NoError(t, err)
	assert.NotEqual(t, uuid.Nil, created.ID)

	// Verify the tool was auto-bound to the skill.
	bindings, _, err := repo.ListBySkill(context.Background(), skillID)
	require.NoError(t, err)
	require.Len(t, bindings, 1, "tool should have been auto-bound to skill")
	assert.Equal(t, created.ID, bindings[0].ToolID)
}

func TestToolService_Create_WithNilSkillID_DoesNotBind(t *testing.T) {
	repo := newMockRepo()
	svc := tool.NewService(repo)

	skillID := uuid.New()
	_, err := svc.Create(context.Background(), tool.CreateRequest{
		Name:    "no-auto-bind-tool",
		Type:    tool.ToolTypeCustom,
		SkillID: nil,
	})
	require.NoError(t, err)

	// No binding should have been created.
	bindings, _, err := repo.ListBySkill(context.Background(), skillID)
	require.NoError(t, err)
	assert.Empty(t, bindings, "no binding should be created when SkillID is nil")
}
