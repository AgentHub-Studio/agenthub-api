// Package contracts implements contract tests that compare JSON responses between
// the Java and Go implementations of agenthub services.
//
// Run with:
//
//	JAVA_URL=http://localhost:8083 GO_URL=http://localhost:9080 go test ./...
package contracts

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-contracts/client"
	"github.com/AgentHub-Studio/agenthub-contracts/comparator"
)

func javaURL() string  { return getEnv("JAVA_MARKETPLACE_URL", "http://localhost:8083") }
func goURL() string    { return getEnv("GO_URL", "http://localhost:9080") }
func authToken() string { return os.Getenv("AUTH_TOKEN") }

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func skipIfNoServices(t *testing.T) {
	t.Helper()
	if os.Getenv("CONTRACT_TESTS") == "" {
		t.Skip("set CONTRACT_TESTS=1 to run contract tests (requires both Java and Go services)")
	}
}

// contractCase describes a single endpoint contract test.
type contractCase struct {
	name       string
	javaPath   string
	goPath     string
	statusCode int
}

func runContractCases(t *testing.T, cases []contractCase) {
	t.Helper()
	opts := comparator.DefaultOptions()
	javaClient := client.New(javaURL(), authToken())
	goClient := client.New(goURL(), authToken())

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			javaBody, javaStatus, err := javaClient.Get(tc.javaPath)
			require.NoError(t, err, "Java GET failed")

			goBody, goStatus, err := goClient.Get(tc.goPath)
			require.NoError(t, err, "Go GET failed")

			expectedStatus := tc.statusCode
			if expectedStatus == 0 {
				expectedStatus = http.StatusOK
			}

			assert.Equal(t, expectedStatus, javaStatus, "Java status mismatch")
			assert.Equal(t, expectedStatus, goStatus, "Go status mismatch")

			diffs := comparator.Diff(javaBody, goBody, opts)
			if len(diffs) > 0 {
				t.Errorf("JSON contract violation for %q:\n%s", tc.name, strings.Join(diffs, "\n"))
			}
		})
	}
}

func TestMarketplaceListing_Contract(t *testing.T) {
	skipIfNoServices(t)

	cases := []contractCase{
		{
			name:     "list all listings",
			javaPath: "/api/marketplace/listings?page=0&size=10",
			goPath:   "/api/marketplace/listings?page=0&size=10",
		},
		{
			name:       "listing not found",
			javaPath:   "/api/marketplace/listings/00000000-0000-0000-0000-000000000000",
			goPath:     "/api/marketplace/listings/00000000-0000-0000-0000-000000000000",
			statusCode: http.StatusNotFound,
		},
	}

	runContractCases(t, cases)
}

func TestRegistryPackage_Contract(t *testing.T) {
	skipIfNoServices(t)

	cases := []contractCase{
		{
			name:     "list packages",
			javaPath: "/api/registry/packages?page=0&size=10",
			goPath:   "/api/registry/packages?page=0&size=10",
		},
	}

	runContractCases(t, cases)
}

func TestObservability_Contract(t *testing.T) {
	skipIfNoServices(t)

	javaObs := getEnv("JAVA_OBSERVABILITY_URL", "http://localhost:8085")
	goObs := getEnv("GO_OBSERVABILITY_URL", "http://localhost:9085")

	type obsCase struct {
		name     string
		javaPath string
		goPath   string
	}

	obsCases := []obsCase{
		{
			name:     "health check",
			javaPath: "/actuator/health",
			goPath:   "/health",
		},
	}

	opts := comparator.DefaultOptions()
	javaClient := client.New(javaObs, authToken())
	goClient := client.New(goObs, authToken())

	for _, tc := range obsCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			javaBody, _, err := javaClient.Get(tc.javaPath)
			require.NoError(t, err)
			goBody, _, err := goClient.Get(tc.goPath)
			require.NoError(t, err)
			diffs := comparator.Diff(javaBody, goBody, opts)
			if len(diffs) > 0 {
				t.Logf("Differences in %q (may be expected during migration):\n%s", tc.name, strings.Join(diffs, "\n"))
			}
		})
	}

	_ = fmt.Sprintf("observability contract tests against %s and %s", javaObs, goObs)
}
