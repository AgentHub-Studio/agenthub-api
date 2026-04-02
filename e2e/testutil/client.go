package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

// APIClient is a test HTTP client for agenthub-api.
type APIClient struct {
	BaseURL string
	Token   string
	http    *http.Client
	t       *testing.T
}

// NewAPIClient creates a new test client.
func NewAPIClient(t *testing.T, baseURL, token string) *APIClient {
	return &APIClient{
		BaseURL: baseURL,
		Token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
		t:       t,
	}
}

// Get performs GET and decodes the JSON response into result.
func (c *APIClient) Get(path string, result any) int {
	c.t.Helper()
	return c.do(http.MethodGet, path, nil, result)
}

// Post performs POST with body and decodes the JSON response.
func (c *APIClient) Post(path string, body, result any) int {
	c.t.Helper()
	return c.doJSON(http.MethodPost, path, body, result)
}

// Put performs PUT with body.
func (c *APIClient) Put(path string, body, result any) int {
	c.t.Helper()
	return c.doJSON(http.MethodPut, path, body, result)
}

// Patch performs PATCH with body.
func (c *APIClient) Patch(path string, body, result any) int {
	c.t.Helper()
	return c.doJSON(http.MethodPatch, path, body, result)
}

// Delete performs DELETE.
func (c *APIClient) Delete(path string) int {
	c.t.Helper()
	return c.do(http.MethodDelete, path, nil, nil)
}

func (c *APIClient) doJSON(method, path string, body, result any) int {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal body: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	return c.do(method, path, bodyReader, result)
}

func (c *APIClient) do(method, path string, body io.Reader, result any) int {
	req, err := http.NewRequest(method, c.BaseURL+path, body)
	if err != nil {
		c.t.Fatalf("build request: %v", err)
	}
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("do request %s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			c.t.Logf("decode response body: %v", err)
		}
	}
	return resp.StatusCode
}

// MustEqual asserts status code and fails test if not matching.
func (c *APIClient) MustEqual(expected, actual int, msg string) {
	c.t.Helper()
	if expected != actual {
		c.t.Errorf("%s: expected status %d, got %d", msg, expected, actual)
	}
}

// Page is the pagination response wrapper.
type Page[T any] struct {
	Content       []T `json:"content"`
	TotalElements int `json:"totalElements"`
	Page          int `json:"page"`
	Size          int `json:"size"`
}

// ErrorResponse is a standard API error response.
type ErrorResponse struct {
	Error string `json:"error"`
}

// IDResponse is used for resources that return just an ID.
type IDResponse struct {
	ID string `json:"id"`
}

// FormatURL formats a URL with a path value, e.g. "/api/agents/%s" + id.
func FormatURL(pattern string, args ...any) string {
	return fmt.Sprintf(pattern, args...)
}
