package acl_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/auth/acl"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

func newACLRouter(provider acl.Provider, identity acl.Identity) *chi.Mux {
	r := chi.NewRouter()
	acl.NewHandler(provider, func(*http.Request) acl.Identity {
		return identity
	}).RegisterRoutes(r)
	return r
}

func TestHandlerCreateListDeleteGrant(t *testing.T) {
	provider := acl.NewMemoryProvider()
	r := newACLRouter(provider, acl.Identity{SubjectID: "admin", Roles: []string{"admin"}})

	body := `{
		"subject": {"type": "user", "id": "viewer@test.local"},
		"resource": {"type": "agents", "id": "agent-1"},
		"actions": ["read"]
	}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/acl/grants", strings.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	r.ServeHTTP(createRec, createReq)
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", createRec.Code, http.StatusCreated, createRec.Body.String())
	}

	var created acl.Grant
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.SubjectID != "viewer@test.local" || created.ResourceType != "agents" || created.ResourceID != "agent-1" {
		t.Fatalf("created grant = %+v", created)
	}

	listReq := httptest.NewRequest(http.MethodGet, "/api/acl/grants?resource_type=agents&resource_id=agent-1", nil)
	listRec := httptest.NewRecorder()
	r.ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listRec.Code, http.StatusOK, listRec.Body.String())
	}
	var page pagination.Page[acl.Grant]
	if err := json.Unmarshal(listRec.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Content) != 1 {
		t.Fatalf("len(content) = %d, want 1", len(page.Content))
	}
	if page.Content[0].ID != created.ID {
		t.Fatalf("listed grant id = %s, want %s", page.Content[0].ID, created.ID)
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/acl/grants/"+created.ID.String(), nil)
	deleteRec := httptest.NewRecorder()
	r.ServeHTTP(deleteRec, deleteReq)
	if deleteRec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want %d; body=%s", deleteRec.Code, http.StatusNoContent, deleteRec.Body.String())
	}

	listAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/api/acl/grants?resource_type=agents&resource_id=agent-1", nil)
	listAfterDeleteRec := httptest.NewRecorder()
	r.ServeHTTP(listAfterDeleteRec, listAfterDeleteReq)
	if listAfterDeleteRec.Code != http.StatusOK {
		t.Fatalf("list after delete status = %d, want %d", listAfterDeleteRec.Code, http.StatusOK)
	}
	var pageAfterDelete pagination.Page[acl.Grant]
	if err := json.Unmarshal(listAfterDeleteRec.Body.Bytes(), &pageAfterDelete); err != nil {
		t.Fatal(err)
	}
	if len(pageAfterDelete.Content) != 0 {
		t.Fatalf("len(content) after delete = %d, want 0", len(pageAfterDelete.Content))
	}
}

func TestHandlerRejectsNonAdmin(t *testing.T) {
	provider := acl.NewMemoryProvider()
	r := newACLRouter(provider, acl.Identity{SubjectID: "viewer", Roles: []string{"user"}})

	req := httptest.NewRequest(http.MethodPost, "/api/acl/grants", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestHandlerCreateRejectsTrailingJSONWithoutGrant(t *testing.T) {
	provider := acl.NewMemoryProvider()
	r := newACLRouter(provider, acl.Identity{SubjectID: "admin", Roles: []string{"admin"}})
	body := `{"subject":{"type":"user","id":"viewer@test.local"},"resource":{"type":"agents","id":"agent-1"},"actions":["read"]}{"actions":["execute"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/acl/grants", strings.NewReader(body))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	grants, err := provider.ListGrants(context.Background(), acl.GrantFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(grants) != 0 {
		t.Fatalf("grants = %d, want 0", len(grants))
	}
}
