package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- IsBlockedAddress IPv4 ---

func TestIsBlockedAddress_Loopback(t *testing.T) {
	assert.False(t, agentic.IsBlockedAddress("127.0.0.1"))
}

func TestIsBlockedAddress_LoopbackRange(t *testing.T) {
	assert.False(t, agentic.IsBlockedAddress("127.255.255.255"))
}

func TestIsBlockedAddress_ThisNetwork(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("0.0.0.0"))
}

func TestIsBlockedAddress_Private10(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("10.0.0.1"))
}

func TestIsBlockedAddress_Private172(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("172.16.0.1"))
}

func TestIsBlockedAddress_Private172Upper(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("172.31.255.255"))
}

func TestIsBlockedAddress_NotPrivate172(t *testing.T) {
	assert.False(t, agentic.IsBlockedAddress("172.32.0.1"))
}

func TestIsBlockedAddress_Private192(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("192.168.1.1"))
}

func TestIsBlockedAddress_LinkLocal(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("169.254.169.254"))
}

func TestIsBlockedAddress_CGNAT(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("100.100.100.200"))
}

func TestIsBlockedAddress_CGNATLower(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("100.64.0.1"))
}

func TestIsBlockedAddress_CGNATUpper(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("100.127.255.255"))
}

func TestIsBlockedAddress_NotCGNAT(t *testing.T) {
	assert.False(t, agentic.IsBlockedAddress("100.128.0.1"))
}

func TestIsBlockedAddress_PublicIP(t *testing.T) {
	assert.False(t, agentic.IsBlockedAddress("8.8.8.8"))
}

func TestIsBlockedAddress_PublicIP2(t *testing.T) {
	assert.False(t, agentic.IsBlockedAddress("1.1.1.1"))
}

// --- IsBlockedAddress IPv6 ---

func TestIsBlockedAddress_IPv6Loopback(t *testing.T) {
	assert.False(t, agentic.IsBlockedAddress("::1"))
}

func TestIsBlockedAddress_IPv6Unspecified(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("::"))
}

func TestIsBlockedAddress_IPv6UniqueLocal(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("fc00::1"))
}

func TestIsBlockedAddress_IPv6UniqueLocalFD(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("fd12:3456:789a::1"))
}

func TestIsBlockedAddress_IPv6LinkLocal(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("fe80::1"))
}

func TestIsBlockedAddress_IPv6Public(t *testing.T) {
	assert.False(t, agentic.IsBlockedAddress("2001:4860:4860::8888"))
}

func TestIsBlockedAddress_IPv4Mapped_Private(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("::ffff:10.0.0.1"))
}

func TestIsBlockedAddress_IPv4Mapped_Public(t *testing.T) {
	assert.False(t, agentic.IsBlockedAddress("::ffff:8.8.8.8"))
}

func TestIsBlockedAddress_IPv4Mapped_Metadata(t *testing.T) {
	assert.True(t, agentic.IsBlockedAddress("::ffff:169.254.169.254"))
}

// --- Invalid input ---

func TestIsBlockedAddress_InvalidIP(t *testing.T) {
	assert.False(t, agentic.IsBlockedAddress("not-an-ip"))
}

func TestIsBlockedAddress_Empty(t *testing.T) {
	assert.False(t, agentic.IsBlockedAddress(""))
}

// --- IsPrivateIPv4String ---

func TestIsPrivateIPv4String_Private10(t *testing.T) {
	assert.True(t, agentic.IsPrivateIPv4String("10.1.2.3"))
}

func TestIsPrivateIPv4String_Private172(t *testing.T) {
	assert.True(t, agentic.IsPrivateIPv4String("172.20.0.1"))
}

func TestIsPrivateIPv4String_Private192(t *testing.T) {
	assert.True(t, agentic.IsPrivateIPv4String("192.168.0.1"))
}

func TestIsPrivateIPv4String_Public(t *testing.T) {
	assert.False(t, agentic.IsPrivateIPv4String("8.8.8.8"))
}

func TestIsPrivateIPv4String_Invalid(t *testing.T) {
	assert.False(t, agentic.IsPrivateIPv4String("invalid"))
}
