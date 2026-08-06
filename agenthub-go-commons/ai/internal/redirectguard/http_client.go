package redirectguard

import (
	"errors"
	"net/http"
	"time"
)

var errRedirectNotAllowed = errors.New("HTTP redirects are not allowed for model providers")

// NewHTTPClient returns a client that never follows redirects from model providers.
// The configured provider base URL is validated by the API before provider creation.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return errRedirectNotAllowed
		},
	}
}

// WithTransport clones client so its redirect policy and timeout are retained.
func WithTransport(client *http.Client, transport http.RoundTripper) *http.Client {
	clone := *client
	clone.Transport = transport
	return &clone
}
