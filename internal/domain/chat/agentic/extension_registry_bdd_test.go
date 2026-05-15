package agentic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBDD_ExtensionRegistry(t *testing.T) {
	t.Run("Scenario_AdminInstallsMarketplaceExtensionAndItStartsPendingReview", func(t *testing.T) {
		// Given a marketplace extension with a valid signed checksum,
		// When admin installs without specifying status,
		// Then the registry stores it as pending_review (admin must
		// explicitly enable — never auto-trust marketplace bundles).
		r := NewInMemoryExtensionRegistry()
		d := validExtensionDescriptor()
		saved, err := r.Install(context.Background(), d)
		require.NoError(t, err)
		assert.Equal(t, ExtensionStatusPendingReview, saved.Status)
		assert.True(t, saved.EnabledAt.IsZero())
	})

	t.Run("Scenario_BuiltinExtensionsCanBeAutoEnabledAtInstall", func(t *testing.T) {
		// Given a builtin extension shipped with the platform,
		// When the platform installs it with status=enabled,
		// Then EnabledAt is set immediately (no admin gate for builtins).
		r := NewInMemoryExtensionRegistry()
		d := validExtensionDescriptor()
		d.Slug = "builtin-pack"
		d.Source = ExtensionSourceBuiltin
		d.Checksum = ""
		d.Status = ExtensionStatusEnabled
		saved, err := r.Install(context.Background(), d)
		require.NoError(t, err)
		assert.False(t, saved.EnabledAt.IsZero())
	})

	t.Run("Scenario_UntrustedSourceWithoutChecksumIsRejected", func(t *testing.T) {
		// Given a marketplace/git/url extension submission,
		// When the bundle has no checksum,
		// Then install fails (cannot verify integrity later → reject up-front).
		r := NewInMemoryExtensionRegistry()
		d := validExtensionDescriptor()
		d.Source = ExtensionSourceGit
		d.SourceRef = "https://github.com/x/y"
		d.Checksum = ""
		_, err := r.Install(context.Background(), d)
		assert.Error(t, err)
	})

	t.Run("Scenario_DuplicateInstallSameTenantIsRejected", func(t *testing.T) {
		// Given an extension is already installed in tenant-a,
		// When admin tries to install the same slug again,
		// Then registry rejects (must uninstall first to upgrade).
		r := NewInMemoryExtensionRegistry()
		_, _ = r.Install(context.Background(), validExtensionDescriptor())
		_, err := r.Install(context.Background(), validExtensionDescriptor())
		assert.Error(t, err)
	})

	t.Run("Scenario_TenantIsolationLetsTwoTenantsRunSameExtensionIndependently", func(t *testing.T) {
		// Given tenant-a installs ext X,
		// When tenant-b installs ext X,
		// Then both succeed (tenant isolation: separate copies, separate
		// lifecycle — quarantining in tenant-a doesn't affect tenant-b).
		r := NewInMemoryExtensionRegistry()
		_, err := r.Install(context.Background(), validExtensionDescriptor())
		require.NoError(t, err)
		d2 := validExtensionDescriptor()
		d2.TenantID = "tenant-b"
		_, err = r.Install(context.Background(), d2)
		assert.NoError(t, err)
	})

	t.Run("Scenario_QuarantineIsStickyAndRequiresAdminResetToRecover", func(t *testing.T) {
		// Given an extension started misbehaving (signature failed at
		// verify-time, repeated runtime errors, security flag),
		// When the platform quarantines it,
		// Then a regular Enable call is BLOCKED — only AdminResetQuarantine
		// can clear (mirrors FUTURE-002 mistrusted contract: trust loss
		// is harder to recover from than to lose).
		r := NewInMemoryExtensionRegistry()
		saved, _ := r.Install(context.Background(), validExtensionDescriptor())
		_, _ = r.Quarantine(context.Background(), saved.TenantID, saved.Slug, "signature failed at verify time")

		_, err := r.Enable(context.Background(), saved.TenantID, saved.Slug)
		require.Error(t, err, "Enable must be blocked while quarantined")

		// Admin reset returns to disabled (NOT enabled) — admin must
		// then make a separate explicit Enable decision.
		cleared, err := r.AdminResetQuarantine(context.Background(), saved.TenantID, saved.Slug)
		require.NoError(t, err)
		assert.Equal(t, ExtensionStatusDisabled, cleared.Status,
			"admin reset returns to disabled — re-enable is a separate decision")
	})

	t.Run("Scenario_QuarantineRequiresExplicitReasonForAuditTrail", func(t *testing.T) {
		// Given platform writes governance events when quarantining,
		// When the caller provides no reason,
		// Then quarantine fails (no silent quarantines — every transition
		// must be auditable per GOV-001).
		r := NewInMemoryExtensionRegistry()
		saved, _ := r.Install(context.Background(), validExtensionDescriptor())
		_, err := r.Quarantine(context.Background(), saved.TenantID, saved.Slug, "   ")
		assert.Error(t, err)
	})

	t.Run("Scenario_RuntimeDiscoversWhichExtensionsProvideASkillByListByComponent", func(t *testing.T) {
		// Given the agentic runtime needs to find which installed
		// extension provides a skill,
		// When it queries ListByComponent(skills),
		// Then only extensions claiming the kind are returned (avoids
		// scanning every extension's component map at request time).
		r := NewInMemoryExtensionRegistry()
		d1 := validExtensionDescriptor()
		d1.Slug = "skills-pack"
		d1.Components = map[ExtensionComponentKind][]string{
			ExtensionComponentSkills: {"web-search"},
		}
		_, _ = r.Install(context.Background(), d1)

		d2 := validExtensionDescriptor()
		d2.Slug = "tools-pack"
		d2.Components = map[ExtensionComponentKind][]string{
			ExtensionComponentTools: {"http-fetch"},
		}
		_, _ = r.Install(context.Background(), d2)

		got, err := r.ListByComponent(context.Background(), "tenant-a", ExtensionComponentSkills)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "skills-pack", got[0].Slug)
	})

	t.Run("Scenario_DisableIsIdempotentForRetryableProvisioning", func(t *testing.T) {
		// Given the provisioning pipeline retries Disable on transient
		// upstream failures,
		// When Disable is called twice,
		// Then both calls succeed and produce the same final status
		// (no spurious "already disabled" errors that retry loops have
		// to special-case).
		r := NewInMemoryExtensionRegistry()
		saved, _ := r.Install(context.Background(), validExtensionDescriptor())
		_, _ = r.Enable(context.Background(), saved.TenantID, saved.Slug)
		d1, err := r.Disable(context.Background(), saved.TenantID, saved.Slug)
		require.NoError(t, err)
		d2, err := r.Disable(context.Background(), saved.TenantID, saved.Slug)
		require.NoError(t, err)
		assert.Equal(t, d1.Status, d2.Status)
	})

	t.Run("Scenario_VersionMustBeStrictSemverForUpgradeOrdering", func(t *testing.T) {
		// Given the marketplace lists multiple versions of the same
		// extension and the runtime needs to pick "latest",
		// When admin installs with a non-semver version string,
		// Then registry rejects (semver is the contract for ordering;
		// loose version strings break upgrade paths).
		r := NewInMemoryExtensionRegistry()
		d := validExtensionDescriptor()
		d.Version = "1.x"
		_, err := r.Install(context.Background(), d)
		assert.Error(t, err)
	})

	t.Run("Scenario_ListByStatusEnablesPendingReviewWorkflow", func(t *testing.T) {
		// Given admin opens the "needs review" inbox,
		// When the UI calls ListByStatus(pending_review),
		// Then it sees only extensions that haven't been triaged
		// (without scanning the full installed list).
		r := NewInMemoryExtensionRegistry()
		for _, slug := range []string{"to-review-1", "to-review-2", "already-enabled"} {
			d := validExtensionDescriptor()
			d.Slug = slug
			_, _ = r.Install(context.Background(), d)
		}
		_, _ = r.Enable(context.Background(), "tenant-a", "already-enabled")
		pending, err := r.ListByStatus(context.Background(), "tenant-a", ExtensionStatusPendingReview)
		require.NoError(t, err)
		assert.Len(t, pending, 2)
	})

	t.Run("Scenario_RejectsComponentKindsThatDontApplyToWebPlatform", func(t *testing.T) {
		// Given AgentHub is web-only (no LSP / channels / worktrees),
		// When an extension declares a worktrees component,
		// Then install fails (web product cannot host CLI-only kinds).
		r := NewInMemoryExtensionRegistry()
		d := validExtensionDescriptor()
		d.Components[ExtensionComponentKind("worktrees")] = []string{"my-worktree"}
		_, err := r.Install(context.Background(), d)
		assert.Error(t, err)
	})
}
