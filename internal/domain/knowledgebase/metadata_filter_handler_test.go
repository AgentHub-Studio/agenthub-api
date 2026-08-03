package knowledgebase_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledge"
	"github.com/AgentHub-Studio/agenthub-api/internal/domain/knowledgebase"
)

type metadataFilterSearchClient struct {
	opts  knowledge.SearchOptions
	calls int
}

func (c *metadataFilterSearchClient) Search(_ context.Context, _ string, opts knowledge.SearchOptions) ([]knowledge.SearchResult, error) {
	c.calls++
	c.opts = opts
	return nil, nil
}

func TestKnowledgeBaseHandler_Search_ForwardsMetadataFilter(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	client := &metadataFilterSearchClient{}
	h := knowledgebase.NewHandler(svc).WithSearchClient(client)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(`{"query":"release notes","metadataFilter":{"field":"customer.tier","op":"gte","value":2}}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, client.calls)
	require.NotNil(t, client.opts.MetadataFilter)
	sql, args := client.opts.MetadataFilter.SQL(0)
	assert.Contains(t, sql, "d.metadata")
	assert.Equal(t, []any{[]string{"customer", "tier"}, "2"}, args)
}

func TestKnowledgeBaseHandler_Search_RejectsInvalidMetadataFilter(t *testing.T) {
	svc := newMockKBSvc()
	kbID := uuid.New()
	svc.kbs[kbID] = knowledgebase.KnowledgeBaseResponse{ID: kbID, Name: "Filtered KB", Status: knowledgebase.StatusActive}
	client := &metadataFilterSearchClient{}
	h := knowledgebase.NewHandler(svc).WithSearchClient(client)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/"+kbID.String()+"/search", bytes.NewBufferString(`{"query":"release notes","metadataFilter":{"field":"customer.profile.region","op":"eq","value":"BR"}}`))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Zero(t, client.calls)
}
