package webhook

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type webhookRoundTripper func(*http.Request) (*http.Response, error)

func (f webhookRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestWebhookHTTPClient_BlocksRedirectToSSRFURL(t *testing.T) {
	const originURL = "https://hooks.example.test/events"
	const blockedURL = "http://169.254.169.254/latest/meta-data"

	targetReached := false
	transport := webhookRoundTripper(func(req *http.Request) (*http.Response, error) {
		switch req.URL.String() {
		case originURL:
			return &http.Response{
				StatusCode: http.StatusFound,
				Header:     http.Header{"Location": []string{blockedURL}},
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		case blockedURL:
			targetReached = true
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader("")),
				Request:    req,
			}, nil
		default:
			return nil, assert.AnError
		}
	})

	client := newWebhookHTTPClient()
	client.Transport = transport
	status, _, err := doHTTPPost(client, originURL, []byte(`{}`))

	assert.NoError(t, err)
	assert.Equal(t, http.StatusFound, status)
	assert.False(t, targetReached, "redirect target must not be reached")
}
