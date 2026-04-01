// Package contracts implements regression tests that verify REST API contracts
// using structural snapshot testing.
//
// Snapshots are stored in testdata/snapshots/ and represent the expected
// JSON structure (field names and types) for each endpoint.
//
// Run with: CONTRACT_TESTS=1 GO_URL=http://localhost:8081 go test ./...
package contracts

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-contracts/client"
	"github.com/AgentHub-Studio/agenthub-contracts/snapshot"
)

func skipIfNoContracts(t *testing.T) {
	t.Helper()
	if os.Getenv("CONTRACT_TESTS") == "" {
		t.Skip("set CONTRACT_TESTS=1 to run contract tests")
	}
}

func goAPIClient() *client.Client {
	return client.New(getEnv("GO_URL", "http://localhost:9080"), os.Getenv("AUTH_TOKEN"))
}

// endpointCase describes a single snapshot test case.
type endpointCase struct {
	name       string
	path       string
	wantStatus int
}

func runSnapshotCases(t *testing.T, c *client.Client, cases []endpointCase) {
	t.Helper()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			body, status, err := c.Get(tc.path)
			require.NoError(t, err, "GET %s failed", tc.path)

			expectedStatus := tc.wantStatus
			if expectedStatus == 0 {
				expectedStatus = http.StatusOK
			}
			require.Equal(t, expectedStatus, status, "GET %s status", tc.path)

			if status == http.StatusOK {
				snapshotName := strings.ReplaceAll(tc.name, " ", "_")
				snapshot.Assert(t, snapshotName, body)
			}
		})
	}
}

func TestRegression_Agents(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list agents", path: "/api/agents?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_Pipeline(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list pipelines", path: "/api/pipelines?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_Skills(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list skills", path: "/api/skills?page=0&size=5"},
		{name: "list tools", path: "/api/tools?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_KnowledgeBase(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list knowledge bases", path: "/api/knowledge-bases?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_Marketplace(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list marketplace listings", path: "/api/marketplace/listings?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_Registry(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list packages", path: "/api/packages?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_LLMPresets(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list llm presets", path: "/api/llm-config-presets?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_MCP(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list mcp servers", path: "/api/mcp/servers?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_Chat(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list chat sessions", path: "/api/chat/sessions?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_Execution(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list executions", path: "/api/executions?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_Audit(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list audit logs", path: "/api/audit-logs?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_Metrics(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "tenant metrics", path: "/api/metrics/tenant"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_Experiments(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list experiments", path: "/api/experiments?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_VPN(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list vpn resources", path: "/api/vpn-resources?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_DataSources(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	cases := []endpointCase{
		{name: "list datasources", path: "/api/datasources?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_Tenants(t *testing.T) {
	skipIfNoContracts(t)
	// Public endpoint — no auth required.
	c := client.New(getEnv("GO_URL", "http://localhost:9080"), "")

	cases := []endpointCase{
		{name: "list tenants", path: "/public/tenants?page=0&size=5"},
	}
	runSnapshotCases(t, c, cases)
}

func TestRegression_NotFound(t *testing.T) {
	skipIfNoContracts(t)
	c := goAPIClient()

	nilUUID := "00000000-0000-0000-0000-000000000000"
	cases := []endpointCase{
		{name: "agent not found", path: fmt.Sprintf("/api/agents/%s", nilUUID), wantStatus: http.StatusNotFound},
		{name: "pipeline not found", path: fmt.Sprintf("/api/pipelines/%s", nilUUID), wantStatus: http.StatusNotFound},
		{name: "skill not found", path: fmt.Sprintf("/api/skills/%s", nilUUID), wantStatus: http.StatusNotFound},
		{name: "knowledge base not found", path: fmt.Sprintf("/api/knowledge-bases/%s", nilUUID), wantStatus: http.StatusNotFound},
	}
	runSnapshotCases(t, c, cases)
}
