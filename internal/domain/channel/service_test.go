package channel_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/channel"
	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

// --- in-memory repository stub ---

type memRepo struct {
	channels map[uuid.UUID]channel.Channel
	byToken  map[string]uuid.UUID
}

func newMemRepo() *memRepo {
	return &memRepo{
		channels: make(map[uuid.UUID]channel.Channel),
		byToken:  make(map[string]uuid.UUID),
	}
}

func (r *memRepo) List(_ context.Context) ([]channel.Channel, error) {
	out := make([]channel.Channel, 0, len(r.channels))
	for _, ch := range r.channels {
		out = append(out, ch)
	}
	return out, nil
}

func (r *memRepo) GetByID(_ context.Context, id uuid.UUID) (channel.Channel, error) {
	ch, ok := r.channels[id]
	if !ok {
		return channel.Channel{}, channel.ErrNotFound
	}
	return ch, nil
}

func (r *memRepo) GetByToken(_ context.Context, token string) (channel.Channel, error) {
	id, ok := r.byToken[token]
	if !ok {
		return channel.Channel{}, channel.ErrTokenNotFound
	}
	return r.channels[id], nil
}

func (r *memRepo) Create(_ context.Context, ch channel.Channel) (channel.Channel, error) {
	ch.ID = uuid.New()
	ch.CreatedAt = time.Now()
	ch.UpdatedAt = time.Now()
	r.channels[ch.ID] = ch
	r.byToken[ch.Token] = ch.ID
	return ch, nil
}

func (r *memRepo) Update(_ context.Context, ch channel.Channel) (channel.Channel, error) {
	if _, ok := r.channels[ch.ID]; !ok {
		return channel.Channel{}, channel.ErrNotFound
	}
	ch.UpdatedAt = time.Now()
	r.channels[ch.ID] = ch
	r.byToken[ch.Token] = ch.ID
	return ch, nil
}

