package ssrf_test

import (
	"net"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/ssrf"
)

func TestValidateURL_RFC1918_10x_Blocked(t *testing.T) {
	err := ssrf.ValidateURL("http://10.0.0.1/api/data")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_RFC1918_172x_Blocked(t *testing.T) {
	err := ssrf.ValidateURL("http://172.16.0.1/api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_RFC1918_192168_Blocked(t *testing.T) {
	err := ssrf.ValidateURL("http://192.168.1.1/api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_Loopback_Blocked(t *testing.T) {
	err := ssrf.ValidateURL("http://127.0.0.1/api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_LinkLocal_Blocked(t *testing.T) {
	// AWS metadata endpoint
	err := ssrf.ValidateURL("http://169.254.169.254/latest/meta-data/")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_CGNAT_Blocked(t *testing.T) {
	err := ssrf.ValidateURL("http://100.64.0.1/api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_ClusterLocal_Blocked(t *testing.T) {
	err := ssrf.ValidateURL("http://keycloak.agenthub.svc.cluster.local/auth")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster")
}

func TestValidateURL_ClusterLocalSuffix_Blocked(t *testing.T) {
	err := ssrf.ValidateURL("http://minio.cluster.local/buckets")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster")
}

func TestValidateURL_ClusterLocalUppercase_Blocked(t *testing.T) {
	err := ssrf.ValidateURL("http://KEYCLOAK.AGENTHUB.SVC.CLUSTER.LOCAL/auth")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cluster")
}

func TestValidateURL_TrailingDotLocalTargets_Blocked(t *testing.T) {
	tests := []struct {
		name       string
		rawURL     string
		wantErrSub string
	}{
		{
			name:       "localhost fqdn",
			rawURL:     "http://localhost./api",
			wantErrSub: "loopback",
		},
		{
			name:       "loopback ip fqdn",
			rawURL:     "http://127.0.0.1./api",
			wantErrSub: "private",
		},
		{
			name:       "metadata ip fqdn",
			rawURL:     "http://169.254.169.254./latest/meta-data/",
			wantErrSub: "private",
		},
		{
			name:       "svc cluster local fqdn",
			rawURL:     "http://keycloak.agenthub.svc.cluster.local./auth",
			wantErrSub: "cluster",
		},
		{
			name:       "cluster local fqdn",
			rawURL:     "http://minio.cluster.local./buckets",
			wantErrSub: "cluster",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ssrf.ValidateURL(tc.rawURL)
			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), tc.wantErrSub)
		})
	}
}

func TestValidateURL_InvalidURL_ReturnsError(t *testing.T) {
	err := ssrf.ValidateURL("not-a-url")
	require.Error(t, err)
}

func TestValidateURL_UnsupportedScheme_Blocked(t *testing.T) {
	err := ssrf.ValidateURL("ftp://[2606:4700:4700::1111]/resource")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "scheme")
}

func TestValidateURL_EmptyHost_ReturnsError(t *testing.T) {
	err := ssrf.ValidateURL("http:///path")
	require.Error(t, err)
}

func TestValidateURL_IPv6Loopback_Blocked(t *testing.T) {
	err := ssrf.ValidateURL("http://[::1]/api")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "private")
}

func TestValidateURL_ExternalHostname_Allowed(t *testing.T) {
	// External hostname (no DNS resolution in static check) should pass.
	err := ssrf.ValidateURL("https://api.example.com/data")
	assert.NoError(t, err)
}

func TestValidateURL_ExternalHostnameTrailingDot_Allowed(t *testing.T) {
	// A trailing root dot is valid FQDN syntax for external hosts.
	err := ssrf.ValidateURL("https://api.example.com./data")
	assert.NoError(t, err)
}

func TestValidateHost_TrailingDotLocalTargets_Blocked(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		wantErrSub string
	}{
		{
			name:       "localhost fqdn",
			host:       "localhost.",
			wantErrSub: "loopback",
		},
		{
			name:       "loopback ip fqdn",
			host:       "127.0.0.1.",
			wantErrSub: "loopback",
		},
		{
			name:       "metadata ip fqdn",
			host:       "169.254.169.254.",
			wantErrSub: "link-local",
		},
		{
			name:       "svc cluster local fqdn",
			host:       "keycloak.agenthub.svc.cluster.local.",
			wantErrSub: "cluster",
		},
		{
			name:       "cluster local fqdn",
			host:       "minio.cluster.local.",
			wantErrSub: "cluster",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ssrf.ValidateHost(tc.host)
			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), tc.wantErrSub)
		})
	}
}

