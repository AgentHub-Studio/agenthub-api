// Package client provides a simple HTTP client for contract tests.
package client

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client wraps http.Client with a base URL.
type Client struct {
	baseURL string
	http    *http.Client
	token   string
}

// New creates a Client targeting baseURL with an optional bearer token.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// Get performs GET {path} and returns the response body bytes and status code.
func (c *Client) Get(path string) ([]byte, int, error) {
	return c.do(http.MethodGet, path, nil)
}

func (c *Client) do(method, path string, body io.Reader) ([]byte, int, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, 0, fmt.Errorf("client: build request: %w", err)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("client: do request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("client: read body: %w", err)
	}
	return data, resp.StatusCode, nil
}
