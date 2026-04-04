package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- DJB2Hash ---

func TestDJB2Hash_Deterministic(t *testing.T) {
	assert.Equal(t, agentic.DJB2Hash("hello"), agentic.DJB2Hash("hello"))
}

func TestDJB2Hash_DifferentStrings(t *testing.T) {
	assert.NotEqual(t, agentic.DJB2Hash("hello"), agentic.DJB2Hash("world"))
}

func TestDJB2Hash_Empty(t *testing.T) {
	assert.Equal(t, int32(0), agentic.DJB2Hash(""))
}

func TestDJB2Hash_KnownValue(t *testing.T) {
	// DJB2 starts at 0, so "a" = (0<<5 - 0 + 97) = 97
	assert.Equal(t, int32(97), agentic.DJB2Hash("a"))
}

// --- FNV1aHash ---

func TestFNV1aHash_Deterministic(t *testing.T) {
	assert.Equal(t, agentic.FNV1aHash("test"), agentic.FNV1aHash("test"))
}

func TestFNV1aHash_Different(t *testing.T) {
	assert.NotEqual(t, agentic.FNV1aHash("foo"), agentic.FNV1aHash("bar"))
}

func TestFNV1aHash_NonZeroForEmpty(t *testing.T) {
	// FNV-1a has a non-zero offset basis
	assert.NotEqual(t, uint64(0), agentic.FNV1aHash(""))
}

// --- SHA256Hex ---

func TestSHA256Hex_SHA256(t *testing.T) {
	h := agentic.SHA256Hex("hello")
	// SHA-256 of "hello" is well-known
	assert.Equal(t, "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824", h)
}

func TestSHA256Hex_Deterministic(t *testing.T) {
	assert.Equal(t, agentic.SHA256Hex("data"), agentic.SHA256Hex("data"))
}

func TestSHA256Hex_Empty(t *testing.T) {
	h := agentic.SHA256Hex("")
	assert.Len(t, h, 64) // SHA-256 hex is 64 chars
}

// --- HashPair ---

func TestHashPair_Deterministic(t *testing.T) {
	assert.Equal(t, agentic.HashPair("a", "b"), agentic.HashPair("a", "b"))
}

func TestHashPair_Disambiguates(t *testing.T) {
	// ("ts","code") != ("tsc","ode") thanks to null separator
	assert.NotEqual(t, agentic.HashPair("ts", "code"), agentic.HashPair("tsc", "ode"))
}

func TestHashPair_OrderMatters(t *testing.T) {
	assert.NotEqual(t, agentic.HashPair("a", "b"), agentic.HashPair("b", "a"))
}

func TestHashPair_HexLength(t *testing.T) {
	h := agentic.HashPair("x", "y")
	assert.Len(t, h, 64)
}
