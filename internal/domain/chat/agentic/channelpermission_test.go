package agentic_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- ShortRequestID ---

func TestShortRequestID_Length(t *testing.T) {
	id := agentic.ShortRequestID("toolu_abc123def456")
	assert.Len(t, id, 5)
}

func TestShortRequestID_LettersOnly(t *testing.T) {
	id := agentic.ShortRequestID("toolu_xyz789")
	for _, c := range id {
		assert.True(t, c >= 'a' && c <= 'z' && c != 'l', "char %c should be a-z without l", c)
	}
}

func TestShortRequestID_Deterministic(t *testing.T) {
	id1 := agentic.ShortRequestID("toolu_same_input")
	id2 := agentic.ShortRequestID("toolu_same_input")
	assert.Equal(t, id1, id2)
}

func TestShortRequestID_DifferentInputs(t *testing.T) {
	id1 := agentic.ShortRequestID("toolu_aaa")
	id2 := agentic.ShortRequestID("toolu_bbb")
	assert.NotEqual(t, id1, id2)
}

func TestShortRequestID_NoBlocklisted(t *testing.T) {
	// Generate many IDs and ensure none contain blocklisted words
	for i := 0; i < 100; i++ {
		id := agentic.ShortRequestID("toolu_test_" + string(rune('a'+i)))
		assert.NotContains(t, id, "fuck")
		assert.NotContains(t, id, "shit")
	}
}

// --- TruncateForPreview ---

func TestTruncateForPreview_Short(t *testing.T) {
	assert.Equal(t, "hello", agentic.TruncateForPreview("hello", 200))
}

func TestTruncateForPreview_Long(t *testing.T) {
	long := "abcdefghij"
	result := agentic.TruncateForPreview(long, 5)
	assert.Equal(t, "abcde…", result)
}

func TestTruncateForPreview_Exact(t *testing.T) {
	assert.Equal(t, "abc", agentic.TruncateForPreview("abc", 3))
}

func TestTruncateForPreview_ZeroMax(t *testing.T) {
	assert.Equal(t, "…", agentic.TruncateForPreview("hello", 0))
}

// --- ChannelPermissionCallbacks ---

func TestChannelPermission_NewCallbacks(t *testing.T) {
	c := agentic.NewChannelPermissionCallbacks()
	assert.NotNil(t, c)
	assert.Equal(t, 0, c.PendingCount())
}

func TestChannelPermission_OnResponse_Resolve(t *testing.T) {
	c := agentic.NewChannelPermissionCallbacks()

	var got agentic.ChannelPermissionResponse
	unsub := c.OnResponse("TBXKQ", func(resp agentic.ChannelPermissionResponse) {
		got = resp
	})
	_ = unsub

	assert.Equal(t, 1, c.PendingCount())

	ok := c.Resolve("tbxkq", agentic.ChannelPermissionAllow, "telegram")
	assert.True(t, ok)
	assert.Equal(t, agentic.ChannelPermissionAllow, got.Behavior)
	assert.Equal(t, "telegram", got.FromServer)
	assert.Equal(t, 0, c.PendingCount())
}

func TestChannelPermission_Resolve_Unknown(t *testing.T) {
	c := agentic.NewChannelPermissionCallbacks()
	ok := c.Resolve("nonexistent", agentic.ChannelPermissionAllow, "s")
	assert.False(t, ok)
}

func TestChannelPermission_Resolve_DuplicateIgnored(t *testing.T) {
	c := agentic.NewChannelPermissionCallbacks()
	callCount := 0
	c.OnResponse("id1", func(resp agentic.ChannelPermissionResponse) {
		callCount++
	})

	ok1 := c.Resolve("id1", agentic.ChannelPermissionAllow, "s")
	ok2 := c.Resolve("id1", agentic.ChannelPermissionAllow, "s")
	assert.True(t, ok1)
	assert.False(t, ok2, "second resolve should be no-op")
	assert.Equal(t, 1, callCount)
}

