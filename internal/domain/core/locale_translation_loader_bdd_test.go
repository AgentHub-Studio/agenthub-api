package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBDD_AhCoreLocaleTranslationSeed(t *testing.T) {
	t.Run("Scenario_FreshTenantInheritsFourBaseLocales", func(t *testing.T) {
		// Given a fresh tenant with users in EN/PT/ES/FR,
		assert.Equal(t, 4, len(SeedExpectedLocales),
			"4 base locales: en-US default + pt-BR + es-ES + fr-FR")
	})

	t.Run("Scenario_PortugueseBrazilIsAvailableForAgentHubAudience", func(t *testing.T) {
		// Given AgentHub is built in pt-BR primary language,
		set := map[string]bool{}
		for _, l := range SeedExpectedLocales {
			set[l] = true
		}
		assert.True(t, set["pt-BR"], "pt-BR required for AgentHub primary audience")
	})

	t.Run("Scenario_EnglishUSIsTheDefaultFallback", func(t *testing.T) {
		// Given users without specified locale fall back to default,
		assert.Equal(t, "en-US", SeedDefaultLocale)
	})

	t.Run("Scenario_SixCategoriesCoverCommonAgentInteractions", func(t *testing.T) {
		expected := map[string]bool{
			"greeting": true, "acknowledgement": true, "error": true,
			"confirmation": true, "progress": true, "closing": true,
		}
		for _, c := range SeedExpectedTranslationCategories {
			assert.True(t, expected[string(c)])
		}
	})

	t.Run("Scenario_KeysFollowCategoryDottedNameConvention", func(t *testing.T) {
		for _, k := range SeedExpectedTranslationKeys {
			parts := strings.Split(k, ".")
			assert.Len(t, parts, 2, "key %q must be category.name", k)
		}
	})

	t.Run("Scenario_BCP47LocaleFormatEnablesProperLocaleNegotiation", func(t *testing.T) {
		// Given middleware uses BCP 47 for Accept-Language matching,
		for _, l := range SeedExpectedLocales {
			assert.Contains(t, l, "-",
				"locale %q must follow BCP 47 (lang-REGION)", l)
		}
	})

	t.Run("Scenario_RowCountIsLocalesTimesKeysProduct", func(t *testing.T) {
		// Every locale must have every key — full coverage matrix.
		assert.Equal(t,
			len(SeedExpectedLocales)*len(SeedExpectedTranslationKeys),
			SeedExpectedTranslationRowCount,
			"row count = locales × keys (full coverage)")
	})

	t.Run("Scenario_TranslationsFeedFUTURE002CommunicationStyle", func(t *testing.T) {
		// Given FUTURE-002 user-agent relationship tracks
		//       CommunicationStyle (formal/casual/terse/verbose),
		//       agent uses these translations for tone-appropriate response,
		assert.NotEmpty(t, SeedExpectedTranslationCategories,
			"translations exist for FUTURE-002 personalization")
	})
}
