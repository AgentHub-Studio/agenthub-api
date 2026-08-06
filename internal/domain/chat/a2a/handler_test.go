package a2a

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat"
	tenantctx "github.com/AgentHub-Studio/agenthub-api/internal/tenant"
)

type fakeService struct {
	createTenant string
	invokeTenant string
	createErr    error
	invokeErr    error
	cancelled    bool
}

func (s *fakeService) CreateGrant(_ context.Context, ownerTenant string, _ CreateGrantRequest) (GrantResponse, error) {
	s.createTenant = ownerTenant
	if s.createErr != nil {
		return GrantResponse{}, s.createErr
	}
	now := time.Now().UTC().Format(timeLayout)
	return GrantResponse{
		ID:            uuid.NewString(),
		SubjectTenant: "tenant-a",
		AgentID:       uuid.NewString(),
		Actions:       []string{ActionInvoke},
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (s *fakeService) Invoke(_ context.Context, sourceTenant string, _ InvokeRequest) (InvokeResult, error) {
	s.invokeTenant = sourceTenant
	if s.invokeErr != nil {
		return InvokeResult{}, s.invokeErr
	}
	events := make(chan chat.RunEvent, 2)
	events <- chat.RunEvent{Type: "text_delta", Data: json.RawMessage(`{"delta":"hello"}`)}
	events <- chat.RunEvent{Type: "run_complete", Data: json.RawMessage(`{"status":"completed"}`)}
	close(events)
	return InvokeResult{
		SessionID:    uuid.New(),
		TargetTenant: "tenant-b",
		AgentID:      uuid.New(),
		Events:       events,
		Preparation: InvokePreparationTimings{
			GrantCheckMS:    11,
			RateLimitMS:     12,
			AgentLoadMS:     13,
			SessionCreateMS: 14,
			RunPrepareMS:    15,
			TotalMS:         65,
		},
		cancel: func() {
			s.cancelled = true
		},
	}, nil
}

func TestHandlerCreateGrantPassesTenantAndReturnsCreated(t *testing.T) {
	svc := &fakeService{}
	handler := NewHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/grants", bytes.NewBufferString(`{"subjectTenant":"tenant-a","agentId":"`+uuid.NewString()+`","actions":["invoke"]}`))
	req = req.WithContext(tenantctx.NewContext(req.Context(), "tenant-b"))
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "tenant-b", svc.createTenant)
	assert.Contains(t, rec.Body.String(), `"subjectTenant":"tenant-a"`)
}

func TestHandlerInvokeMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code int
	}{
		{name: "validation", err: ErrValidation, code: http.StatusBadRequest},
		{name: "forbidden", err: ErrForbidden, code: http.StatusForbidden},
		{name: "rate limited", err: ErrRateLimited, code: http.StatusTooManyRequests},
		{name: "not found", err: ErrNotFound, code: http.StatusNotFound},
		{name: "unknown", err: errors.New("boom"), code: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeService{invokeErr: tt.err}
			handler := NewHandler(svc)
			req := httptest.NewRequest(http.MethodPost, "/invoke", bytes.NewBufferString(`{"targetTenant":"tenant-b","agentId":"`+uuid.NewString()+`","input":"hello"}`))
			req = req.WithContext(tenantctx.NewContext(req.Context(), "tenant-a"))
			rec := httptest.NewRecorder()

			handler.Routes().ServeHTTP(rec, req)

			assert.Equal(t, tt.code, rec.Code)
			assert.Equal(t, "tenant-a", svc.invokeTenant)
		})
	}
}

func TestHandlerInvokeAlwaysStreamsSSE(t *testing.T) {
	svc := &fakeService{}
	handler := NewHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/invoke", bytes.NewBufferString(`{"targetTenant":"tenant-b","agentId":"`+uuid.NewString()+`","input":"hello"}`))
	req = req.WithContext(tenantctx.NewContext(req.Context(), "tenant-a"))
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/event-stream")
	assert.Contains(t, rec.Body.String(), "event: a2a_started")
	assert.Contains(t, rec.Body.String(), `"targetTenant":"tenant-b"`)
	assert.Contains(t, rec.Body.String(), `"preparation":{"grantCheckMs":11,"rateLimitMs":12,"agentLoadMs":13,"sessionCreateMs":14,"runPrepareMs":15,"totalMs":65}`)
	assert.Contains(t, rec.Body.String(), "event: text_delta")
	assert.Contains(t, rec.Body.String(), `"delta":"hello"`)
	assert.Contains(t, rec.Body.String(), "event: run_complete")
	assert.True(t, svc.cancelled)
}

func TestHandlerRejectsInvalidJSON(t *testing.T) {
	svc := &fakeService{}
	handler := NewHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/invoke", bytes.NewBufferString(`{`))
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, svc.invokeTenant)
}

func TestHandlerRejectsTrailingJSONWithoutInvokingService(t *testing.T) {
	svc := &fakeService{}
	handler := NewHandler(svc)
	requestBody := `{"targetTenant":"tenant-b","agentId":"` + uuid.NewString() + `"}{"targetTenant":"tenant-c"}`
	req := httptest.NewRequest(http.MethodPost, "/invoke", bytes.NewBufferString(requestBody))
	req = req.WithContext(tenantctx.NewContext(req.Context(), "tenant-a"))
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, svc.invokeTenant)
}

func TestHandlerRejectsTrailingJSONWithoutCreatingGrant(t *testing.T) {
	svc := &fakeService{}
	handler := NewHandler(svc)
	requestBody := `{"subjectTenant":"tenant-a","agentId":"` + uuid.NewString() + `","actions":["invoke"]}{"subjectTenant":"tenant-c"}`
	req := httptest.NewRequest(http.MethodPost, "/grants", bytes.NewBufferString(requestBody))
	req = req.WithContext(tenantctx.NewContext(req.Context(), "tenant-b"))
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Empty(t, svc.createTenant)
}

func FuzzDecodeRequestAcceptsOnlyOneJSONValue(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{"targetTenant":"tenant-b","agentId":"` + uuid.NewString() + `","input":"hello"}`),
		[]byte(`{"targetTenant":"tenant-b"}{"targetTenant":"tenant-c"}`),
		[]byte(`{`),
		[]byte(`null`),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		req := httptest.NewRequest(http.MethodPost, "/invoke", bytes.NewReader(raw))
		var payload InvokeRequest
		err := decodeRequest(req, &payload)
		if err == nil && !json.Valid(raw) {
			t.Fatal("accepted request must contain exactly one valid JSON value")
		}
	})
}
