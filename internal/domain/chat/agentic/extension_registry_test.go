package agentic

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validExtensionDescriptor() ExtensionDescriptor {
	return ExtensionDescriptor{
		TenantID:  "tenant-a",
		Slug:      "claude-research-pack",
		Name:      "Claude Research Pack",
		Version:   "1.2.3",
		Source:    ExtensionSourceMarketplace,
		SourceRef: "marketplace:claude-research-pack@1.2.3",
		Checksum:  "sha256:abc123def456",
		Components: map[ExtensionComponentKind][]string{
			ExtensionComponentSkills:  {"web-research", "kb-research"},
			ExtensionComponentAgents:  {"researcher"},
			ExtensionComponentCommands: {"summarize"},
		},
	}
}

func TestExtension_StatusEnumIsBounded(t *testing.T) {
	for _, s := range AllExtensionStatuses() {
		assert.True(t, IsValidExtensionStatus(s))
	}
	assert.False(t, IsValidExtensionStatus(ExtensionStatus("active")))
	assert.Equal(t, 4, len(AllExtensionStatuses()))
}

func TestExtension_SourceEnumIsBounded(t *testing.T) {
	for _, s := range AllExtensionSources() {
		assert.True(t, IsValidExtensionSource(s))
	}
	assert.False(t, IsValidExtensionSource(ExtensionSource("npm")))
	assert.Equal(t, 5, len(AllExtensionSources()))
}

func TestExtension_ComponentKindEnumIsBounded(t *testing.T) {
	for _, k := range AllExtensionComponentKinds() {
		assert.True(t, IsValidExtensionComponentKind(k))
	}
	assert.False(t, IsValidExtensionComponentKind(ExtensionComponentKind("worktrees")))
	assert.False(t, IsValidExtensionComponentKind(ExtensionComponentKind("lsp_servers")))
	assert.False(t, IsValidExtensionComponentKind(ExtensionComponentKind("channels")))
	// 10 web-applicable kinds: 8 original + settings + user_configuration (FEAT-010, §6.1).
	// LSP servers and channels remain NOT_APPLICABLE_WEB.
	assert.Equal(t, 10, len(AllExtensionComponentKinds()))
}

func TestExtension_Install_DefaultStatusIsPendingReview(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	d := validExtensionDescriptor()
	d.Status = "" // omit; should default
	saved, err := r.Install(context.Background(), d)
	require.NoError(t, err)
	assert.Equal(t, ExtensionStatusPendingReview, saved.Status)
	assert.True(t, saved.EnabledAt.IsZero(), "EnabledAt zero until enabled")
}

func TestExtension_Install_EnabledStatusSetsEnabledAt(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	d := validExtensionDescriptor()
	d.Status = ExtensionStatusEnabled
	saved, err := r.Install(context.Background(), d)
	require.NoError(t, err)
	assert.False(t, saved.EnabledAt.IsZero())
}

func TestExtension_Install_RejectsRequiredFields(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	for _, missing := range []string{"tenant", "slug", "name", "version"} {
		d := validExtensionDescriptor()
		switch missing {
		case "tenant":
			d.TenantID = ""
		case "slug":
			d.Slug = ""
		case "name":
			d.Name = ""
		case "version":
			d.Version = "1.0"
		}
		_, err := r.Install(context.Background(), d)
		assert.Error(t, err, "missing %s must error", missing)
	}
}

func TestExtension_Install_RejectsInvalidSlugFormat(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	for _, badSlug := range []string{"Bad_Slug", "with spaces", "-start", "end-", "UPPER"} {
		d := validExtensionDescriptor()
		d.Slug = badSlug
		_, err := r.Install(context.Background(), d)
		assert.Error(t, err, "slug %q must be rejected", badSlug)
	}
}

func TestExtension_Install_RejectsInvalidSemver(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	for _, badVer := range []string{"1.2", "1.2.3.4", "v1.2.3", "1.2.x"} {
		d := validExtensionDescriptor()
		d.Version = badVer
		_, err := r.Install(context.Background(), d)
		assert.True(t, errors.Is(err, ErrInvalidExtensionVersion),
			"version %q must error with InvalidExtensionVersion", badVer)
	}
}

