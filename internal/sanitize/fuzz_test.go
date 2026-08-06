package sanitize_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/AgentHub-Studio/agenthub-api/internal/sanitize"
)

func FuzzToSlugCanonical(f *testing.F) {
	for _, seed := range []string{
		"HTTP.User Lookup!",
		"Cafe API",
		strings.Repeat("a", sanitize.SlugMaxLength+10),
		"---",
		"<script>alert(1)</script>",
		"emoji 🚀 slug",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, name string) {
		slug := sanitize.ToSlug(name, "tool")
		if !sanitize.ValidSlug(slug) {
			t.Fatalf("ToSlug(%q) returned non-canonical slug %q", name, slug)
		}
		if len(slug) > sanitize.SlugMaxLength {
			t.Fatalf("ToSlug(%q) returned slug longer than %d bytes: %q", name, sanitize.SlugMaxLength, slug)
		}
	})
}

func FuzzSlugWithNumericSuffixCanonical(f *testing.F) {
	for _, seed := range []struct {
		name string
		n    int
	}{
		{name: "Tool", n: 1},
		{name: strings.Repeat("a", sanitize.SlugMaxLength+10), n: 99},
		{name: "---", n: 123456},
	} {
		f.Add(seed.name, seed.n)
	}

	f.Fuzz(func(t *testing.T, name string, n int) {
		if n < 0 {
			n = -n
		}
		base := sanitize.ToSlug(name, "tool")
		slug := sanitize.SlugWithNumericSuffix(base, n)
		if !sanitize.ValidSlug(slug) {
			t.Fatalf("SlugWithNumericSuffix(%q, %d) returned non-canonical slug %q", base, n, slug)
		}
		if len(slug) > sanitize.SlugMaxLength {
			t.Fatalf("SlugWithNumericSuffix(%q, %d) returned slug longer than %d bytes: %q", base, n, sanitize.SlugMaxLength, slug)
		}
	})
}

func FuzzStripHTMLRemovesRecognizedTags(f *testing.F) {
	for _, seed := range []string{
		`<img src=x onerror="alert(1)">Agent`,
		`<script>alert(1)</script>safe`,
		`Use 2 < 3 and 4 > 1 as constraints`,
		`<iframe src="https://example.com"></iframe>text`,
		`<a href="javascript:alert(1)">click</a>`,
		string([]byte{0xff, 0xfe, '<', 'b', '>', 'x', '<', '/', 'b', '>'}),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		output := sanitize.StripHTML(input)
		if sanitize.ContainsHTML(output) {
			t.Fatalf("StripHTML left recognized HTML in output: input=%q output=%q", input, output)
		}
		if again := sanitize.StripHTML(output); again != output {
			t.Fatalf("StripHTML is not idempotent: input=%q output=%q again=%q", input, output, again)
		}
		if !utf8.ValidString(output) {
			t.Fatalf("StripHTML returned invalid UTF-8: input=%q output=%q", input, output)
		}
	})
}
