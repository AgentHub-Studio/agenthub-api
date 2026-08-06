package knowledge

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPgDocumentSearchClient_BlocksRedirectToPrivateEndpoint(t *testing.T) {
	client := NewPgDocumentSearchClient(nil, "https://1.1.1.1")
	requests := 0
	client.httpClient = &http.Client{Transport: pgSearchRoundTripper(func(req *http.Request) (*http.Response, error) {
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

	_, err := client.embed(context.Background(), "hello")

	require.Error(t, err)
	require.Equal(t, 1, requests, "a redirect to metadata must not make a second request")
}

type pgSearchRoundTripper func(*http.Request) (*http.Response, error)

func (f pgSearchRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
