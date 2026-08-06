package channel_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/channel"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

type stubTenantLister struct{ ids []string }

func (l stubTenantLister) ListAllIDs(context.Context) ([]string, error) { return l.ids, nil }

func buildTestRouter(t *testing.T) (*chi.Mux, *memRepo, *stubAdapter) {
	return buildTestRouterWithRoles(t, "admin")
}

func buildTestRouterWithRoles(t *testing.T, roles ...string) (*chi.Mux, *memRepo, *stubAdapter) {
	t.Helper()
	repo := newMemRepo()
	reg := channel.NewRegistry()
	adapter := &stubAdapter{
		parsedMsg: channel.InboundMessage{Text: "hi", ReplyTo: "C1"},
	}
	reg.Register(channel.ChannelTypeSlack, adapter)
	disp := &stubDispatcher{reply: "response text"}
	svc := channel.NewService(repo, reg).
		WithDispatcher(disp).
		WithTenantLister(stubTenantLister{ids: []string{"test-tenant"}})
	h := channel.NewHandler(svc)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := middleware.ContextWithRoles(r.Context(), roles...)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	h.RegisterRoutes(r)
	h.RegisterPublicRoutes(r)
	return r, repo, adapter
}

func TestChannelHandler_AdministrativeRoutesRequireAdminRole(t *testing.T) {
	r, _, _ := buildTestRouterWithRoles(t, "user")
	channelID := uuid.NewString()
	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/channels/"},
		{name: "create", method: http.MethodPost, path: "/api/channels/", body: `{}`},
		{name: "get", method: http.MethodGet, path: "/api/channels/" + channelID},
		{name: "put", method: http.MethodPut, path: "/api/channels/" + channelID, body: `{}`},
		{name: "patch", method: http.MethodPatch, path: "/api/channels/" + channelID, body: `{}`},
		{name: "delete", method: http.MethodDelete, path: "/api/channels/" + channelID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			if tc.body != "" {
				req.Header.Set("Content-Type", "application/json")
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)
			assert.Contains(t, w.Body.String(), "missing required role")
		})
	}
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

func TestChannelHandlerRejectsTrailingJSONWithoutServiceEffects(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		r, repo, _ := buildTestRouter(t)
		req := httptest.NewRequest(http.MethodPost, "/api/channels/", bytes.NewBufferString(`{"name":"first","type":"SLACK","agentId":"`+uuid.NewString()+`"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Empty(t, repo.channels)
	})

	t.Run("update", func(t *testing.T) {
		r, repo, _ := buildTestRouter(t)
		ch := seedChannel(t, repo)
		req := httptest.NewRequest(http.MethodPatch, "/api/channels/"+ch.ID.String(), bytes.NewBufferString(`{"name":"changed"}{"name":"ignored"}`))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
		assert.Equal(t, "test-channel", repo.channels[ch.ID].Name)
	})
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
	svc := channel.NewService(repo, reg).WithTenantLister(stubTenantLister{ids: []string{"test-tenant"}})
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

func TestHandlerInbound_CustomJSONRejectsDuplicateKeysBeforeDispatch(t *testing.T) {
	repo := newMemRepo()
	registry := channel.NewRegistry()
	registry.Register(channel.ChannelTypeCustom, &channel.CustomAdapter{})
	dispatcher := &stubDispatcher{reply: "must not be sent"}
	svc := channel.NewService(repo, registry).WithDispatcher(dispatcher)
	handler := channel.NewHandler(svc)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)

	created, err := repo.Create(context.Background(), channel.Channel{
		Name:    "custom",
		Type:    channel.ChannelTypeCustom,
		AgentID: uuid.New(),
		Enabled: true,
		Token:   "custom-token",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/channels/inbound/"+created.Token,
		bytes.NewBufferString(`{"text":"first","text":"second"}`),
	)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, dispatcher.calls)
}

func TestHandlerInbound_SlackVerifiesRawBodyBeforeRejectingAmbiguousJSON(t *testing.T) {
	const signingSecret = "test-signing-secret"
	const body = `{"type":"url_verification","\u0074ype":"event_callback"}`

	repo := newMemRepo()
	registry := channel.NewRegistry()
	registry.Register(channel.ChannelTypeSlack, &channel.SlackAdapter{})
	dispatcher := &stubDispatcher{reply: "must not be sent"}
	svc := channel.NewService(repo, registry).WithDispatcher(dispatcher)
	handler := channel.NewHandler(svc)
	router := chi.NewRouter()
	handler.RegisterPublicRoutes(router)

	created, err := repo.Create(context.Background(), channel.Channel{
		Name:    "slack",
		Type:    channel.ChannelTypeSlack,
		AgentID: uuid.New(),
		Enabled: true,
		Token:   "slack-token",
		Config:  json.RawMessage(`{"signingSecret":"` + signingSecret + `"}`),
	})
	require.NoError(t, err)

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(signingSecret))
	_, err = mac.Write([]byte("v0:" + timestamp + ":" + body))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/channels/inbound/"+created.Token, bytes.NewBufferString(body))
	req.Header.Set("X-Slack-Request-Timestamp", timestamp)
	req.Header.Set("X-Slack-Signature", "v0="+hex.EncodeToString(mac.Sum(nil)))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Zero(t, dispatcher.calls)
}

func TestHandlerInbound_urlVerificationChallenge(t *testing.T) {
	repo := newMemRepo()
	reg := channel.NewRegistry()
	adapter := &stubAdapter{
		parsedMsg: channel.InboundMessage{Challenge: "challenge_xyz"},
	}
	reg.Register(channel.ChannelTypeSlack, adapter)
	disp := &stubDispatcher{}
	svc := channel.NewService(repo, reg).
		WithDispatcher(disp).
		WithTenantLister(stubTenantLister{ids: []string{"test-tenant"}})
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
