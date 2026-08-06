package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const jwksCacheTTL = 5 * time.Minute
const jwksFetchTimeout = 10 * time.Second

// jwksCache holds per-realm RSA public keys fetched from Keycloak.
// On cache miss or TTL expiry, keys are refetched from the JWKS endpoint.
type jwksCache struct {
	mu    sync.RWMutex
	store map[string]*jwksCacheEntry
}

type jwksCacheEntry struct {
	keys      map[string]*rsa.PublicKey // keyed by "kid"
	fetchedAt time.Time
}

// globalJWKSCache is the process-level JWKS cache.
var globalJWKSCache = &jwksCache{
	store: make(map[string]*jwksCacheEntry),
}

// get returns cached keys for a realm if they are still fresh.
func (c *jwksCache) get(realm string) (map[string]*rsa.PublicKey, bool) {
	c.mu.RLock()
	entry, ok := c.store[realm]
	c.mu.RUnlock()
	if !ok || time.Since(entry.fetchedAt) >= jwksCacheTTL {
		return nil, false
	}
	return entry.keys, true
}

// set stores keys for a realm.
func (c *jwksCache) set(realm string, keys map[string]*rsa.PublicKey) {
	c.mu.Lock()
	c.store[realm] = &jwksCacheEntry{keys: keys, fetchedAt: time.Now()}
	c.mu.Unlock()
}

// jwksDoc is the JSON structure returned by Keycloak's certs endpoint.
type jwksDoc struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// fetchRealmJWKS fetches and parses RSA signing keys from a Keycloak realm's certs endpoint.
// The keycloakBaseURL parameter allows injection of a test server URL in unit tests.
func fetchRealmJWKS(keycloakBaseURL, realm string) (map[string]*rsa.PublicKey, error) {
	jwksURL, err := buildRealmJWKSURL(keycloakBaseURL, realm)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), jwksFetchTimeout)
	defer cancel()
	// #nosec G704 -- jwksURL is built from a trusted Keycloak base URL and an escaped realm segment.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, fmt.Errorf("jwks: build request for %s: %w", jwksURL, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jwks: fetch %s: %w", jwksURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks: fetch %s returned HTTP %d", jwksURL, resp.StatusCode)
	}

	var doc jwksDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("jwks: decode response from %s: %w", jwksURL, err)
	}

	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
		if k.Kty != "RSA" || k.Use != "sig" {
			continue
		}
		pub, err := jwkToRSA(k)
		if err != nil {
			// Skip unreadable keys; log would be ideal but we avoid logger coupling here.
			continue
		}
		keys[k.Kid] = pub
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("jwks: no usable RSA signing keys found for realm %q", realm)
	}
	return keys, nil
}

func buildRealmJWKSURL(keycloakBaseURL, realm string) (string, error) {
	parsed, err := parseTrustedHTTPBaseURL(keycloakBaseURL, "jwks: invalid Keycloak base URL")
	if err != nil {
		return "", err
	}
	if realm == "" || strings.Contains(realm, "/") || containsControlChar(realm) {
		return "", fmt.Errorf("jwks: invalid realm %q", realm)
	}
	return appendEscapedPathSegments(parsed, "realms", realm, "protocol", "openid-connect", "certs")
}

func parseTrustedHTTPBaseURL(rawBase, message string) (*url.URL, error) {
	if rawBase == "" || containsControlChar(rawBase) {
		return nil, fmt.Errorf("%s: empty or unsafe base URL", message)
	}
	parsed, err := url.Parse(strings.TrimRight(rawBase, "/"))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", message, err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("%s: unsupported scheme %q", message, parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("%s: missing host", message)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, fmt.Errorf("%s: userinfo, query and fragment are not allowed", message)
	}
	return parsed, nil
}

func appendEscapedPathSegments(base *url.URL, segments ...string) (string, error) {
	escapedPath := strings.TrimSuffix(base.EscapedPath(), "/")
	for _, segment := range segments {
		if segment == "" || containsControlChar(segment) {
			return "", fmt.Errorf("invalid URL path segment")
		}
		escapedPath += "/" + url.PathEscape(segment)
	}
	if escapedPath == "" {
		escapedPath = "/"
	}
	unescapedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return "", err
	}
	next := *base
	next.Path = unescapedPath
	next.RawPath = escapedPath
	return next.String(), nil
}

func containsControlChar(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// jwkToRSA converts a JWK entry into an *rsa.PublicKey.
func jwkToRSA(k jwkKey) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("jwks: decode n for kid=%q: %w", k.Kid, err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("jwks: decode e for kid=%q: %w", k.Kid, err)
	}

	n := new(big.Int).SetBytes(nBytes)
	var e int
	for _, b := range eBytes {
		e = e*256 + int(b)
	}

	return &rsa.PublicKey{N: n, E: e}, nil
}

// getRealmKeys returns the cached (or freshly fetched) signing keys for a realm.
func getRealmKeys(keycloakBaseURL, realm string) (map[string]*rsa.PublicKey, error) {
	if keys, ok := globalJWKSCache.get(realm); ok {
		return keys, nil
	}
	keys, err := fetchRealmJWKS(keycloakBaseURL, realm)
	if err != nil {
		return nil, err
	}
	globalJWKSCache.set(realm, keys)
	return keys, nil
}
