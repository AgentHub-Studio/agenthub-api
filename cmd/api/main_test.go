package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunHealthCheckUsesPortEnvironment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/health", r.URL.Path)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	port := strings.TrimPrefix(server.URL, "http://127.0.0.1:")
	t.Setenv("PORT", port)

	require.Equal(t, 0, runHealthCheck())
}

func TestRunHealthCheckFailsOnUnavailableEndpoint(t *testing.T) {
	t.Setenv("PORT", "1")

	require.Equal(t, 1, runHealthCheck())
}

func TestHealthCheckEndpointDefaultsToLoopback(t *testing.T) {
	endpoint, ok := healthCheckEndpoint("")

	require.True(t, ok)
	require.Equal(t, "http://127.0.0.1:8081/health", endpoint)
}

func TestHealthCheckEndpointRejectsUnsafePorts(t *testing.T) {
	cases := []string{
		"0",
		"65536",
		"8081/health",
		"8081\n",
		"http://127.0.0.1:8081",
	}

	for _, rawPort := range cases {
		t.Run(rawPort, func(t *testing.T) {
			endpoint, ok := healthCheckEndpoint(rawPort)

			require.False(t, ok)
			require.Empty(t, endpoint)
		})
	}
}
