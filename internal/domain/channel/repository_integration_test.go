//go:build integration

package channel_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/channel"
	"github.com/AgentHub-Studio/agenthub-api/internal/middleware"
	"github.com/AgentHub-Studio/agenthub-api/internal/tenant"
	"github.com/AgentHub-Studio/agenthub-go-commons/testutil"
)

const channelIntegrationTenant = "channelredactiontest"

func setupChannelTenantSchema(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	pool := testutil.NewPostgresContainer(t)
	ctx := context.Background()

	schema := "ah_" + channelIntegrationTenant
	testutil.MustExec(t, pool, "CREATE SCHEMA IF NOT EXISTS "+schema)

	dir, err := filepath.Abs("../../../migrations/schemas")
	require.NoError(t, err)
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var ups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".up.sql") {
			ups = append(ups, entry.Name())
		}
	}
	sort.Strings(ups)
	require.NotEmpty(t, ups)

	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, "SET search_path TO "+schema)
	require.NoError(t, err)
	for _, name := range ups {
		sqlBytes, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err, "read migration %s", name)
		_, err = conn.Exec(ctx, string(sqlBytes))
		require.NoError(t, err, "apply migration %s", name)
	}

	return pool, tenant.NewContext(ctx, channelIntegrationTenant)
}

func TestIntegration_ChannelResponseMasksNestedSensitiveConfigKeys(t *testing.T) {
	pool, ctx := setupChannelTenantSchema(t)
	repo := channel.NewRepository(pool)
	svc := channel.NewService(repo, channel.NewRegistry())

	created, err := repo.Create(ctx, channel.Channel{
		Name:    "slack-nested-redaction",
		Type:    channel.ChannelTypeSlack,
		AgentID: uuid.New(),
		Config: json.RawMessage(`{
			"workspace":"acme",
			"oauth":{
				"clientSecret":"nested-client-secret",
				"accessToken":"nested-access-token",
				"safe":"kept"
			},
			"events":[
				{"name":"message","signingSecret":"nested-signing-secret"}
			]
		}`),
		Token:   "inbound-token-secret",
		Enabled: true,
	})
	require.NoError(t, err)

	persisted, err := repo.GetByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Contains(t, string(persisted.Config), "nested-client-secret")
	assert.Contains(t, string(persisted.Config), "nested-signing-secret")

	resp, err := svc.GetByID(ctx, created.ID)
	require.NoError(t, err)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	body := string(data)

	assert.NotContains(t, body, "inbound-token-secret")
	assert.NotContains(t, body, "nested-client-secret")
	assert.NotContains(t, body, "nested-access-token")
	assert.NotContains(t, body, "nested-signing-secret")
	assert.Contains(t, body, `"clientSecret":"***"`)
	assert.Contains(t, body, `"accessToken":"***"`)
	assert.Contains(t, body, `"signingSecret":"***"`)
	assert.Contains(t, body, "kept")
	assert.Contains(t, body, "message")
}

type channelIntegrationTenantLister struct {
	ids []string
}

func (l channelIntegrationTenantLister) ListAllIDs(_ context.Context) ([]string, error) {
	return l.ids, nil
}

