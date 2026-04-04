package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- IsSSLErrorCode ---

func TestIsSSLErrorCode_Known(t *testing.T) {
	assert.True(t, agentic.IsSSLErrorCode("CERT_HAS_EXPIRED"))
	assert.True(t, agentic.IsSSLErrorCode("DEPTH_ZERO_SELF_SIGNED_CERT"))
	assert.True(t, agentic.IsSSLErrorCode("UNABLE_TO_VERIFY_LEAF_SIGNATURE"))
	assert.True(t, agentic.IsSSLErrorCode("SELF_SIGNED_CERT_IN_CHAIN"))
	assert.True(t, agentic.IsSSLErrorCode("ERR_TLS_CERT_ALTNAME_INVALID"))
}

func TestIsSSLErrorCode_Unknown(t *testing.T) {
	assert.False(t, agentic.IsSSLErrorCode("ETIMEDOUT"))
	assert.False(t, agentic.IsSSLErrorCode("ECONNREFUSED"))
	assert.False(t, agentic.IsSSLErrorCode(""))
}

// --- ClassifyConnectionError ---

func TestClassifyConnectionError_SSL(t *testing.T) {
	d := agentic.ClassifyConnectionError("CERT_HAS_EXPIRED", "certificate expired")
	assert.NotNil(t, d)
	assert.True(t, d.IsSSLError)
	assert.Equal(t, "CERT_HAS_EXPIRED", d.Code)
}

func TestClassifyConnectionError_NonSSL(t *testing.T) {
	d := agentic.ClassifyConnectionError("ETIMEDOUT", "connection timed out")
	assert.NotNil(t, d)
	assert.False(t, d.IsSSLError)
	assert.Equal(t, "ETIMEDOUT", d.Code)
}

func TestClassifyConnectionError_EmptyCode(t *testing.T) {
	d := agentic.ClassifyConnectionError("", "some error")
	assert.Nil(t, d)
}

// --- FormatConnectionError ---

func TestFormatConnectionError_Nil(t *testing.T) {
	result := agentic.FormatConnectionError(nil)
	assert.Equal(t, "Unable to connect to API. Check your internet connection", result)
}

func TestFormatConnectionError_Timeout(t *testing.T) {
	d := &agentic.ConnectionErrorDetails{Code: "ETIMEDOUT", Message: "timeout"}
	result := agentic.FormatConnectionError(d)
	assert.Contains(t, result, "timed out")
	assert.Contains(t, result, "proxy")
}

func TestFormatConnectionError_SSLVerification(t *testing.T) {
	d := &agentic.ConnectionErrorDetails{Code: "UNABLE_TO_VERIFY_LEAF_SIGNATURE", IsSSLError: true}
	result := agentic.FormatConnectionError(d)
	assert.Contains(t, result, "SSL certificate verification failed")
}

func TestFormatConnectionError_SSLExpired(t *testing.T) {
	d := &agentic.ConnectionErrorDetails{Code: "CERT_HAS_EXPIRED", IsSSLError: true}
	result := agentic.FormatConnectionError(d)
	assert.Contains(t, result, "expired")
}

func TestFormatConnectionError_SSLRevoked(t *testing.T) {
	d := &agentic.ConnectionErrorDetails{Code: "CERT_REVOKED", IsSSLError: true}
	result := agentic.FormatConnectionError(d)
	assert.Contains(t, result, "revoked")
}

func TestFormatConnectionError_SSLSelfSigned(t *testing.T) {
	d := &agentic.ConnectionErrorDetails{Code: "DEPTH_ZERO_SELF_SIGNED_CERT", IsSSLError: true}
	result := agentic.FormatConnectionError(d)
	assert.Contains(t, result, "Self-signed")
}

func TestFormatConnectionError_SSLHostname(t *testing.T) {
	d := &agentic.ConnectionErrorDetails{Code: "HOSTNAME_MISMATCH", IsSSLError: true}
	result := agentic.FormatConnectionError(d)
	assert.Contains(t, result, "hostname mismatch")
}

