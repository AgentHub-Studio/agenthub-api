package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestToTaggedID_Basic(t *testing.T) {
	id, err := agentic.ToTaggedID("user", "550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	assert.True(t, len(id) > 0)
	assert.Contains(t, id, "user_01")
}

func TestToTaggedID_NoHyphens(t *testing.T) {
	id, err := agentic.ToTaggedID("org", "550e8400e29b41d4a716446655440000")
	require.NoError(t, err)
	assert.Contains(t, id, "org_01")
}

func TestToTaggedID_DifferentTags(t *testing.T) {
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	userID, _ := agentic.ToTaggedID("user", uuid)
	orgID, _ := agentic.ToTaggedID("org", uuid)
	assert.NotEqual(t, userID, orgID, "different tags should produce different IDs")
	// But the encoded part should be the same
	assert.Equal(t, userID[5:], orgID[4:], "encoded UUID should be identical")
}

func TestToTaggedID_InvalidUUID(t *testing.T) {
	_, err := agentic.ToTaggedID("user", "not-a-uuid")
	assert.Error(t, err)
}

func TestToTaggedID_ShortUUID(t *testing.T) {
	_, err := agentic.ToTaggedID("user", "550e8400")
	assert.Error(t, err)
}

func TestToTaggedID_FixedLength(t *testing.T) {
	// The encoded part should always be exactly 22 chars + version 2 chars
	id, err := agentic.ToTaggedID("user", "550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	// "user_" (5) + "01" (2) + base58(22) = 29
	assert.Len(t, id, 29)
}

func TestToTaggedID_ZeroUUID(t *testing.T) {
	id, err := agentic.ToTaggedID("test", "00000000-0000-0000-0000-000000000000")
	require.NoError(t, err)
	assert.Contains(t, id, "test_01")
}

// --- ParseTaggedID ---

func TestParseTaggedID_Roundtrip(t *testing.T) {
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	id, err := agentic.ToTaggedID("user", uuid)
	require.NoError(t, err)

	tag, parsedUUID, err := agentic.ParseTaggedID(id)
	require.NoError(t, err)
	assert.Equal(t, "user", tag)
	assert.Equal(t, uuid, parsedUUID)
}

func TestParseTaggedID_ZeroUUID(t *testing.T) {
	uuid := "00000000-0000-0000-0000-000000000000"
	id, _ := agentic.ToTaggedID("test", uuid)

	tag, parsedUUID, err := agentic.ParseTaggedID(id)
	require.NoError(t, err)
	assert.Equal(t, "test", tag)
	assert.Equal(t, uuid, parsedUUID)
}

func TestParseTaggedID_NoSeparator(t *testing.T) {
	_, _, err := agentic.ParseTaggedID("nounderscore")
	assert.Error(t, err)
}

func TestParseTaggedID_WrongVersion(t *testing.T) {
	_, _, err := agentic.ParseTaggedID("user_99AAAAAAAAAAAAAAAAAAAAAA")
	assert.Error(t, err)
}

func TestParseTaggedID_InvalidBase58(t *testing.T) {
	_, _, err := agentic.ParseTaggedID("user_010000000000000000000000") // '0' is not in base58
	assert.Error(t, err)
}

func TestParseTaggedID_MultipleUUIDs(t *testing.T) {
	uuids := []string{
		"550e8400-e29b-41d4-a716-446655440000",
		"6ba7b810-9dad-11d1-80b4-00c04fd430c8",
		"ffffffff-ffff-ffff-ffff-ffffffffffff",
		"00000000-0000-0000-0000-000000000001",
	}

	for _, uuid := range uuids {
		id, err := agentic.ToTaggedID("session", uuid)
		require.NoError(t, err)

		tag, parsed, err := agentic.ParseTaggedID(id)
		require.NoError(t, err)
		assert.Equal(t, "session", tag)
		assert.Equal(t, uuid, parsed)
	}
}
