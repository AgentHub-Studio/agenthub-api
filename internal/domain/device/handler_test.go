package device

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

var errUnknownAgent = errors.New("agent not found")
var errDeviceStore = errors.New("device store unavailable")

type stubDeviceService struct {
	listByAgentCalls int
	listByAgentErr   error
	createCalls      int
	updateCalls      int
	heartbeatCalls   int
}

func (s *stubDeviceService) List(context.Context, pagination.PageRequest) (pagination.Page[DeviceResponse], error) {
	return pagination.Page[DeviceResponse]{}, nil
}

func (s *stubDeviceService) GetByID(context.Context, uuid.UUID) (DeviceResponse, error) {
	return DeviceResponse{}, nil
}

func (s *stubDeviceService) Create(context.Context, CreateDeviceRequest) (DeviceResponse, error) {
	s.createCalls++
	return DeviceResponse{}, nil
}

func (s *stubDeviceService) Update(context.Context, uuid.UUID, UpdateDeviceRequest) (DeviceResponse, error) {
	s.updateCalls++
	return DeviceResponse{}, nil
}

func (s *stubDeviceService) Delete(context.Context, uuid.UUID) error { return nil }

func (s *stubDeviceService) Heartbeat(context.Context, uuid.UUID, HeartbeatRequest) error {
	s.heartbeatCalls++
	return nil
}

func (s *stubDeviceService) ListByAgent(context.Context, uuid.UUID) ([]DeviceResponse, error) {
	s.listByAgentCalls++
	return nil, s.listByAgentErr
}

func (s *stubDeviceService) AttachToAgent(context.Context, uuid.UUID, uuid.UUID) error { return nil }

func (s *stubDeviceService) DetachFromAgent(context.Context, uuid.UUID, uuid.UUID) error { return nil }

type stubAgentExister struct{ err error }

func (s stubAgentExister) GetByID(context.Context, uuid.UUID) error { return s.err }

func newDeviceHandlerRouter(svc *stubDeviceService, agent agentExister) *chi.Mux {
	return newDeviceHandlerRouterWithRoles(svc, agent, "admin")
}

func newDeviceHandlerRouterWithRoles(svc *stubDeviceService, agent agentExister, roles ...string) *chi.Mux {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	NewHandler(svc).WithAgentExister(agent).RegisterRoutes(r)
	return r
}

func TestDeviceHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r := newDeviceHandlerRouterWithRoles(&stubDeviceService{}, stubAgentExister{}, "user")
	agentID := uuid.NewString()
	deviceID := uuid.NewString()
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/devices/"},
		{name: "create", method: http.MethodPost, path: "/api/devices/", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/api/devices/" + deviceID},
		{name: "put", method: http.MethodPut, path: "/api/devices/" + deviceID, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/devices/" + deviceID, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/devices/" + deviceID},
		{name: "list by agent", method: http.MethodGet, path: "/api/agents/" + agentID + "/devices/"},
		{name: "attach", method: http.MethodPost, path: "/api/agents/" + agentID + "/devices/" + deviceID},
		{name: "detach", method: http.MethodDelete, path: "/api/agents/" + agentID + "/devices/" + deviceID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			require.Equal(t, http.StatusForbidden, w.Code)
			require.Contains(t, w.Body.String(), "missing required role")
		})
	}
}

func TestDeviceHandler_HeartbeatRequiresMCPClientRuntimeRole(t *testing.T) {
	deviceID := uuid.NewString()

	t.Run("user is refused", func(t *testing.T) {
		r := newDeviceHandlerRouterWithRoles(&stubDeviceService{}, stubAgentExister{}, "user")
		req := httptest.NewRequest(http.MethodPost, "/api/devices/"+deviceID+"/heartbeat", bytes.NewBufferString(`{"status":"ONLINE"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("runtime is accepted", func(t *testing.T) {
		r := newDeviceHandlerRouterWithRoles(&stubDeviceService{}, stubAgentExister{}, "mcp-client-runtime")
		req := httptest.NewRequest(http.MethodPost, "/api/devices/"+deviceID+"/heartbeat", bytes.NewBufferString(`{"status":"ONLINE"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusNoContent, w.Code)
	})
}

func TestDeviceHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		svc := &stubDeviceService{}
		r := newDeviceHandlerRouter(svc, stubAgentExister{})
		req := httptest.NewRequest(http.MethodPost, "/api/devices/", bytes.NewBufferString(`{"name":"first","type":"SENSOR"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		require.Zero(t, svc.createCalls)
	})

	t.Run("update", func(t *testing.T) {
		svc := &stubDeviceService{}
		r := newDeviceHandlerRouter(svc, stubAgentExister{})
		req := httptest.NewRequest(http.MethodPatch, "/api/devices/"+uuid.NewString(), bytes.NewBufferString(`{"name":"changed"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		require.Zero(t, svc.updateCalls)
	})

	t.Run("heartbeat", func(t *testing.T) {
		svc := &stubDeviceService{}
		r := newDeviceHandlerRouterWithRoles(svc, stubAgentExister{}, "mcp-client-runtime")
		req := httptest.NewRequest(http.MethodPost, "/api/devices/"+uuid.NewString()+"/heartbeat", bytes.NewBufferString(`{"status":"ONLINE"}{"status":"OFFLINE"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		require.Zero(t, svc.heartbeatCalls)
	})
}

func TestDeviceHandler_ListByAgent_UnknownParentReturnsNotFound(t *testing.T) {
	svc := &stubDeviceService{}
	r := newDeviceHandlerRouter(svc, stubAgentExister{err: errUnknownAgent})

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+uuid.NewString()+"/devices/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Equal(t, 0, svc.listByAgentCalls)
}

func TestDeviceHandler_ListByAgent_RepositoryFailureReturnsInternalError(t *testing.T) {
	svc := &stubDeviceService{listByAgentErr: errDeviceStore}
	r := newDeviceHandlerRouter(svc, stubAgentExister{})

	req := httptest.NewRequest(http.MethodGet, "/api/agents/"+uuid.NewString()+"/devices/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, 1, svc.listByAgentCalls)
}
