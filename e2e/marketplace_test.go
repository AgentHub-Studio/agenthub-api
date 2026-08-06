//go:build e2e

package e2e

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-e2e/testutil"
)

// TestE2E_MarketplaceListingLifecycle validates full CRUD for marketplace listings.
func TestE2E_MarketplaceListingLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)
	packageSlug := "marketplace-pkg-" + tenant.Slug
	listingSlug := "marketplace-listing-" + tenant.Slug

	// First create a registry package so the listing can reference it
	var pkg map[string]any
	status := c.Post("/api/packages", map[string]any{
		"name":        packageSlug,
		"slug":        packageSlug,
		"description": "Package for marketplace E2E",
		"type":        "AGENT",
		"visibility":  "PUBLIC",
	}, &pkg)
	require.Equal(t, http.StatusCreated, status)
	pkgID := pkg["id"].(string)

	// --- Create listing ---
	var listing map[string]any
	status = c.Post("/api/marketplace/listings", map[string]any{
		"packageId":   pkgID,
		"name":        "E2E Test Agent",
		"slug":        listingSlug,
		"description": "A test agent listing for E2E",
		"type":        "AGENT",
		"category":    "productivity",
	}, &listing)
	require.Equal(t, http.StatusCreated, status)
	assert.Equal(t, "E2E Test Agent", listing["name"])
	listingID, ok := listing["id"].(string)
	require.True(t, ok)

	// --- Get by ID ---
	var fetched map[string]any
	status = c.Get("/api/marketplace/listings/"+listingID, &fetched)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "E2E Test Agent", fetched["name"])

	// --- List (public endpoint — filter by type) ---
	var page testutil.Page[map[string]any]
	status = c.Get("/api/marketplace/listings?type=AGENT", &page)
	assert.Equal(t, http.StatusOK, status)

	// --- Update ---
	var updated map[string]any
	status = c.Put("/api/marketplace/listings/"+listingID, map[string]any{
		"name": "E2E Test Agent Updated",
	}, &updated)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "E2E Test Agent Updated", updated["name"])

	// --- Delete ---
	status = c.Delete("/api/marketplace/listings/" + listingID)
	assert.Equal(t, http.StatusNoContent, status)

	var removed map[string]any
	status = c.Get("/api/marketplace/listings/"+listingID, &removed)
	assert.Equal(t, http.StatusOK, status)
	assert.Equal(t, "REMOVED", removed["status"])
}

// TestE2E_MarketplaceReviewLifecycle validates review CRUD under a listing.
func TestE2E_MarketplaceReviewLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)
	packageSlug := "marketplace-review-pkg-" + tenant.Slug
	listingSlug := "marketplace-review-listing-" + tenant.Slug

	// Setup: installable package
	var pkg map[string]any
	status := c.Post("/api/packages", map[string]any{
		"name": packageSlug, "slug": packageSlug,
		"description": "pkg", "type": "AGENT", "visibility": "PUBLIC",
	}, &pkg)
	require.Equal(t, http.StatusCreated, status)
	pkgID := pkg["id"].(string)

	var listing map[string]any
	status = c.Post("/api/marketplace/listings", map[string]any{
		"packageId": pkgID, "name": "Reviewed Agent", "slug": listingSlug,
		"description": "d", "type": "AGENT", "category": "test",
	}, &listing)
	require.Equal(t, http.StatusCreated, status)
	listingID := listing["id"].(string)

	reviewBase := "/api/marketplace/listings/" + listingID + "/reviews"

	// --- Create review ---
	var review map[string]any
	status = c.Post(reviewBase, map[string]any{
		"rating":  5,
		"comment": "Excellent agent!",
	}, &review)
	require.Equal(t, http.StatusCreated, status)
	reviewID := review["id"].(string)

	// --- List reviews ---
	var reviewPage testutil.Page[map[string]any]
	status = c.Get(reviewBase+"?page=0&size=10", &reviewPage)
	assert.Equal(t, http.StatusOK, status)
	assert.GreaterOrEqual(t, reviewPage.TotalElements, 1)

	// --- Delete review ---
	status = c.Delete(reviewBase + "/" + reviewID)
	assert.Equal(t, http.StatusNoContent, status)
}

// TestE2E_MarketplaceInstallationLifecycle validates install and uninstall.
func TestE2E_MarketplaceInstallationLifecycle(t *testing.T) {
	cfg := e2eConfig()
	tenant := testutil.NewTenantFixture(t,
		cfg.backendURL, cfg.keycloakURL,
		cfg.keycloakAdmin, cfg.keycloakAdminPass,
		cfg.e2eUserPassword,
	)
	c := tenant.Client(t, cfg.backendURL)
	packageSlug := "marketplace-install-pkg-" + tenant.Slug

	// Setup: installable package
	var pkg map[string]any
	status := c.Post("/api/packages", map[string]any{
		"name": packageSlug, "slug": packageSlug,
		"description": "pkg", "type": "AGENT", "visibility": "PUBLIC",
	}, &pkg)
	require.Equal(t, http.StatusCreated, status)
	pkgID := pkg["id"].(string)

	// --- Install ---
	var installation map[string]any
	status = c.Post("/api/marketplace/installations", map[string]any{
		"packageId":      pkgID,
		"packageVersion": "1.0.0",
	}, &installation)
	require.Equal(t, http.StatusCreated, status)
	installID := installation["id"].(string)

	// --- List installations ---
	var installPage testutil.Page[map[string]any]
	status = c.Get("/api/marketplace/installations?page=0&size=10", &installPage)
	assert.Equal(t, http.StatusOK, status)
	assert.GreaterOrEqual(t, installPage.TotalElements, 1)

	// --- Uninstall ---
	status = c.Delete("/api/marketplace/installations/" + installID)
	assert.Equal(t, http.StatusNoContent, status)
}