func TestExtension_Install_RequiresChecksumForUntrustedSources(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	for _, src := range []ExtensionSource{
		ExtensionSourceMarketplace, ExtensionSourceGit, ExtensionSourceURL,
	} {
		d := validExtensionDescriptor()
		d.Source = src
		d.Checksum = ""
		_, err := r.Install(context.Background(), d)
		assert.True(t, errors.Is(err, ErrExtensionChecksumRequired),
			"source %q without checksum must error", src)
	}
}

func TestExtension_Install_AllowsBuiltinAndLocalWithoutChecksum(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	cases := []struct {
		slug   string
		source ExtensionSource
	}{
		{"builtin-pack", ExtensionSourceBuiltin},
		{"local-pack", ExtensionSourceLocalPath},
	}
	for _, c := range cases {
		d := validExtensionDescriptor()
		d.Slug = c.slug
		d.Source = c.source
		d.Checksum = ""
		_, err := r.Install(context.Background(), d)
		assert.NoError(t, err, "source %q must allow no checksum", c.source)
	}
}

func TestExtension_Install_RejectsDuplicateSlugSameTenant(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	_, err := r.Install(context.Background(), validExtensionDescriptor())
	require.NoError(t, err)
	_, err = r.Install(context.Background(), validExtensionDescriptor())
	assert.True(t, errors.Is(err, ErrExtensionAlreadyExists))
}

func TestExtension_Install_AllowsSameSlugAcrossDifferentTenants(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	d1 := validExtensionDescriptor()
	_, err := r.Install(context.Background(), d1)
	require.NoError(t, err)
	d2 := validExtensionDescriptor()
	d2.TenantID = "tenant-b"
	_, err = r.Install(context.Background(), d2)
	assert.NoError(t, err, "tenant isolation: same slug allowed in different tenant")
}

func TestExtension_Install_RejectsDuplicateComponentSlugWithinExtension(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	d := validExtensionDescriptor()
	d.Components[ExtensionComponentSkills] = []string{"web-research", "web-research"}
	_, err := r.Install(context.Background(), d)
	assert.Error(t, err)
}

func TestExtension_Install_RejectsInvalidComponentKind(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	d := validExtensionDescriptor()
	d.Components[ExtensionComponentKind("worktrees")] = []string{"my-worktree"}
	_, err := r.Install(context.Background(), d)
	assert.True(t, errors.Is(err, ErrInvalidComponentKind))
}

func TestExtension_Find_RoundTrips(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	saved, _ := r.Install(context.Background(), validExtensionDescriptor())
	got, err := r.Find(context.Background(), saved.TenantID, saved.Slug)
	require.NoError(t, err)
	assert.Equal(t, saved.ID, got.ID)
}

func TestExtension_Find_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	_, err := r.Find(context.Background(), "tenant-a", "nope")
	assert.True(t, errors.Is(err, ErrExtensionNotFound))
}

func TestExtension_Enable_TransitionsAndSetsEnabledAt(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	saved, _ := r.Install(context.Background(), validExtensionDescriptor())
	require.Equal(t, ExtensionStatusPendingReview, saved.Status)
	enabled, err := r.Enable(context.Background(), saved.TenantID, saved.Slug)
	require.NoError(t, err)
	assert.Equal(t, ExtensionStatusEnabled, enabled.Status)
	assert.False(t, enabled.EnabledAt.IsZero())
}

func TestExtension_Enable_QuarantinedIsBlocked(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	saved, _ := r.Install(context.Background(), validExtensionDescriptor())
	_, _ = r.Quarantine(context.Background(), saved.TenantID, saved.Slug, "signature mismatch")
	_, err := r.Enable(context.Background(), saved.TenantID, saved.Slug)
	assert.True(t, errors.Is(err, ErrExtensionQuarantined),
		"quarantined cannot be enabled directly")
}

func TestExtension_Disable_QuarantinedIsBlocked(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	saved, _ := r.Install(context.Background(), validExtensionDescriptor())
	_, _ = r.Quarantine(context.Background(), saved.TenantID, saved.Slug, "abuse")
	_, err := r.Disable(context.Background(), saved.TenantID, saved.Slug)
	assert.True(t, errors.Is(err, ErrExtensionQuarantined))
}

func TestExtension_Disable_IsIdempotent(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	saved, _ := r.Install(context.Background(), validExtensionDescriptor())
	_, _ = r.Enable(context.Background(), saved.TenantID, saved.Slug)
	d1, err := r.Disable(context.Background(), saved.TenantID, saved.Slug)
	require.NoError(t, err)
	d2, err := r.Disable(context.Background(), saved.TenantID, saved.Slug)
	require.NoError(t, err)
	assert.Equal(t, d1.Status, d2.Status)
	assert.Equal(t, ExtensionStatusDisabled, d2.Status)
}

