package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"
)

type jwksCache struct {
	mu      sync.RWMutex
	entries map[string]*jwksCacheEntry
	ttl     time.Duration
	baseURL string
}

type jwksCacheEntry struct {
	keys      map[string]*rsa.PublicKey // kid -> key
	fetchedAt time.Time
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func newJWKSCache(keycloakBaseURL string, ttl time.Duration) *jwksCache {
	if ttl == 0 {
		ttl = 5 * time.Minute
	}
	return &jwksCache{
		entries: make(map[string]*jwksCacheEntry),
		ttl:     ttl,
		baseURL: keycloakBaseURL,
	}
}

// getKey returns the RSA public key for the given tenantID and kid.
func (c *jwksCache) getKey(ctx context.Context, tenantID, kid string) (*rsa.PublicKey, error) {
	c.mu.RLock()
	entry, ok := c.entries[tenantID]
	c.mu.RUnlock()

	if ok && time.Since(entry.fetchedAt) < c.ttl {
		if key, found := entry.keys[kid]; found {
			return key, nil
		}
	}

	// Fetch and cache
	keys, err := c.fetchJWKS(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.entries[tenantID] = &jwksCacheEntry{keys: keys, fetchedAt: time.Now()}
	c.mu.Unlock()

	key, found := keys[kid]
	if !found {
		return nil, fmt.Errorf("auth: kid %q not found in JWKS for tenant %q", kid, tenantID)
	}
	return key, nil
}

func (c *jwksCache) fetchJWKS(ctx context.Context, tenantID string) (map[string]*rsa.PublicKey, error) {
	url := fmt.Sprintf("%s/realms/%s/protocol/openid-connect/certs", c.baseURL, tenantID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("auth: create JWKS request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("auth: fetch JWKS for tenant %q: %w", tenantID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth: JWKS endpoint returned %d for tenant %q", resp.StatusCode, tenantID)
	}

	var jwksResp jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwksResp); err != nil {
		return nil, fmt.Errorf("auth: decode JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwksResp.Keys))
	for _, k := range jwksResp.Keys {
		if k.Kty != "RSA" {
			continue
		}
		pub, err := parseRSAPublicKey(k)
		if err != nil {
			continue
		}
		keys[k.Kid] = pub
	}
	return keys, nil
}

func parseRSAPublicKey(k jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("auth: decode RSA N: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("auth: decode RSA E: %w", err)
	}

	n := new(big.Int).SetBytes(nBytes)
	e := new(big.Int).SetBytes(eBytes)

	return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
}
