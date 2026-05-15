package core

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validPlatformCatalog() PlatformCatalogManifest {
	return PlatformCatalogManifest{
		Slug:                  "builtin_subagent_default_template",
		Kind:                  PlatformCatalogKindSubagentRoster,
		MigrationNumber:       61,
		LoaderPackage:         "core",
		ExpectedRowCount:      7,
		RequiresAdminApproval: false,
		IsTenantShared:        true,
	}
}

func TestPlatformCatalog_IsValidKind(t *testing.T) {
	for _, k := range allPlatformCatalogKinds {
		assert.True(t, IsValidPlatformCatalogKind(k))
	}
	assert.False(t, IsValidPlatformCatalogKind(PlatformCatalogKind("nope")))
}

func TestPlatformCatalog_AllKindsReturnsCopy(t *testing.T) {
	k := AllPlatformCatalogKinds()
	require.Equal(t, 11, len(k))
	k[0] = "tampered"
	k2 := AllPlatformCatalogKinds()
	assert.Equal(t, PlatformCatalogKindSubagentRoster, k2[0])
}

func TestPlatformCatalog_ValidateBadSlug(t *testing.T) {
	m := validPlatformCatalog()
	m.Slug = "BadSlug"
	assert.ErrorIs(t, m.Validate(), ErrPlatformCatalogBadSlug)
}

func TestPlatformCatalog_ValidateBadKind(t *testing.T) {
	m := validPlatformCatalog()
	m.Kind = "limbo"
	assert.ErrorIs(t, m.Validate(), ErrPlatformCatalogBadKind)
}

func TestPlatformCatalog_ValidateBadMigration(t *testing.T) {
	m := validPlatformCatalog()
	m.MigrationNumber = 0
	assert.ErrorIs(t, m.Validate(), ErrPlatformCatalogBadMigration)
	m.MigrationNumber = -1
	assert.ErrorIs(t, m.Validate(), ErrPlatformCatalogBadMigration)
}

func TestPlatformCatalog_ValidateBadPackage(t *testing.T) {
	m := validPlatformCatalog()
	m.LoaderPackage = "Core"
	assert.ErrorIs(t, m.Validate(), ErrPlatformCatalogBadPackage)
	m.LoaderPackage = ""
	assert.ErrorIs(t, m.Validate(), ErrPlatformCatalogBadPackage)
	m.LoaderPackage = "with space"
	assert.ErrorIs(t, m.Validate(), ErrPlatformCatalogBadPackage)
}

func TestPlatformCatalog_ValidateBadRowCount(t *testing.T) {
	m := validPlatformCatalog()
	m.ExpectedRowCount = 0
	assert.ErrorIs(t, m.Validate(), ErrPlatformCatalogBadRowCount)
	m.ExpectedRowCount = -3
	assert.ErrorIs(t, m.Validate(), ErrPlatformCatalogBadRowCount)
}

func TestPlatformCatalog_RegistryRegisterAndLookup(t *testing.T) {
	r := NewPlatformCatalogRegistry()
	m := validPlatformCatalog()
	require.NoError(t, r.Register(m))
	got, ok := r.Lookup(m.Slug)
	require.True(t, ok)
	assert.Equal(t, m.MigrationNumber, got.MigrationNumber)
}

func TestPlatformCatalog_RegistryLookupUnknown(t *testing.T) {
	r := NewPlatformCatalogRegistry()
	_, ok := r.Lookup("nope")
	assert.False(t, ok)
}

func TestPlatformCatalog_RegistryRejectsBadManifest(t *testing.T) {
	r := NewPlatformCatalogRegistry()
	bad := validPlatformCatalog()
	bad.MigrationNumber = 0
	assert.ErrorIs(t, r.Register(bad), ErrPlatformCatalogBadMigration)
}

func TestPlatformCatalog_RegistryRejectsDuplicateSlug(t *testing.T) {
	r := NewPlatformCatalogRegistry()
	m := validPlatformCatalog()
	require.NoError(t, r.Register(m))
	err := r.Register(m)
	assert.ErrorIs(t, err, ErrPlatformCatalogDuplicateSlug)
}