func TestFormatConnectionError_SSLNotYetValid(t *testing.T) {
	d := &agentic.ConnectionErrorDetails{Code: "CERT_NOT_YET_VALID", IsSSLError: true}
	result := agentic.FormatConnectionError(d)
	assert.Contains(t, result, "not yet valid")
}

func TestFormatConnectionError_SSLGeneric(t *testing.T) {
	d := &agentic.ConnectionErrorDetails{Code: "CERT_CHAIN_TOO_LONG", IsSSLError: true}
	result := agentic.FormatConnectionError(d)
	assert.Contains(t, result, "SSL error")
	assert.Contains(t, result, "CERT_CHAIN_TOO_LONG")
}

func TestFormatConnectionError_NonSSL(t *testing.T) {
	d := &agentic.ConnectionErrorDetails{Code: "ECONNREFUSED"}
	result := agentic.FormatConnectionError(d)
	assert.Contains(t, result, "ECONNREFUSED")
}

// --- SSLErrorHint ---

func TestSSLErrorHint_SSL(t *testing.T) {
	hint := agentic.SSLErrorHint("CERT_HAS_EXPIRED")
	assert.Contains(t, hint, "SSL certificate error")
	assert.Contains(t, hint, "CERT_HAS_EXPIRED")
	assert.Contains(t, hint, "corporate proxy")
}

func TestSSLErrorHint_NonSSL(t *testing.T) {
	hint := agentic.SSLErrorHint("ETIMEDOUT")
	assert.Equal(t, "", hint)
}

func TestSSLErrorHint_Empty(t *testing.T) {
	hint := agentic.SSLErrorHint("")
	assert.Equal(t, "", hint)
}

// --- SanitizeHTMLError ---

func TestSanitizeHTMLError_PlainMessage(t *testing.T) {
	result := agentic.SanitizeHTMLError("rate limit exceeded")
	assert.Equal(t, "rate limit exceeded", result)
}

func TestSanitizeHTMLError_HTMLWithTitle(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>502 Bad Gateway</title></head><body>...</body></html>`
	result := agentic.SanitizeHTMLError(html)
	assert.Equal(t, "502 Bad Gateway", result)
}

func TestSanitizeHTMLError_HTMLNoTitle(t *testing.T) {
	html := `<html><body>Error</body></html>`
	result := agentic.SanitizeHTMLError(html)
	assert.Equal(t, "", result)
}

func TestSanitizeHTMLError_HTMLEmptyTitle(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title></title></head></html>`
	result := agentic.SanitizeHTMLError(html)
	assert.Equal(t, "", result)
}

// --- FormatAPIError ---

func TestFormatAPIError_ConnectionTimeout(t *testing.T) {
	result := agentic.FormatAPIError("ETIMEDOUT", "timeout", 0)
	assert.Contains(t, result, "timed out")
}

func TestFormatAPIError_SSLError(t *testing.T) {
	result := agentic.FormatAPIError("CERT_HAS_EXPIRED", "cert expired", 0)
	assert.Contains(t, result, "expired")
}

func TestFormatAPIError_ConnectionError(t *testing.T) {
	result := agentic.FormatAPIError("", "Connection error.", 0)
	assert.Contains(t, result, "Check your internet")
}

func TestFormatAPIError_ConnectionErrorWithCode(t *testing.T) {
	result := agentic.FormatAPIError("ECONNREFUSED", "Connection error.", 0)
	assert.Contains(t, result, "ECONNREFUSED")
}

func TestFormatAPIError_NoMessage(t *testing.T) {
	result := agentic.FormatAPIError("", "", 429)
	assert.Equal(t, "API error (status 429)", result)
}

func TestFormatAPIError_HTMLMessage(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>503 Service Unavailable</title></head></html>`
	result := agentic.FormatAPIError("", html, 503)
	assert.Equal(t, "503 Service Unavailable", result)
}

func TestFormatAPIError_PlainMessage(t *testing.T) {
	result := agentic.FormatAPIError("", "rate limit exceeded", 429)
	assert.Equal(t, "rate limit exceeded", result)
}