func (r *memRepo) ExistsByName(_ context.Context, name string) (bool, error) {
	for _, c := range r.channels {
		if c.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (r *memRepo) Delete(_ context.Context, id uuid.UUID) error {
	ch, ok := r.channels[id]
	if !ok {
		return channel.ErrNotFound
	}
	delete(r.byToken, ch.Token)
	delete(r.channels, id)
	return nil
}

// --- adapter stubs ---

type stubAdapter struct {
	verifyErr error
	parsedMsg channel.InboundMessage
	parseErr  error
	sentReply channel.OutboundMessage
	sendErr   error
}

func (a *stubAdapter) VerifyRequest(_ context.Context, _ channel.Channel, _ map[string]string, _ []byte) error {
	return a.verifyErr
}
func (a *stubAdapter) ParseMessage(_ context.Context, _ channel.Channel, _ []byte) (channel.InboundMessage, error) {
	return a.parsedMsg, a.parseErr
}
func (a *stubAdapter) SendReply(_ context.Context, _ channel.Channel, msg channel.OutboundMessage) error {
	a.sentReply = msg
	return a.sendErr
}

// --- dispatcher stub ---

type stubDispatcher struct {
	reply string
	err   error
	calls int
}

func (d *stubDispatcher) Dispatch(_ context.Context, _ uuid.UUID, _ string, _ string) (string, error) {
	d.calls++
	return d.reply, d.err
}

// --- helpers ---

func buildService(t *testing.T) (*channel.Handler, *memRepo, *channel.Registry, *stubAdapter, *stubDispatcher) {
	t.Helper()
	repo := newMemRepo()
	reg := channel.NewRegistry()
	adapter := &stubAdapter{}
	reg.Register(channel.ChannelTypeSlack, adapter)
	disp := &stubDispatcher{reply: "hello back"}
	svc := channel.NewService(repo, reg).WithDispatcher(disp)
	return channel.NewHandler(svc), repo, reg, adapter, disp
}

// --- tests ---

func TestChannelResponse_MasksNestedSensitiveConfigKeys(t *testing.T) {
	ch := channel.Channel{
		ID:      uuid.New(),
		Name:    "slack",
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
	}

	resp := channel.ResponseFrom(ch)
	data, err := json.Marshal(resp)
	require.NoError(t, err)
	body := string(data)

	assert.NotContains(t, body, "nested-client-secret")
	assert.NotContains(t, body, "nested-access-token")
	assert.NotContains(t, body, "nested-signing-secret")
	assert.Contains(t, body, `"clientSecret":"***"`)
	assert.Contains(t, body, `"accessToken":"***"`)
	assert.Contains(t, body, `"signingSecret":"***"`)
	assert.Contains(t, body, "kept")
	assert.Contains(t, body, "message")
}

func TestServiceCreate_success(t *testing.T) {
	_, repo, _, _, _ := buildService(t)
	svc := channel.NewService(repo, channel.NewRegistry())

	resp, err := svc.Create(context.Background(), channel.CreateChannelRequest{
		Name:    "my-channel",
		Type:    channel.ChannelTypeSlack,
		AgentID: uuid.New(),
		Config:  json.RawMessage(`{"botToken":"xoxb-test"}`),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
	assert.Equal(t, "my-channel", resp.Name)
	assert.Equal(t, channel.ChannelTypeSlack, resp.Type)
}

func TestServiceCreate_missingName(t *testing.T) {
	_, repo, _, _, _ := buildService(t)
	svc := channel.NewService(repo, channel.NewRegistry())

	_, err := svc.Create(context.Background(), channel.CreateChannelRequest{
		Type:    channel.ChannelTypeSlack,
		AgentID: uuid.New(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestServiceCreate_missingType(t *testing.T) {
	_, repo, _, _, _ := buildService(t)
	svc := channel.NewService(repo, channel.NewRegistry())

	_, err := svc.Create(context.Background(), channel.CreateChannelRequest{
		Name:    "my-channel",
		AgentID: uuid.New(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "type is required")
}

func TestServiceCreate_defaultEnabled(t *testing.T) {
	repo := newMemRepo()
	svc := channel.NewService(repo, channel.NewRegistry())

	resp, err := svc.Create(context.Background(), channel.CreateChannelRequest{
		Name:    "ch",
		Type:    channel.ChannelTypeCustom,
		AgentID: uuid.New(),
	})
	require.NoError(t, err)
	assert.True(t, resp.Enabled)
}

func TestServiceGetByID_notFound(t *testing.T) {
	repo := newMemRepo()
	svc := channel.NewService(repo, channel.NewRegistry())

	_, err := svc.GetByID(context.Background(), uuid.New())
	require.Error(t, err)
	assert.True(t, errors.Is(err, channel.ErrNotFound))
}

func TestServiceUpdate_success(t *testing.T) {
	repo := newMemRepo()
	svc := channel.NewService(repo, channel.NewRegistry())

	created, err := svc.Create(context.Background(), channel.CreateChannelRequest{
		Name:    "original",
		Type:    channel.ChannelTypeSlack,
		AgentID: uuid.New(),
	})
	require.NoError(t, err)

	newName := "updated"
	enabled := false
	resp, err := svc.Update(context.Background(), created.ID, channel.UpdateChannelRequest{
		Name:    &newName,
		Enabled: &enabled,
	})
	require.NoError(t, err)
	assert.Equal(t, "updated", resp.Name)
	assert.False(t, resp.Enabled)
}

func TestServiceDelete_success(t *testing.T) {
	repo := newMemRepo()
	svc := channel.NewService(repo, channel.NewRegistry())

	created, err := svc.Create(context.Background(), channel.CreateChannelRequest{
		Name:    "to-delete",
		Type:    channel.ChannelTypeCustom,
		AgentID: uuid.New(),
	})
	require.NoError(t, err)

	err = svc.Delete(context.Background(), created.ID)
	require.NoError(t, err)

	_, err = svc.GetByID(context.Background(), created.ID)
	assert.True(t, errors.Is(err, channel.ErrNotFound))
}

func TestServiceHandleInbound_success(t *testing.T) {
	repo := newMemRepo()
	reg := channel.NewRegistry()
	adapter := &stubAdapter{
		parsedMsg: channel.InboundMessage{Text: "hello", ReplyTo: "C123"},
	}
	reg.Register(channel.ChannelTypeSlack, adapter)
	disp := &stubDispatcher{reply: "hi there"}
	svc := channel.NewService(repo, reg).WithDispatcher(disp)

	created, _ := svc.Create(context.Background(), channel.CreateChannelRequest{
		Name:    "slack-ch",
		Type:    channel.ChannelTypeSlack,
		AgentID: uuid.New(),
	})
	token := created.Token

	msg, err := svc.HandleInbound(context.Background(), token, nil, []byte(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "hello", msg.Text)
	assert.Equal(t, "hi there", adapter.sentReply.Text)
	assert.Equal(t, "C123", adapter.sentReply.ReplyTo)
}

func TestServiceHandleInbound_tokenNotFound(t *testing.T) {
	repo := newMemRepo()
	svc := channel.NewService(repo, channel.NewRegistry())

	_, err := svc.HandleInbound(context.Background(), "bad-token", nil, []byte(`{}`))
	require.Error(t, err)
	assert.True(t, errors.Is(err, channel.ErrTokenNotFound))
}

func TestServiceHandleInbound_verifyFails(t *testing.T) {
	repo := newMemRepo()
	reg := channel.NewRegistry()
	adapter := &stubAdapter{verifyErr: errors.New("bad signature")}
	reg.Register(channel.ChannelTypeSlack, adapter)
	svc := channel.NewService(repo, reg)

	created, _ := svc.Create(context.Background(), channel.CreateChannelRequest{
		Name:    "ch",
		Type:    channel.ChannelTypeSlack,
		AgentID: uuid.New(),
	})

	_, err := svc.HandleInbound(context.Background(), created.Token, nil, []byte(`{}`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "verify")
}

func TestServiceHandleInbound_parseFailsBeforeDispatch(t *testing.T) {
	repo := newMemRepo()
	reg := channel.NewRegistry()
	adapter := &stubAdapter{parseErr: errors.New("malformed payload")}
	reg.Register(channel.ChannelTypeSlack, adapter)
	dispatcher := &stubDispatcher{}
	svc := channel.NewService(repo, reg).WithDispatcher(dispatcher)
	created, err := repo.Create(context.Background(), channel.Channel{
		Name:    "ch",
		Type:    channel.ChannelTypeSlack,
		AgentID: uuid.New(),
		Enabled: true,
	})
	require.NoError(t, err)

	_, err = svc.HandleInbound(context.Background(), created.Token, nil, []byte(`{}`))

	require.ErrorIs(t, err, channel.ErrInvalidInboundPayload)
	assert.Zero(t, dispatcher.calls)
}

func TestServiceHandleInbound_urlVerificationChallenge(t *testing.T) {
	repo := newMemRepo()
	reg := channel.NewRegistry()
	adapter := &stubAdapter{
		parsedMsg: channel.InboundMessage{Challenge: "abc123"},
	}
	reg.Register(channel.ChannelTypeSlack, adapter)
	disp := &stubDispatcher{}
	svc := channel.NewService(repo, reg).WithDispatcher(disp)

	created, _ := svc.Create(context.Background(), channel.CreateChannelRequest{
		Name:    "ch",
		Type:    channel.ChannelTypeSlack,
		AgentID: uuid.New(),
	})

	msg, err := svc.HandleInbound(context.Background(), created.Token, nil, []byte(`{}`))
	require.NoError(t, err)
	assert.Equal(t, "abc123", msg.Challenge)
	// dispatcher should NOT have been called
	assert.Empty(t, adapter.sentReply.Text)
}

func TestServiceList_empty(t *testing.T) {
	repo := newMemRepo()
	svc := channel.NewService(repo, channel.NewRegistry())

	page, err := svc.List(context.Background(), pagination.PageRequest{Page: 0, Size: 20})
	require.NoError(t, err)
	assert.Equal(t, int64(0), page.TotalElements)
	assert.Empty(t, page.Content)
}
