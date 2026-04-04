package agentic

import (
	"net"
	"strconv"
	"strings"
)

// SSRF guard for HTTP hook/webhook IP validation.
//
// Inspired by Claude Code's ssrfGuard.ts — blocks private,
// link-local, and non-routable address ranges to prevent
// HTTP hooks from reaching cloud metadata endpoints
// (169.254.169.254) or internal infrastructure. Loopback
// (127.0.0.0/8, ::1) is intentionally ALLOWED for local dev.

// IsBlockedAddress returns true if the IP address is in a range
// that HTTP hooks/webhooks should not reach.
//
// Blocked IPv4: 0.0.0.0/8, 10.0.0.0/8, 100.64.0.0/10 (CGNAT),
// 169.254.0.0/16 (link-local/metadata), 172.16.0.0/12, 192.168.0.0/16
//
// Blocked IPv6: :: (unspecified), fc00::/7 (unique local),
// fe80::/10 (link-local), ::ffff:<blocked-v4> (mapped)
//
// Allowed: 127.0.0.0/8 (loopback), ::1 (loopback), all public IPs
func IsBlockedAddress(address string) bool {
	ip := net.ParseIP(address)
	if ip == nil {
		return false
	}

	if ip4 := ip.To4(); ip4 != nil {
		return isBlockedV4(ip4)
	}
	return isBlockedV6(ip, address)
}

func isBlockedV4(ip net.IP) bool {
	a := ip[0]
	b := ip[1]

	// Loopback explicitly allowed.
	if a == 127 {
		return false
	}
	// 0.0.0.0/8
	if a == 0 {
		return true
	}
	// 10.0.0.0/8
	if a == 10 {
		return true
	}
	// 169.254.0.0/16 — link-local, cloud metadata
	if a == 169 && b == 254 {
		return true
	}
	// 172.16.0.0/12
	if a == 172 && b >= 16 && b <= 31 {
		return true
	}
	// 100.64.0.0/10 — CGNAT (RFC 6598)
	if a == 100 && b >= 64 && b <= 127 {
		return true
	}
	// 192.168.0.0/16
	if a == 192 && b == 168 {
		return true
	}

	return false
}

func isBlockedV6(ip net.IP, address string) bool {
	lower := strings.ToLower(address)

	// ::1 loopback explicitly allowed.
	if ip.Equal(net.IPv6loopback) {
		return false
	}
	// :: unspecified.
	if ip.Equal(net.IPv6unspecified) {
		return true
	}

	// IPv4-mapped IPv6 — check the embedded IPv4 address.
	if ip4 := extractMappedIPv4(ip); ip4 != nil {
		return isBlockedV4(ip4)
	}

	// fc00::/7 — unique local (fc00:: through fdff::).
	if strings.HasPrefix(lower, "fc") || strings.HasPrefix(lower, "fd") {
		// Also check via the raw bytes for expanded forms.
		if len(ip) == 16 && (ip[0] == 0xfc || ip[0] == 0xfd) {
			return true
		}
		return true
	}
	if len(ip) == 16 && (ip[0] == 0xfc || ip[0] == 0xfd) {
		return true
	}

	// fe80::/10 — link-local.
	if len(ip) == 16 && ip[0] == 0xfe && (ip[1]&0xc0) == 0x80 {
		return true
	}

	return false
}

// extractMappedIPv4 returns the embedded IPv4 address from an
// IPv4-mapped IPv6 address (::ffff:X.Y.Z.W), or nil if not mapped.
func extractMappedIPv4(ip net.IP) net.IP {
	if len(ip) != 16 {
		return nil
	}
	// Check prefix: first 10 bytes zero, next 2 bytes 0xffff.
	for i := 0; i < 10; i++ {
		if ip[i] != 0 {
			return nil
		}
	}
	if ip[10] != 0xff || ip[11] != 0xff {
		return nil
	}
	return net.IP(ip[12:16])
}

// IsPrivateIPv4String checks if a dotted-decimal IPv4 string is in
// a private range (10/8, 172.16/12, 192.168/16).
func IsPrivateIPv4String(address string) bool {
	parts := strings.Split(address, ".")
	if len(parts) != 4 {
		return false
	}
	a, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	b, err := strconv.Atoi(parts[1])
	if err != nil {
		return false
	}

	if a == 10 {
		return true
	}
	if a == 172 && b >= 16 && b <= 31 {
		return true
	}
	if a == 192 && b == 168 {
		return true
	}
	return false
}