func TestExtension_Quarantine_RequiresReason(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	saved, _ := r.Install(context.Background(), validExtensionDescriptor())
	_, err := r.Quarantine(context.Background(), saved.TenantID, saved.Slug, "")
	assert.Error(t, err)
}

func TestExtension_AdminResetQuarantine_ReturnsToDisabled(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	saved, _ := r.Install(context.Background(), validExtensionDescriptor())
	_, _ = r.Quarantine(context.Background(), saved.TenantID, saved.Slug, "abuse")
	cleared, err := r.AdminResetQuarantine(context.Background(), saved.TenantID, saved.Slug)
	require.NoError(t, err)
	assert.Equal(t, ExtensionStatusDisabled, cleared.Status,
		"admin reset returns to disabled, not enabled — admin must explicitly re-enable")
}

func TestExtension_AdminResetQuarantine_RejectsIfNotQuarantined(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	saved, _ := r.Install(context.Background(), validExtensionDescriptor())
	_, err := r.AdminResetQuarantine(context.Background(), saved.TenantID, saved.Slug)
	assert.Error(t, err, "cannot reset non-quarantined extension")
}

func TestExtension_Uninstall_RemovesEntry(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	saved, _ := r.Install(context.Background(), validExtensionDescriptor())
	require.NoError(t, r.Uninstall(context.Background(), saved.TenantID, saved.Slug))
	_, err := r.Find(context.Background(), saved.TenantID, saved.Slug)
	assert.True(t, errors.Is(err, ErrExtensionNotFound))
}

func TestExtension_Uninstall_UnknownReturnsNotFound(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	err := r.Uninstall(context.Background(), "tenant-a", "missing")
	assert.True(t, errors.Is(err, ErrExtensionNotFound))
}

func TestExtension_List_OrderedBySlug(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	for _, slug := range []string{"zeta-pack", "alpha-pack", "mid-pack"} {
		d := validExtensionDescriptor()
		d.Slug = slug
		_, _ = r.Install(context.Background(), d)
	}
	got, err := r.List(context.Background(), "tenant-a")
	require.NoError(t, err)
	require.Len(t, got, 3)
	for i := 1; i < len(got); i++ {
		assert.Less(t, got[i-1].Slug, got[i].Slug)
	}
}

func TestExtension_List_TenantIsolation(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	_, _ = r.Install(context.Background(), validExtensionDescriptor())
	d2 := validExtensionDescriptor()
	d2.TenantID = "tenant-b"
	_, _ = r.Install(context.Background(), d2)
	got, err := r.List(context.Background(), "tenant-a")
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "tenant-a", got[0].TenantID)
}

func TestExtension_ListByStatus_Filters(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	for _, slug := range []string{"a-pack", "b-pack", "c-pack"} {
		d := validExtensionDescriptor()
		d.Slug = slug
		_, _ = r.Install(context.Background(), d)
	}
	_, _ = r.Enable(context.Background(), "tenant-a", "b-pack")
	enabled, err := r.ListByStatus(context.Background(), "tenant-a", ExtensionStatusEnabled)
	require.NoError(t, err)
	assert.Len(t, enabled, 1)
	pending, _ := r.ListByStatus(context.Background(), "tenant-a", ExtensionStatusPendingReview)
	assert.Len(t, pending, 2)
}

func TestExtension_ListByComponent_Filters(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	d1 := validExtensionDescriptor()
	d1.Slug = "skills-only"
	d1.Components = map[ExtensionComponentKind][]string{
		ExtensionComponentSkills: {"x"},
	}
	_, _ = r.Install(context.Background(), d1)

	d2 := validExtensionDescriptor()
	d2.Slug = "agents-only"
	d2.Components = map[ExtensionComponentKind][]string{
		ExtensionComponentAgents: {"y"},
	}
	_, _ = r.Install(context.Background(), d2)

	skills, err := r.ListByComponent(context.Background(), "tenant-a", ExtensionComponentSkills)
	require.NoError(t, err)
	require.Len(t, skills, 1)
	assert.Equal(t, "skills-only", skills[0].Slug)
}