func TestChannelPermission_Unsubscribe(t *testing.T) {
	c := agentic.NewChannelPermissionCallbacks()
	unsub := c.OnResponse("id1", func(resp agentic.ChannelPermissionResponse) {})
	assert.Equal(t, 1, c.PendingCount())

	unsub()
	assert.Equal(t, 0, c.PendingCount())
}

func TestChannelPermission_CaseInsensitive(t *testing.T) {
	c := agentic.NewChannelPermissionCallbacks()
	var got agentic.ChannelPermissionResponse
	c.OnResponse("AbCdE", func(resp agentic.ChannelPermissionResponse) {
		got = resp
	})

	ok := c.Resolve("abcde", agentic.ChannelPermissionDeny, "discord")
	assert.True(t, ok)
	assert.Equal(t, agentic.ChannelPermissionDeny, got.Behavior)
}

func TestChannelPermission_ConcurrentAccess(t *testing.T) {
	c := agentic.NewChannelPermissionCallbacks()
	var wg sync.WaitGroup

	for i := 0; i < 20; i++ {
		wg.Add(1)
		id := string(rune('a' + i))
		go func() {
			defer wg.Done()
			unsub := c.OnResponse(id, func(resp agentic.ChannelPermissionResponse) {})
			c.Resolve(id, agentic.ChannelPermissionAllow, "s")
			unsub()
		}()
	}
	wg.Wait()
	assert.Equal(t, 0, c.PendingCount())
}

// --- FilterPermissionRelayClients ---

func TestFilterPermissionRelayClients(t *testing.T) {
	clients := []agentic.ChannelClient{
		{
			Name:      "telegram",
			Connected: true,
			Capabilities: map[string]interface{}{
				"claude/channel":            true,
				"claude/channel/permission": true,
			},
		},
		{
			Name:      "discord",
			Connected: true,
			Capabilities: map[string]interface{}{
				"claude/channel": true,
				// Missing permission capability
			},
		},
		{
			Name:      "slack",
			Connected: false,
			Capabilities: map[string]interface{}{
				"claude/channel":            true,
				"claude/channel/permission": true,
			},
		},
		{
			Name:      "imessage",
			Connected: true,
			Capabilities: map[string]interface{}{
				"claude/channel":            true,
				"claude/channel/permission": true,
			},
		},
	}

	allowAll := func(name string) bool { return true }
	result := agentic.FilterPermissionRelayClients(clients, allowAll)
	require.Len(t, result, 2)
	assert.Equal(t, "telegram", result[0].Name)
	assert.Equal(t, "imessage", result[1].Name)
}

func TestFilterPermissionRelayClients_WithAllowlist(t *testing.T) {
	clients := []agentic.ChannelClient{
		{
			Name:      "telegram",
			Connected: true,
			Capabilities: map[string]interface{}{
				"claude/channel":            true,
				"claude/channel/permission": true,
			},
		},
		{
			Name:      "discord",
			Connected: true,
			Capabilities: map[string]interface{}{
				"claude/channel":            true,
				"claude/channel/permission": true,
			},
		},
	}

	onlyTelegram := func(name string) bool { return name == "telegram" }
	result := agentic.FilterPermissionRelayClients(clients, onlyTelegram)
	require.Len(t, result, 1)
	assert.Equal(t, "telegram", result[0].Name)
}

func TestFilterPermissionRelayClients_NilCapabilities(t *testing.T) {
	clients := []agentic.ChannelClient{
		{Name: "x", Connected: true, Capabilities: nil},
	}
	result := agentic.FilterPermissionRelayClients(clients, func(string) bool { return true })
	assert.Empty(t, result)
}

// --- Behaviors ---

func TestChannelPermissionBehavior_Values(t *testing.T) {
	assert.Equal(t, agentic.ChannelPermissionBehavior("allow"), agentic.ChannelPermissionAllow)
	assert.Equal(t, agentic.ChannelPermissionBehavior("deny"), agentic.ChannelPermissionDeny)
}
