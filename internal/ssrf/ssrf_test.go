package ssrf_test

import (
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

func TestValidateURL_InvalidURL_ReturnsError(t *testing.T) {
	err := ssrf.ValidateURL("not-a-url")
	require.Error(t, err)
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

func TestValidateURL_Idempotent(t *testing.T) {
	// Calling twice on the same safe URL should consistently pass.
	u := "https://api.example.com/data"
	assert.NoError(t, ssrf.ValidateURL(u))
	assert.NoError(t, ssrf.ValidateURL(u))
}