func TestValidateURL_Idempotent(t *testing.T) {
	// Calling twice on the same safe URL should consistently pass.
	u := "https://api.example.com/data"
	assert.NoError(t, ssrf.ValidateURL(u))
	assert.NoError(t, ssrf.ValidateURL(u))
}

func FuzzValidateURLBlocksLiteralPrivateHosts(f *testing.F) {
	for _, seed := range []string{
		"10.0.0.1",
		"172.16.0.1",
		"172.31.255.255",
		"192.168.1.1",
		"127.0.0.1",
		"169.254.169.254",
		"100.64.0.1",
		"100.127.255.255",
		"::1",
		"fc00::1",
		"fd12:3456:789a::1",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, host string) {
		host = strings.TrimSpace(host)
		ip := net.ParseIP(host)
		if ip == nil || !isBlockedURLLiteralIP(ip) {
			return
		}

		err := ssrf.ValidateURL("http://" + urlHostFromLiteral(host) + "/api")
		if err == nil {
			t.Fatalf("expected literal private/reserved host %q to be blocked", host)
		}
	})
}

func FuzzValidateURLBlocksClusterLocalSuffix(f *testing.F) {
	for _, seed := range []string{
		"keycloak.agenthub",
		"KEYCLOAK.AGENTHUB",
		"minio",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, prefix string) {
		prefix = dnsPrefixFromFuzzInput(prefix)
		for _, host := range []string{
			prefix + ".cluster.local",
			prefix + ".svc.cluster.local",
			strings.ToUpper(prefix) + ".SVC.CLUSTER.LOCAL",
		} {
			err := ssrf.ValidateURL("http://" + host + "/api")
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "cluster") {
				t.Fatalf("expected cluster-local host %q to be blocked, got %v", host, err)
			}
		}
	})
}

func FuzzValidateHostBlocksLocalTargets(f *testing.F) {
	for _, seed := range []string{
		"localhost",
		"LOCALHOST",
		"ip6-localhost",
		"ip6-loopback",
		"127.0.0.1",
		"127.255.255.255",
		"169.254.169.254",
		"::1",
		"keycloak.agenthub.svc.cluster.local",
		"KEYCLOAK.AGENTHUB.SVC.CLUSTER.LOCAL",
		"minio.cluster.local",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, host string) {
		host = strings.TrimSpace(host)
		if !isBlockedValidateHostTarget(host) {
			return
		}

		err := ssrf.ValidateHost(host)
		if err == nil {
			t.Fatalf("expected local/internal host %q to be blocked", host)
		}
	})
}

func isBlockedURLLiteralIP(ip net.IP) bool {
	for _, cidr := range []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"::1/128",
		"fc00::/7",
		"100.64.0.0/10",
	} {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			panic(err)
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func urlHostFromLiteral(host string) string {
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		return "[" + host + "]"
	}
	return host
}

func dnsPrefixFromFuzzInput(input string) string {
	var b strings.Builder
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '-',
			r == '.':
			b.WriteRune(r)
		}
		if b.Len() >= 63 {
			break
		}
	}

	prefix := strings.Trim(b.String(), ".-")
	if prefix == "" {
		return "service"
	}
	return prefix
}

func isBlockedValidateHostTarget(host string) bool {
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || lowerHost == "ip6-localhost" || lowerHost == "ip6-loopback" {
		return true
	}
	if strings.HasSuffix(lowerHost, ".svc.cluster.local") ||
		strings.HasSuffix(lowerHost, ".cluster.local") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback() || ip.IsLinkLocalUnicast()
	}
	return false
}
