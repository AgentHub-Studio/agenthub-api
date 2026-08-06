package sanitize

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// CanonicalSlugPattern is the product v1 slug contract exposed by AgentHub APIs.
const CanonicalSlugPattern = `^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$`

// SlugMaxLength is the maximum slug length accepted by CanonicalSlugPattern.
const SlugMaxLength = 64

var canonicalSlugPattern = regexp.MustCompile(CanonicalSlugPattern)

// ValidSlug validates the canonical API slug format.
func ValidSlug(slug string) bool {
	return canonicalSlugPattern.MatchString(slug)
}

// ToSlug converts a display name into the canonical lowercase hyphenated slug.
func ToSlug(name, fallbackPrefix string) string {
	result := slugBody(name)
	if result == "" {
		prefix := slugBody(fallbackPrefix)
		if prefix == "" {
			prefix = "slug"
		}
		result = prefix + "-" + uuid.New().String()[:8]
	}
	if len(result) == 1 {
		suffix := slugBody(fallbackPrefix)
		if suffix == "" || suffix == result {
			suffix = "slug"
		}
		result = result + "-" + suffix
	}
	result = trimSlug(result)
	if !ValidSlug(result) {
		result = "slug-" + uuid.New().String()[:8]
	}
	return trimSlug(result)
}

func slugBody(input string) string {
	name := removeDiacritics(strings.ToLower(strings.TrimSpace(input)))
	var b strings.Builder
	prevHyphen := true
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
			continue
		}
		if !prevHyphen {
			b.WriteByte('-')
			prevHyphen = true
		}
	}
	return trimSlug(b.String())
}

func removeDiacritics(input string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	output, _, err := transform.String(t, input)
	if err != nil {
		return input
	}
	return output
}

// SlugWithNumericSuffix appends "-N" without violating the canonical max length.
func SlugWithNumericSuffix(base string, n int) string {
	base = trimSlug(base)
	if base == "" {
		base = "slug"
	}
	suffix := fmt.Sprintf("-%d", n)
	if len(base)+len(suffix) <= SlugMaxLength {
		return base + suffix
	}
	maxBaseLen := SlugMaxLength - len(suffix)
	if maxBaseLen < 1 {
		maxBaseLen = 1
	}
	return strings.TrimRight(base[:maxBaseLen], "-") + suffix
}

func trimSlug(slug string) string {
	if len(slug) <= SlugMaxLength {
		return strings.Trim(slug, "-")
	}
	return strings.Trim(slug[:SlugMaxLength], "-")
}
