// Package ssrf provides URL validation to prevent Server-Side Request Forgery.
// P-C220-1 / P-C221-1: HTTP tools must not target private or cluster-internal addresses.
package ssrf

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// privateCIDRs is the blocklist used when validating HTTP tool URLs at creation time.
var privateCIDRs []*net.IPNet

// allowedHosts lets operators whitelist specific internal hostnames (typically
// for dev/test harnesses inside the cluster). Populated from the
// SSRF_ALLOWED_HOSTS env var at process start — comma-separated exact host
// matches, e.g. "fake-crm-api.agenthub-e2e.svc.cluster.local,echo.internal".
// Entries bypass both the cluster-DNS suffix check and the private-IP check.
var allowedHosts map[string]struct{}

func init() {
	cidrs := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16", // link-local / AWS metadata
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 ULA
		"100.64.0.0/10",  // CGNAT (RFC 6598)
	}
	for _, cidr := range cidrs {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil {
			privateCIDRs = append(privateCIDRs, network)
		}
	}
	allowedHosts = map[string]struct{}{}
	for _, h := range strings.Split(os.Getenv("SSRF_ALLOWED_HOSTS"), ",") {
		h = strings.TrimSpace(strings.ToLower(h))
		if h != "" {
			allowedHosts[h] = struct{}{}
		}
	}
}

// ValidateURL performs static SSRF checks on rawURL:
//   - Blocks *.svc.cluster.local and *.cluster.local hostnames.
//   - Blocks any URL whose host parses directly as a private/reserved IP.
//
// DNS resolution is intentionally skipped here (that is the runtime's job) so
// that the API layer remains testable without network access. The skill-runtime
// executor performs a full DNS-resolved check at execution time.
func ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Host == "" {
		return fmt.Errorf("URL has no host")
	}

	host := u.Hostname()

	// Explicit allowlist bypass (dev/test harness hosts).
	if _, ok := allowedHosts[strings.ToLower(host)]; ok {
		return nil
	}

	// Block internal cluster DNS names.
	if strings.HasSuffix(host, ".svc.cluster.local") ||
		strings.HasSuffix(host, ".cluster.local") {
		return fmt.Errorf("URL targets internal cluster DNS: %s", host)
	}

	// If the URL host is a literal IP address, check it directly.
	if ip := net.ParseIP(host); ip != nil {
		for _, cidr := range privateCIDRs {
			if cidr.Contains(ip) {
				return fmt.Errorf("URL targets private/reserved network address: %s", host)
			}
		}
	}

	return nil
}
