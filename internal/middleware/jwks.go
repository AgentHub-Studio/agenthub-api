package middleware

import (
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

const jwksCacheTTL = 5 * time.Minute

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
	url := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", keycloakBaseURL, realm)
	resp, err := http.Get(url) //nolint:noctx // JWKS fetch uses a short-lived background request
	if err != nil {
		return nil, fmt.Errorf("jwks: fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks: fetch %s returned HTTP %d", url, resp.StatusCode)
	}

	var doc jwksDoc
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("jwks: decode response from %s: %w", url, err)
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
