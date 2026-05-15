package core

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CoreLocaleTranslation represents a platform-managed i18n string.
// Locale code is BCP 47 (e.g. en-US, pt-BR). Key is dotted-path
// namespace (e.g. greeting.hello, error.generic).
type CoreLocaleTranslation struct {
	ID         uuid.UUID
	LocaleCode string
	Key        string
	Value      string
	Category   string
	IsActive   bool
}

// CoreLocaleTranslationLoader loads i18n translations.
type CoreLocaleTranslationLoader struct {
	pool *pgxpool.Pool
}

// NewCoreLocaleTranslationLoader creates loader.
func NewCoreLocaleTranslationLoader(pool *pgxpool.Pool) *CoreLocaleTranslationLoader {
	return &CoreLocaleTranslationLoader{pool: pool}
}

// LoadAll returns all active translations.
func (l *CoreLocaleTranslationLoader) LoadAll(ctx context.Context) ([]CoreLocaleTranslation, error) {
	conn, err := l.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("core: acquire connection: %w", err)
	}
	defer conn.Release()

	const query = `
		SELECT id, locale_code, key, value, category, is_active
		  FROM ah_core.locale_translation
		 WHERE is_active = true
		 ORDER BY locale_code, key`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		if isUndefinedRelation(err) {
			slog.WarnContext(ctx, "core: ah_core.locale_translation not accessible", "err", err)
			return nil, nil
		}
		return nil, fmt.Errorf("core: query locale_translation: %w", err)
	}
	defer rows.Close()

	var translations []CoreLocaleTranslation
	for rows.Next() {
		var t CoreLocaleTranslation
		if err := rows.Scan(&t.ID, &t.LocaleCode, &t.Key, &t.Value, &t.Category, &t.IsActive); err != nil {
			return nil, fmt.Errorf("core: scan locale_translation: %w", err)
		}
		translations = append(translations, t)
	}
	if err := rows.Err(); err != nil {
		if isUndefinedRelation(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("core: iterate locale_translation: %w", err)
	}
	return translations, nil
}

// FindByLocaleAndKey returns a single translation.
func (l *CoreLocaleTranslationLoader) FindByLocaleAndKey(ctx context.Context, locale, key string) (CoreLocaleTranslation, bool, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return CoreLocaleTranslation{}, false, err
	}
	for _, t := range all {
		if t.LocaleCode == locale && t.Key == key {
			return t, true, nil
		}
	}
	return CoreLocaleTranslation{}, false, nil
}

// LoadByLocale returns all translations for a locale.
func (l *CoreLocaleTranslationLoader) LoadByLocale(ctx context.Context, locale string) ([]CoreLocaleTranslation, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreLocaleTranslation
	for _, t := range all {
		if t.LocaleCode == locale {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// LoadByCategory returns translations across all locales in a category.
func (l *CoreLocaleTranslationLoader) LoadByCategory(ctx context.Context, category string) ([]CoreLocaleTranslation, error) {
	all, err := l.LoadAll(ctx)
	if err != nil {
		return nil, err
	}
	var matched []CoreLocaleTranslation
	for _, t := range all {
		if t.Category == category {
			matched = append(matched, t)
		}
	}
	return matched, nil
}

// SeedExpectedLocales is the closed set of seeded locales.
var SeedExpectedLocales = []string{
	"en-US", "pt-BR", "es-ES", "fr-FR",
}

// SeedExpectedTranslationCategories is the closed set of categories.
var SeedExpectedTranslationCategories = []string{
	"greeting", "acknowledgement", "error", "confirmation", "progress", "closing",
}

// SeedExpectedTranslationKeys is the closed set of keys (each locale has all 6).
var SeedExpectedTranslationKeys = []string{
	"greeting.hello",
	"acknowledgement.understood",
	"error.generic",
	"confirmation.proceed",
	"progress.thinking",
	"closing.farewell",
}

// SeedDefaultLocale is the platform fallback when user locale unset.
const SeedDefaultLocale = "en-US"

// SeedExpectedTranslationRowCount = 4 locales × 6 keys = 24.
const SeedExpectedTranslationRowCount = 24
