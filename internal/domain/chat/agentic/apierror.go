package agentic

import (
	"fmt"
	"strings"
)

// API error classification and user-friendly formatting.
//
// Inspired by Claude Code's errorUtils.ts — classifies connection
// errors (SSL/TLS, timeout, network), extracts error codes from
// cause chains, sanitizes HTML error pages, and provides actionable
// hints for common SSL issues in corporate proxy environments.

// SSLErrorCode represents known SSL/TLS error codes from OpenSSL.
type SSLErrorCode string

// Known SSL/TLS error codes.
const (
	SSLUnableToVerifyLeaf      SSLErrorCode = "UNABLE_TO_VERIFY_LEAF_SIGNATURE"
	SSLUnableToGetIssuer       SSLErrorCode = "UNABLE_TO_GET_ISSUER_CERT"
	SSLUnableToGetIssuerLocal  SSLErrorCode = "UNABLE_TO_GET_ISSUER_CERT_LOCALLY"
	SSLCertSignatureFailure    SSLErrorCode = "CERT_SIGNATURE_FAILURE"
	SSLCertNotYetValid         SSLErrorCode = "CERT_NOT_YET_VALID"
	SSLCertExpired             SSLErrorCode = "CERT_HAS_EXPIRED"
	SSLCertRevoked             SSLErrorCode = "CERT_REVOKED"
	SSLCertRejected            SSLErrorCode = "CERT_REJECTED"
	SSLCertUntrusted           SSLErrorCode = "CERT_UNTRUSTED"
	SSLSelfSigned              SSLErrorCode = "DEPTH_ZERO_SELF_SIGNED_CERT"
	SSLSelfSignedInChain       SSLErrorCode = "SELF_SIGNED_CERT_IN_CHAIN"
	SSLChainTooLong            SSLErrorCode = "CERT_CHAIN_TOO_LONG"
	SSLPathLengthExceeded      SSLErrorCode = "PATH_LENGTH_EXCEEDED"
	SSLAltNameInvalid          SSLErrorCode = "ERR_TLS_CERT_ALTNAME_INVALID"
	SSLHostnameMismatch        SSLErrorCode = "HOSTNAME_MISMATCH"
	SSLHandshakeTimeout        SSLErrorCode = "ERR_TLS_HANDSHAKE_TIMEOUT"
	SSLWrongVersion            SSLErrorCode = "ERR_SSL_WRONG_VERSION_NUMBER"
	SSLDecryptionFailed        SSLErrorCode = "ERR_SSL_DECRYPTION_FAILED_OR_BAD_RECORD_MAC"
)

// sslErrorCodes is the set of all known SSL error codes.
var sslErrorCodes = map[SSLErrorCode]bool{
	SSLUnableToVerifyLeaf:     true,
	SSLUnableToGetIssuer:      true,
	SSLUnableToGetIssuerLocal: true,
	SSLCertSignatureFailure:   true,
	SSLCertNotYetValid:        true,
	SSLCertExpired:            true,
	SSLCertRevoked:            true,
	SSLCertRejected:           true,
	SSLCertUntrusted:          true,
	SSLSelfSigned:             true,
	SSLSelfSignedInChain:      true,
	SSLChainTooLong:           true,
	SSLPathLengthExceeded:     true,
	SSLAltNameInvalid:         true,
	SSLHostnameMismatch:       true,
	SSLHandshakeTimeout:       true,
	SSLWrongVersion:           true,
	SSLDecryptionFailed:       true,
}

// ConnectionErrorDetails holds extracted connection error information.
type ConnectionErrorDetails struct {
	Code       string
	Message    string
	IsSSLError bool
}

// IsSSLErrorCode returns true if the code is a known SSL/TLS error.
func IsSSLErrorCode(code string) bool {
	return sslErrorCodes[SSLErrorCode(code)]
}

