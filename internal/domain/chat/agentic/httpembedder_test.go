package agentic

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHTTPEmbedder_DefaultTimeoutIs60s(t *testing.T) {
	// The 30s default was too tight for e5-large on CPU (~28s per embed).
	// Locking the new default in a test so regressions are caught fast.
	e := NewHTTPEmbedder("http://example")
	if e.httpClient.Timeout != 60*time.Second {
		t.Errorf("default timeout = %v, want 60s", e.httpClient.Timeout)
	}
}

func TestHTTPEmbedder_Embed_Success(t *testing.T) {
	var gotPath, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		_, _ = fmt.Fprint(w, `{"embedding":[0.1,0.2,0.3]}`)
	}))
	defer srv.Close()

	vec, err := NewHTTPEmbedder(srv.URL).Embed(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(vec) != 3 || vec[0] != 0.1 {
		t.Errorf("vector=%v, want [0.1 0.2 0.3]", vec)
	}
	if gotPath != "/embed" {
		t.Errorf("path=%q, want /embed", gotPath)
	}
	if !strings.Contains(gotBody, `"text":"hello"`) {
		t.Errorf("body=%q should carry text field", gotBody)
	}
}

func TestHTTPEmbedder_Embed_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := NewHTTPEmbedder(srv.URL).Embed(context.Background(), "hi")
	if err == nil {
		t.Fatalf("expected error on 500")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("error=%q should mention HTTP 500", err.Error())
	}
}

func TestHTTPEmbedder_Embed_EmptyVector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"embedding":[]}`)
	}))
	defer srv.Close()

	_, err := NewHTTPEmbedder(srv.URL).Embed(context.Background(), "hi")
	if err == nil || !strings.Contains(err.Error(), "empty vector") {
		t.Errorf("expected empty-vector error, got %v", err)
	}
}

func TestHTTPEmbedder_Embed_TimeoutOverride(t *testing.T) {
	// Server that never replies; client with 50ms timeout must fail quickly.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	e := NewHTTPEmbedderWithTimeout(srv.URL, 50*time.Millisecond)
	start := time.Now()
	_, err := e.Embed(context.Background(), "x")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatalf("expected timeout error")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("timeout took %v; custom 50ms timeout was not respected", elapsed)
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("error=%q should be wrapped 'request failed'", err.Error())
	}
}

func TestHTTPEmbedder_Embed_ContextCancellation(t *testing.T) {
	// Ensure the caller's context cancel aborts the request, even if the
	// embedder's own Client.Timeout is long.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()
	_, err := NewHTTPEmbedder(srv.URL).Embed(ctx, "x")
	if err == nil {
		t.Fatalf("expected context error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "context deadline") {
		t.Errorf("error=%q should reflect context cancellation", err.Error())
	}
}

func TestHTTPEmbedder_BlocksRedirectToPrivateEndpoint(t *testing.T) {
	embedder := NewHTTPEmbedder("https://1.1.1.1")
	requests := 0
	embedder.httpClient = &http.Client{Transport: httpEmbedderRoundTripper(func(req *http.Request) (*http.Response, error) {
		requests++
		if requests == 1 {
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{"http://169.254.169.254/latest/meta-data"}},
				Body:       io.NopCloser(strings.NewReader("redirect")),
				Request:    req,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"embedding":[0.1]}`)),
			Request:    req,
		}, nil
	})}

	_, err := embedder.Embed(context.Background(), "hello")

	if err == nil {
		t.Fatal("expected redirect to metadata to be blocked")
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

type httpEmbedderRoundTripper func(*http.Request) (*http.Response, error)

func (f httpEmbedderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
