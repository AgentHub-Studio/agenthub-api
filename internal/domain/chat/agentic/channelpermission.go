package agentic

import (
	"hash/fnv"
	"strings"
	"sync"
)

// Channel permission request-response with short human-friendly IDs.
//
// Inspired by Claude Code's channelPermissions — generates short,
// letters-only IDs from tool-use UUIDs using FNV-1a hashing. Supports
// callback-based response matching for out-of-band permission channels
// (e.g., Telegram, Slack, iMessage). IDs avoid offensive substrings
// and use a 25-letter alphabet (no 'l' which looks like 1/I).

// 25-letter alphabet: a-z minus 'l' (looks like 1/I). 25^5 ≈ 9.8M space.
const permIDAlphabet = "abcdefghijkmnopqrstuvwxyz"

// Substrings to avoid in generated IDs.
var permIDAvoidSubstrings = []string{
	"fuck", "shit", "cunt", "cock", "dick", "twat", "piss", "crap",
	"bitch", "whore", "ass", "tit", "cum", "fag", "dyke", "nig",
	"kike", "rape", "nazi", "damn", "poo", "pee", "wank", "anus",
}

// ShortRequestID generates a 5-letter human-friendly ID from a tool-use ID.
// Uses FNV-1a hashing with the 25-letter alphabet. Re-hashes with a salt
// if the result contains a blocklisted substring (cap 10 retries).
func ShortRequestID(toolUseID string) string {
	candidate := hashToPermID(toolUseID)
	for salt := 0; salt < 10; salt++ {
		if !containsBlocklisted(candidate) {
			return candidate
		}
		candidate = hashToPermID(toolUseID + ":" + string(rune('0'+salt)))
	}
	return candidate
}

func hashToPermID(input string) string {
	h := fnv.New32a()
	h.Write([]byte(input))
	v := h.Sum32()

	var b [5]byte
	for i := 0; i < 5; i++ {
		b[i] = permIDAlphabet[v%25]
		v /= 25
	}
	return string(b[:])
}

func containsBlocklisted(id string) bool {
	for _, bad := range permIDAvoidSubstrings {
		if strings.Contains(id, bad) {
			return true
		}
	}
	return false
}

// TruncateForPreview truncates a string to maxLen characters for
// phone-sized display. Appends "…" if truncated.
func TruncateForPreview(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 0 {
		return "…"
	}
	return s[:maxLen] + "…"
}

// ChannelPermissionBehavior is the user's decision on a permission request.
type ChannelPermissionBehavior string

const (
	ChannelPermissionAllow ChannelPermissionBehavior = "allow"
	ChannelPermissionDeny  ChannelPermissionBehavior = "deny"
)

// ChannelPermissionResponse is the resolved permission decision.
type ChannelPermissionResponse struct {
	Behavior   ChannelPermissionBehavior
	FromServer string
}

// ChannelPermissionCallbacks manages pending permission requests
// with callback-based resolution.
type ChannelPermissionCallbacks struct {
	mu      sync.Mutex
	pending map[string]func(ChannelPermissionResponse)
}

// NewChannelPermissionCallbacks creates a callbacks instance.
func NewChannelPermissionCallbacks() *ChannelPermissionCallbacks {
	return &ChannelPermissionCallbacks{
		pending: make(map[string]func(ChannelPermissionResponse)),
	}
}

// OnResponse registers a handler for a request ID. Returns an
// unsubscribe function.
func (c *ChannelPermissionCallbacks) OnResponse(requestID string, handler func(ChannelPermissionResponse)) func() {
	key := strings.ToLower(requestID)
	c.mu.Lock()
	c.pending[key] = handler
	c.mu.Unlock()

	return func() {
		c.mu.Lock()
		delete(c.pending, key)
		c.mu.Unlock()
	}
}

// Resolve resolves a pending request. Returns true if the request was
// found and resolved. The handler is removed before invocation to
// prevent double-resolve from duplicate events.
func (c *ChannelPermissionCallbacks) Resolve(requestID string, behavior ChannelPermissionBehavior, fromServer string) bool {
	key := strings.ToLower(requestID)
	c.mu.Lock()
	handler, ok := c.pending[key]
	if ok {
		delete(c.pending, key)
	}
	c.mu.Unlock()

	if !ok {
		return false
	}
	handler(ChannelPermissionResponse{Behavior: behavior, FromServer: fromServer})
	return true
}

// PendingCount returns the number of pending requests.
func (c *ChannelPermissionCallbacks) PendingCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.pending)
}

// ChannelCapability is a required capability name for permission relay.
const (
	ChannelCapability           = "claude/channel"
	ChannelPermissionCapability = "claude/channel/permission"
)

// ChannelClient describes a connected channel client.
type ChannelClient struct {
	Name         string
	Connected    bool
	Capabilities map[string]interface{}
}

// FilterPermissionRelayClients returns only clients that are connected
// and declare both channel capabilities.
func FilterPermissionRelayClients(clients []ChannelClient, isAllowed func(name string) bool) []ChannelClient {
	var result []ChannelClient
	for _, c := range clients {
		if !c.Connected {
			continue
		}
		if !isAllowed(c.Name) {
			continue
		}
		if c.Capabilities == nil {
			continue
		}
		_, hasChan := c.Capabilities[ChannelCapability]
		_, hasPerm := c.Capabilities[ChannelPermissionCapability]
		if hasChan && hasPerm {
			result = append(result, c)
		}
	}
	return result
}