func TestPlatformCatalog_RegistryRejectsDuplicateMigrationNumber(t *testing.T) {
	r := NewPlatformCatalogRegistry()
	m1 := validPlatformCatalog()
	m2 := validPlatformCatalog()
	m2.Slug = "another_table"
	// same migration number
	require.NoError(t, r.Register(m1))
	err := r.Register(m2)
	assert.ErrorIs(t, err, ErrPlatformCatalogDuplicateMigration)
}

func TestPlatformCatalog_ListAllSortedBySlug(t *testing.T) {
	r := NewPlatformCatalogRegistry()
	m1 := validPlatformCatalog()
	m1.Slug = "zeta_table"
	m1.MigrationNumber = 70
	m2 := validPlatformCatalog()
	m2.Slug = "alpha_table"
	m2.MigrationNumber = 80
	require.NoError(t, r.Register(m1))
	require.NoError(t, r.Register(m2))
	list := r.ListAll()
	require.Equal(t, 2, len(list))
	assert.Equal(t, "alpha_table", list[0].Slug)
}

func TestPlatformCatalog_ListByKindFilters(t *testing.T) {
	r := NewPlatformCatalogRegistry()
	m1 := validPlatformCatalog()
	m1.Slug = "subagent_roster_a"
	m1.MigrationNumber = 61
	m1.Kind = PlatformCatalogKindSubagentRoster
	m2 := validPlatformCatalog()
	m2.Slug = "fork_strategy_a"
	m2.MigrationNumber = 60
	m2.Kind = PlatformCatalogKindForkStrategy
	require.NoError(t, r.Register(m1))
	require.NoError(t, r.Register(m2))
	rosters := r.ListByKind(PlatformCatalogKindSubagentRoster)
	require.Equal(t, 1, len(rosters))
	assert.Equal(t, "subagent_roster_a", rosters[0].Slug)
}

func TestPlatformCatalog_TotalExpectedRowCount(t *testing.T) {
	r := NewPlatformCatalogRegistry()
	m1 := validPlatformCatalog()
	m1.Slug = "table_a"
	m1.MigrationNumber = 90
	m1.ExpectedRowCount = 7
	m2 := validPlatformCatalog()
	m2.Slug = "table_b"
	m2.MigrationNumber = 91
	m2.ExpectedRowCount = 4
	require.NoError(t, r.Register(m1))
	require.NoError(t, r.Register(m2))
	assert.Equal(t, 11, r.TotalExpectedRowCount())
}

func TestPlatformCatalog_Size(t *testing.T) {
	r := NewPlatformCatalogRegistry()
	assert.Equal(t, 0, r.Size())
	m := validPlatformCatalog()
	_ = r.Register(m)
	assert.Equal(t, 1, r.Size())
}

func TestPlatformCatalog_ConcurrentRegisterSafe(t *testing.T) {
	r := NewPlatformCatalogRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			m := validPlatformCatalog()
			m.Slug = "concurrent_" + string(rune('a'+idx%26)) + string(rune('0'+idx/26))
			m.MigrationNumber = 1000 + idx
			_ = r.Register(m)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, 50, r.Size())
}

func TestPlatformCatalog_ElevenKindsCoverMajorFamilies(t *testing.T) {
	expected := map[PlatformCatalogKind]bool{
		PlatformCatalogKindSubagentRoster:      true,
		PlatformCatalogKindAgentDefinition:     true,
		PlatformCatalogKindToolsetPolicy:       true,
		PlatformCatalogKindInheritanceMode:     true,
		PlatformCatalogKindSummaryShape:        true,
		PlatformCatalogKindForkStrategy:        true,
		PlatformCatalogKindBackgroundLane:      true,
		PlatformCatalogKindContextPolicy:       true,
		PlatformCatalogKindPermissionPolicy:    true,
		PlatformCatalogKindExtensionDescriptor: true,
		PlatformCatalogKindOperationalTemplate: true,
	}
	assert.Equal(t, 11, len(expected))
	for _, k := range allPlatformCatalogKinds {
		assert.True(t, expected[k])
	}
}