func TestIntegration_ChannelHTTPCRUDAndPublicInboundTokenLookup(t *testing.T) {
	pool, tenantCtx := setupChannelTenantSchema(t)
	repo := channel.NewRepository(pool)
	registry := channel.NewRegistry()
	registry.Register(channel.ChannelTypeCustom, &channel.CustomAdapter{})
	registry.Register(channel.ChannelTypeSlack, &channel.SlackAdapter{})

	svc := channel.NewService(repo, registry).
		WithTenantLister(channelIntegrationTenantLister{ids: []string{channelIntegrationTenant}})
	handler := channel.NewHandler(svc)
	router := chi.NewRouter()
	handler.RegisterRoutes(router)
	handler.RegisterPublicRoutes(router)

	createBody := []byte(`{
		"name":"custom-http-integration",
		"type":"CUSTOM",
		"agentId":"` + uuid.NewString() + `",
		"config":{"secret":"must-not-leak","endpoint":"https://example.test/inbound"}
	}`)
	adminCtx := middleware.ContextWithRoles(tenantCtx, "admin")
	createReq := httptest.NewRequest(http.MethodPost, "/api/channels/", bytes.NewReader(createBody)).WithContext(adminCtx)
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)
	require.Equal(t, http.StatusCreated, createRes.Code, createRes.Body.String())

	var created channel.ChannelTokenResponse
	require.NoError(t, json.Unmarshal(createRes.Body.Bytes(), &created))
	require.NotEmpty(t, created.Token)
	require.Len(t, created.Token, 64)
	require.Equal(t, "custom-http-integration", created.Name)

	persisted, err := repo.GetByID(tenantCtx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.Token, persisted.Token)
	assert.JSONEq(t, `{"secret":"must-not-leak","endpoint":"https://example.test/inbound"}`, string(persisted.Config))

	getReq := httptest.NewRequest(http.MethodGet, "/api/channels/"+created.ID.String(), nil).WithContext(adminCtx)
	getRes := httptest.NewRecorder()
	router.ServeHTTP(getRes, getReq)
	require.Equal(t, http.StatusOK, getRes.Code, getRes.Body.String())
	assert.NotContains(t, getRes.Body.String(), created.Token)
	assert.NotContains(t, getRes.Body.String(), "must-not-leak")
	assert.Contains(t, getRes.Body.String(), `"secret":"***"`)

	// The public endpoint has no tenant context. It must find the tenant-bound
	// channel by token, parse the real CUSTOM payload, and return successfully.
	inboundReq := httptest.NewRequest(
		http.MethodPost,
		"/api/channels/inbound/"+created.Token,
		bytes.NewBufferString(`{"text":"","sender":"external-user"}`),
	)
	inboundReq.Header.Set("Content-Type", "application/json")
	inboundRes := httptest.NewRecorder()
	router.ServeHTTP(inboundRes, inboundReq)
	require.Equal(t, http.StatusOK, inboundRes.Code, inboundRes.Body.String())

	// Ambiguous JSON must be rejected at the public boundary before it can be
	// interpreted as a CUSTOM message or dispatched to an agent.
	ambiguousInboundReq := httptest.NewRequest(
		http.MethodPost,
		"/api/channels/inbound/"+created.Token,
		bytes.NewBufferString(`{"text":"first","text":"second","sender":"external-user"}`),
	)
	ambiguousInboundReq.Header.Set("Content-Type", "application/json")
	ambiguousInboundRes := httptest.NewRecorder()
	router.ServeHTTP(ambiguousInboundRes, ambiguousInboundReq)
	require.Equal(t, http.StatusBadRequest, ambiguousInboundRes.Code, ambiguousInboundRes.Body.String())

	const slackSigningSecret = "integration-signing-secret"
	slackCreateBody := []byte(`{
		"name":"slack-http-integration",
		"type":"SLACK",
		"agentId":"` + uuid.NewString() + `",
		"config":{"signingSecret":"` + slackSigningSecret + `"}
	}`)
	slackCreateReq := httptest.NewRequest(http.MethodPost, "/api/channels/", bytes.NewReader(slackCreateBody)).WithContext(adminCtx)
	slackCreateReq.Header.Set("Content-Type", "application/json")
	slackCreateRes := httptest.NewRecorder()
	router.ServeHTTP(slackCreateRes, slackCreateReq)
	require.Equal(t, http.StatusCreated, slackCreateRes.Code, slackCreateRes.Body.String())

	var slackCreated channel.ChannelTokenResponse
	require.NoError(t, json.Unmarshal(slackCreateRes.Body.Bytes(), &slackCreated))

	const ambiguousSlackBody = `{"type":"url_verification","\u0074ype":"event_callback"}`
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(slackSigningSecret))
	_, err = mac.Write([]byte("v0:" + timestamp + ":" + ambiguousSlackBody))
	require.NoError(t, err)

	slackInboundReq := httptest.NewRequest(
		http.MethodPost,
		"/api/channels/inbound/"+slackCreated.Token,
		bytes.NewBufferString(ambiguousSlackBody),
	)
	slackInboundReq.Header.Set("X-Slack-Request-Timestamp", timestamp)
	slackInboundReq.Header.Set("X-Slack-Signature", "v0="+hex.EncodeToString(mac.Sum(nil)))
	slackInboundRes := httptest.NewRecorder()
	router.ServeHTTP(slackInboundRes, slackInboundReq)
	require.Equal(t, http.StatusBadRequest, slackInboundRes.Code, slackInboundRes.Body.String())
}