func TestExtension_ListBySource_Filters(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	for i, src := range []ExtensionSource{
		ExtensionSourceMarketplace, ExtensionSourceGit, ExtensionSourceMarketplace,
	} {
		d := validExtensionDescriptor()
		d.Slug = []string{"mp-1", "git-1", "mp-2"}[i]
		d.Source = src
		_, _ = r.Install(context.Background(), d)
	}
	mp, err := r.ListBySource(context.Background(), "tenant-a", ExtensionSourceMarketplace)
	require.NoError(t, err)
	assert.Len(t, mp, 2)
}

func TestExtension_ConcurrentInstallIsSafe(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := validExtensionDescriptor()
			d.Slug = "ext-" + string(rune('a'+(i%26))) + "-" + string(rune('a'+(i/26)))
			_, _ = r.Install(context.Background(), d)
		}()
	}
	wg.Wait()
	got, _ := r.List(context.Background(), "tenant-a")
	assert.GreaterOrEqual(t, len(got), 1)
}

func TestExtension_ContextCancelled(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	saved, _ := r.Install(context.Background(), validExtensionDescriptor())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.Install(ctx, validExtensionDescriptor())
	assert.Error(t, err)
	_, err = r.Find(ctx, saved.TenantID, saved.Slug)
	assert.Error(t, err)
	err = r.Uninstall(ctx, saved.TenantID, saved.Slug)
	assert.Error(t, err)
	_, err = r.Enable(ctx, saved.TenantID, saved.Slug)
	assert.Error(t, err)
	_, err = r.Disable(ctx, saved.TenantID, saved.Slug)
	assert.Error(t, err)
	_, err = r.Quarantine(ctx, saved.TenantID, saved.Slug, "x")
	assert.Error(t, err)
	_, err = r.AdminResetQuarantine(ctx, saved.TenantID, saved.Slug)
	assert.Error(t, err)
	_, err = r.List(ctx, saved.TenantID)
	assert.Error(t, err)
	_, err = r.ListByStatus(ctx, saved.TenantID, ExtensionStatusEnabled)
	assert.Error(t, err)
	_, err = r.ListByComponent(ctx, saved.TenantID, ExtensionComponentSkills)
	assert.Error(t, err)
	_, err = r.ListBySource(ctx, saved.TenantID, ExtensionSourceMarketplace)
	assert.Error(t, err)
}

func TestExtensionRegistry_SetClockInjectsDeterministicTimestamps(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	stamp := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	r.SetClock(func() time.Time { return stamp })

	d := ExtensionDescriptor{
		TenantID: "t1", Slug: "test-ext", Name: "Test Extension",
		Version: "1.0.0", Source: ExtensionSourceBuiltin,
		Components: map[ExtensionComponentKind][]string{
			ExtensionComponentSkills: {"my-skill"},
		},
	}
	saved, err := r.Install(context.Background(), d)
	require.NoError(t, err)
	assert.Equal(t, stamp, saved.InstalledAt)
	assert.Equal(t, stamp, saved.UpdatedAt)

	// Enable triggers another clock read; with same injected clock the
	// timestamps remain deterministic.
	enabled, err := r.Enable(context.Background(), "t1", "test-ext")
	require.NoError(t, err)
	assert.Equal(t, stamp, enabled.EnabledAt)
	assert.Equal(t, stamp, enabled.UpdatedAt)
}

func TestExtensionRegistry_SetClockNilIsNoop(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	stamp := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	r.SetClock(func() time.Time { return stamp })
	// Calling SetClock(nil) must NOT clear the previously-set clock.
	r.SetClock(nil)

	d := ExtensionDescriptor{
		TenantID: "t1", Slug: "test-ext", Name: "Test Extension",
		Version: "1.0.0", Source: ExtensionSourceBuiltin,
	}
	saved, _ := r.Install(context.Background(), d)
	assert.Equal(t, stamp, saved.InstalledAt)
}

func TestExtensionRegistry_DefaultClockIsTimeNow(t *testing.T) {
	r := NewInMemoryExtensionRegistry()
	d := ExtensionDescriptor{
		TenantID: "t1", Slug: "test-ext", Name: "Test Extension",
		Version: "1.0.0", Source: ExtensionSourceBuiltin,
	}
	before := time.Now()
	saved, _ := r.Install(context.Background(), d)
	after := time.Now()
	// Default clock produces a timestamp within the window.
	assert.True(t, !saved.InstalledAt.Before(before))
	assert.True(t, !saved.InstalledAt.After(after))
}
