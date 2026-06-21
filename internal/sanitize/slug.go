package sanitize

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// CanonicalSlugPattern is the product v1 slug contract exposed by AgentHub APIs.
const CanonicalSlugPattern = `^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$`

// SlugMaxLength is the maximum slug length accepted by CanonicalSlugPattern.
const SlugMaxLength = 64

var canonicalSlugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,62}[a-z0-9])?$`)

// ValidSlug validates the canonical API slug format.
func ValidSlug(slug string) bool {
	return canonicalSlugPattern.MatchString(slug)
}

// ToSlug converts a display name into the canonical lowercase hyphenated slug.
func ToSlug(name, fallbackPrefix string) string {
	name = strings.ToLower(strings.TrimSpace(name))
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
	result := strings.Trim(b.String(), "-")
	if result == "" {
		result = strings.Trim(fallbackPrefix, "-") + "-" + uuid.New().String()[:8]
	}
	return trimSlug(result)
}

// SlugWithNumericSuffix appends "-N" without violating the canonical max length.
func SlugWithNumericSuffix(base string, n int) string {
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
