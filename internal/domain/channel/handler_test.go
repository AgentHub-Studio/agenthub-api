package channel_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/channel"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

func buildTestRouter(t *testing.T) (*chi.Mux, *memRepo, *stubAdapter) {
	t.Helper()
	repo := newMemRepo()
	reg := channel.NewRegistry()
	adapter := &stubAdapter{
		parsedMsg: channel.InboundMessage{Text: "hi", ReplyTo: "C1"},
	}
	reg.Register(channel.ChannelTypeSlack, adapter)
	disp := &stubDispatcher{reply: "response text"}
	svc := channel.NewService(repo, reg).WithDispatcher(disp)
	h := channel.NewHandler(svc)

	r := chi.NewRouter()
	h.RegisterRoutes(r)
	h.RegisterPublicRoutes(r)
	return r, repo, adapter
}

func seedChannel(t *testing.T, repo *memRepo) channel.Channel {
	t.Helper()
	ch := channel.Channel{
		ID:      uuid.New(),
		Name:    "test-channel",
		Type:    channel.ChannelTypeSlack,
		AgentID: uuid.New(),
		Enabled: true,
		Token:   "test-token-" + uuid.New().String(),
	}
	created, err := repo.Create(context.Background(), ch)
	require.NoError(t, err)
	return created
}

func TestHandlerList_empty(t *testing.T) {
	r, _, _ := buildTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/channels", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var page pagination.Page[channel.ChannelResponse]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &page))
	assert.Equal(t, int64(0), page.TotalElements)
}

func TestHandlerCreate_success(t *testing.T) {
	r, _, _ := buildTestRouter(t)
	body := map[string]any{
		"name":    "slack-prod",
		"type":    "SLACK",
		"agentId": uuid.New().String(),
		"config":  map[string]string{"botToken": "xoxb-test"},
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/channels", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	var resp channel.ChannelTokenResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "slack-prod", resp.Name)
	assert.NotEmpty(t, resp.Token)
}

func TestHandlerCreate_invalidBody(t *testing.T) {
	r, _, _ := buildTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/channels", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerGetByID_success(t *testing.T) {
	r, repo, _ := buildTestRouter(t)
	ch := seedChannel(t, repo)

	req := httptest.NewRequest(http.MethodGet, "/api/channels/"+ch.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp channel.ChannelResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, ch.ID, resp.ID)
}

func TestHandlerGetByID_notFound(t *testing.T) {
	r, _, _ := buildTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/channels/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerGetByID_invalidUUID(t *testing.T) {
	r, _, _ := buildTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/api/channels/bad-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandlerUpdate_success(t *testing.T) {
	r, repo, _ := buildTestRouter(t)
	ch := seedChannel(t, repo)

	newName := "updated-name"
	body, _ := json.Marshal(channel.UpdateChannelRequest{Name: &newName})
	req := httptest.NewRequest(http.MethodPut, "/api/channels/"+ch.ID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp channel.ChannelResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "updated-name", resp.Name)
}

func TestHandlerDelete_success(t *testing.T) {
	r, repo, _ := buildTestRouter(t)
	ch := seedChannel(t, repo)

	req := httptest.NewRequest(http.MethodDelete, "/api/channels/"+ch.ID.String(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHandlerInbound_success(t *testing.T) {
	r, repo, adapter := buildTestRouter(t)
	adapter.parsedMsg = channel.InboundMessage{Text: "hello from slack", ReplyTo: "C99"}

	ch := seedChannel(t, repo)

	req := httptest.NewRequest(http.MethodPost, "/api/channels/inbound/"+ch.Token, bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "response text", adapter.sentReply.Text)
}

func TestHandlerInbound_tokenNotFound(t *testing.T) {
	r, _, _ := buildTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/channels/inbound/bad-token", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerInbound_verifyFails(t *testing.T) {
	repo := newMemRepo()
	reg := channel.NewRegistry()
	adapter := &stubAdapter{verifyErr: errors.New("bad sig")}
	reg.Register(channel.ChannelTypeSlack, adapter)
	svc := channel.NewService(repo, reg)
	h := channel.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterPublicRoutes(r)

	// Seed a channel directly in the repo so we have a valid token.
	ch := channel.Channel{Name: "ch", Type: channel.ChannelTypeSlack, AgentID: uuid.New(), Enabled: true, Token: "mytoken"}
	created, _ := repo.Create(context.Background(), ch)

	req := httptest.NewRequest(http.MethodPost, "/api/channels/inbound/"+created.Token, bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHandlerInbound_urlVerificationChallenge(t *testing.T) {
	repo := newMemRepo()
	reg := channel.NewRegistry()
	adapter := &stubAdapter{
		parsedMsg: channel.InboundMessage{Challenge: "challenge_xyz"},
	}
	reg.Register(channel.ChannelTypeSlack, adapter)
	disp := &stubDispatcher{}
	svc := channel.NewService(repo, reg).WithDispatcher(disp)
	h := channel.NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterPublicRoutes(r)

	ch := channel.Channel{Name: "ch", Type: channel.ChannelTypeSlack, AgentID: uuid.New(), Enabled: true, Token: "tok"}
	created, _ := repo.Create(context.Background(), ch)

	body := `{"type":"url_verification","challenge":"challenge_xyz"}`
	req := httptest.NewRequest(http.MethodPost, "/api/channels/inbound/"+created.Token, bytes.NewReader([]byte(body)))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "challenge_xyz", resp["challenge"])
}
