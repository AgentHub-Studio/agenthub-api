// Package ssrf provides URL validation to prevent Server-Side Request Forgery.
// P-C220-1 / P-C221-1: HTTP tools must not target private or cluster-internal addresses.
package ssrf

import (
	"errors"
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
		h = canonicalHost(strings.TrimSpace(h))
		if h != "" {
			allowedHosts[h] = struct{}{}
		}
	}
}

// AllowHost adds a host to the in-memory allowlist. Used by test harnesses
// (httptest.NewServer returns 127.0.0.1 URLs that would otherwise be blocked).
// Production code should never call this — set SSRF_ALLOWED_HOSTS env var
// instead.
func AllowHost(host string) {
	if host == "" {
		return
	}
	if normalized := canonicalHost(host); normalized != "" {
		allowedHosts[normalized] = struct{}{}
	}
}

func canonicalHost(host string) string {
	return strings.TrimRight(strings.ToLower(strings.TrimSpace(host)), ".")
}

// ValidateHost performs narrower SSRF checks suitable for *database hosts*.
// Unlike ValidateURL, it allows RFC1918 private addresses (10.0.0.0/8,
// 172.16.0.0/12, 192.168.0.0/16, 100.64.0.0/10) since those are legitimate
// targets for user-managed on-prem databases reached via VPN. Still blocks:
//   - "localhost" / "ip6-localhost" / "ip6-loopback" hostnames
//   - 127.0.0.0/8 (loopback)
//   - 169.254.0.0/16 (link-local / AWS metadata)
//   - ::1 (IPv6 loopback)
//   - *.svc.cluster.local / *.cluster.local
//
// Bug 103: SQL tool executor connects to data_source.host from inside the
// cluster. Without this gate, an admin could exfil cluster secrets by pointing
// a datasource at the AWS metadata endpoint or k8s service DNS.
func ValidateHost(host string) error {
	host = strings.TrimSpace(host)
	if host == "" {
		return fmt.Errorf("host is empty")
	}
	normalizedHost := canonicalHost(host)
	if normalizedHost == "" {
		return fmt.Errorf("host is empty")
	}

	if _, ok := allowedHosts[normalizedHost]; ok {
		return nil
	}

	if normalizedHost == "localhost" || normalizedHost == "ip6-localhost" || normalizedHost == "ip6-loopback" {
		return fmt.Errorf("host targets loopback hostname: %s", host)
	}
	if strings.HasSuffix(normalizedHost, ".svc.cluster.local") ||
		strings.HasSuffix(normalizedHost, ".cluster.local") {
		return fmt.Errorf("host targets internal cluster DNS: %s", host)
	}

	if ip := net.ParseIP(normalizedHost); ip != nil {
		// Loopback (127.0.0.0/8 + ::1)
		if ip.IsLoopback() {
			return fmt.Errorf("host targets loopback address: %s", host)
		}
		// Link-local (169.254.0.0/16) — covers AWS / GCP / Azure instance metadata
		if ip.IsLinkLocalUnicast() {
			return fmt.Errorf("host targets link-local / metadata address: %s", host)
		}
	}
	return nil
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
		return errors.New("invalid URL")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL has no host")
	}

	host := u.Hostname()
	normalizedHost := canonicalHost(host)
	if normalizedHost == "" {
		return fmt.Errorf("URL has no host")
	}

	// Explicit allowlist bypass (dev/test harness hosts).
	if _, ok := allowedHosts[normalizedHost]; ok {
		return nil
	}

	// Block internal cluster DNS names.
	if strings.HasSuffix(normalizedHost, ".svc.cluster.local") ||
		strings.HasSuffix(normalizedHost, ".cluster.local") {
		return fmt.Errorf("URL targets internal cluster DNS: %s", host)
	}

	// Block well-known loopback hostnames that don't parse as IPs.
	if normalizedHost == "localhost" || normalizedHost == "ip6-localhost" || normalizedHost == "ip6-loopback" {
		return fmt.Errorf("URL targets loopback hostname: %s", host)
	}

	// If the URL host is a literal IP address, check it directly.
	if ip := net.ParseIP(normalizedHost); ip != nil {
		for _, cidr := range privateCIDRs {
			if cidr.Contains(ip) {
				return fmt.Errorf("URL targets private/reserved network address: %s", host)
			}
		}
	}

	return nil
}