// ClassifyConnectionError classifies an error code and message as
// a connection error with SSL detection.
func ClassifyConnectionError(code, message string) *ConnectionErrorDetails {
	if code == "" {
		return nil
	}
	return &ConnectionErrorDetails{
		Code:       code,
		Message:    message,
		IsSSLError: IsSSLErrorCode(code),
	}
}

// FormatConnectionError returns a user-friendly message for a
// connection error, with specific guidance for SSL issues.
func FormatConnectionError(details *ConnectionErrorDetails) string {
	if details == nil {
		return "Unable to connect to API. Check your internet connection"
	}

	// Handle timeout
	if details.Code == "ETIMEDOUT" {
		return "Request timed out. Check your internet connection and proxy settings"
	}

	// Handle SSL errors with specific messages
	if details.IsSSLError {
		return formatSSLError(details.Code)
	}

	return fmt.Sprintf("Unable to connect to API (%s)", details.Code)
}

// SSLErrorHint returns an actionable hint for SSL/TLS errors in
// corporate proxy environments. Returns empty string if not an SSL error.
func SSLErrorHint(code string) string {
	if !IsSSLErrorCode(code) {
		return ""
	}
	return fmt.Sprintf(
		"SSL certificate error (%s). If you are behind a corporate proxy or "+
			"TLS-intercepting firewall, configure your CA bundle path, or ask IT "+
			"to allowlist the API domain.",
		code,
	)
}

// SanitizeHTMLError strips HTML content (e.g., CloudFlare error pages)
// from a message string, returning the <title> content or empty string
// if HTML is detected. Returns original message if no HTML found.
func SanitizeHTMLError(message string) string {
	if !strings.Contains(message, "<!DOCTYPE html") && !strings.Contains(message, "<html") {
		return message
	}

	// Try to extract <title> content
	titleStart := strings.Index(message, "<title>")
	if titleStart >= 0 {
		titleStart += len("<title>")
		titleEnd := strings.Index(message[titleStart:], "</title>")
		if titleEnd >= 0 {
			title := strings.TrimSpace(message[titleStart : titleStart+titleEnd])
			if title != "" {
				return title
			}
		}
	}

	return ""
}

// FormatAPIError produces a user-friendly error message from an API
// error, handling connection errors, SSL issues, HTML responses, and
// nested error structures.
func FormatAPIError(code, message string, statusCode int) string {
	// Check for connection errors
	if code != "" {
		details := ClassifyConnectionError(code, message)
		if details != nil {
			return FormatConnectionError(details)
		}
	}

	// Handle generic "Connection error." message
	if message == "Connection error." {
		if code != "" {
			return fmt.Sprintf("Unable to connect to API (%s)", code)
		}
		return "Unable to connect to API. Check your internet connection"
	}

	// No message — fallback
	if message == "" {
		return fmt.Sprintf("API error (status %d)", statusCode)
	}

	// Try to sanitize HTML
	sanitized := SanitizeHTMLError(message)
	if sanitized != message && sanitized != "" {
		return sanitized
	}

	return message
}

// formatSSLError returns a specific message for an SSL error code.
func formatSSLError(code string) string {
	switch SSLErrorCode(code) {
	case SSLUnableToVerifyLeaf, SSLUnableToGetIssuer, SSLUnableToGetIssuerLocal:
		return "Unable to connect to API: SSL certificate verification failed. Check your proxy or corporate SSL certificates"
	case SSLCertExpired:
		return "Unable to connect to API: SSL certificate has expired"
	case SSLCertRevoked:
		return "Unable to connect to API: SSL certificate has been revoked"
	case SSLSelfSigned, SSLSelfSignedInChain:
		return "Unable to connect to API: Self-signed certificate detected. Check your proxy or corporate SSL certificates"
	case SSLAltNameInvalid, SSLHostnameMismatch:
		return "Unable to connect to API: SSL certificate hostname mismatch"
	case SSLCertNotYetValid:
		return "Unable to connect to API: SSL certificate is not yet valid"
	default:
		return fmt.Sprintf("Unable to connect to API: SSL error (%s)", code)
	}
}
