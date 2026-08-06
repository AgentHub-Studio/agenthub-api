package approval

import "testing"

func TestApprovalTenantSearchPathQuotesTenantSchema(t *testing.T) {
	if got, want := approvalTenantSearchPath("e2e-tenant-123"), `"ah_e2e-tenant-123", public`; got != want {
		t.Fatalf("unexpected search path: got %q, want %q", got, want)
	}
}
