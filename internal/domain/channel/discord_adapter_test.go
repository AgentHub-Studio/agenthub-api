package channel_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/channel"
)

func discordChannel(t *testing.T, pubKey string) channel.Channel {
	t.Helper()
	cfg, _ := json.Marshal(map[string]string{
		"publicKey": pubKey,
		"botToken":  "Bot test-bot-token",
	})
	return channel.Channel{
		ID:      uuid.New(),
		Type:    channel.ChannelTypeDiscord,
		Config:  cfg,
		Enabled: true,
	}
}

func signDiscord(t *testing.T, priv ed25519.PrivateKey, timestamp, body string) map[string]string {
	t.Helper()
	msg := []byte(timestamp + body)
	sig := ed25519.Sign(priv, msg)
	return map[string]string{
		"X-Signature-Ed25519":   hex.EncodeToString(sig),
		"X-Signature-Timestamp": timestamp,
	}
}

func TestDiscordAdapter_VerifyRequest_Valid(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	body := `{"type":2,"id":"1"}`
	headers := signDiscord(t, priv, "1700000000", body)

	ch := discordChannel(t, hex.EncodeToString(pub))
	adapter := &channel.DiscordAdapter{}
	err = adapter.VerifyRequest(context.Background(), ch, headers, []byte(body))
	assert.NoError(t, err)
}

func TestDiscordAdapter_VerifyRequest_Invalid(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	_, wrongPriv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	body := `{"type":2,"id":"1"}`
	headers := signDiscord(t, wrongPriv, "1700000000", body)

	ch := discordChannel(t, hex.EncodeToString(pub))
	adapter := &channel.DiscordAdapter{}
	err = adapter.VerifyRequest(context.Background(), ch, headers, []byte(body))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signature verification failed")
}

func TestDiscordAdapter_VerifyRequest_MissingHeaders(t *testing.T) {
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	ch := discordChannel(t, hex.EncodeToString(pub))
	adapter := &channel.DiscordAdapter{}
	err = adapter.VerifyRequest(context.Background(), ch, map[string]string{}, []byte(`{}`))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing signature headers")
}

func TestDiscordAdapter_ParseMessage_PING(t *testing.T) {
	adapter := &channel.DiscordAdapter{}
	body := `{"type":1,"id":"ping-id"}`
	ch := discordChannel(t, "")
	msg, err := adapter.ParseMessage(context.Background(), ch, []byte(body))
	require.NoError(t, err)
	assert.Equal(t, "pong", msg.Challenge)
	assert.Empty(t, msg.Text)
}

func TestDiscordAdapter_ParseMessage_ApplicationCommand(t *testing.T) {
	adapter := &channel.DiscordAdapter{}
	body := `{
		"type": 2,
		"id": "cmd-123",
		"channel_id": "ch-456",
		"token": "interaction-token-xyz",
		"member": {
			"user": {"id": "user-789", "username": "alice", "global_name": "Alice"}
		},
		"data": {
			"name": "ask",
			"options": [{"name": "question", "value": "hello world"}]
		}
	}`
	ch := discordChannel(t, "")
	msg, err := adapter.ParseMessage(context.Background(), ch, []byte(body))
	require.NoError(t, err)
	assert.Equal(t, "hello world", msg.Text)
	assert.Equal(t, "user-789", msg.SenderID)
	assert.Equal(t, "Alice", msg.SenderName)
	assert.Equal(t, "ch-456:interaction-token-xyz", msg.ReplyTo)
	assert.Equal(t, "cmd-123", msg.ExternalID)
}

func TestDiscordAdapter_ParseMessage_CommandNoOptions(t *testing.T) {
	adapter := &channel.DiscordAdapter{}
	body := `{
		"type": 2,
		"id": "cmd-999",
		"channel_id": "ch-1",
		"token": "tok",
		"data": {"name": "help"}
	}`
	ch := discordChannel(t, "")
	msg, err := adapter.ParseMessage(context.Background(), ch, []byte(body))
	require.NoError(t, err)
	assert.Equal(t, "help", msg.Text)
}

func TestDiscordAdapter_ParseMessage_UnknownType(t *testing.T) {
	adapter := &channel.DiscordAdapter{}
	body := `{"type":99,"id":"x"}`
	ch := discordChannel(t, "")
	msg, err := adapter.ParseMessage(context.Background(), ch, []byte(body))
	require.NoError(t, err)
	assert.Empty(t, msg.Text)
	assert.Empty(t, msg.Challenge)
}
