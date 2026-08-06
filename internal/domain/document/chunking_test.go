package document

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitPlainTextChunksMaintainsUTF8AtChunkBoundary(t *testing.T) {
	input := strings.Repeat("a", 1599) + "é" + strings.Repeat("b", 32)

	chunks := splitPlainTextChunks(input)

	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d: %#v", len(chunks), chunks)
	}
	assertTextChunkInvariants(t, chunks)
}

func FuzzSplitPlainTextChunksMaintainsInvariants(f *testing.F) {
	for _, seed := range []string{
		"first paragraph\n\nsecond paragraph",
		strings.Repeat("a", 1599) + "é" + strings.Repeat("b", 32),
		"line one\r\nline two\r\n\r\nline three",
		string([]byte{0xff, 0xfe, 'A', 'g', 'e', 'n', 't'}),
		strings.Repeat("á", 900),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		if len(input) > maxInlineTextIndexBytes {
			input = input[:maxInlineTextIndexBytes]
		}

		chunks := splitPlainTextChunks(input)
		assertTextChunkInvariants(t, chunks)
	})
}

func assertTextChunkInvariants(t *testing.T, chunks []string) {
	t.Helper()

	for i, chunk := range chunks {
		if strings.TrimSpace(chunk) == "" {
			t.Fatalf("chunk %d is blank: %#v", i, chunks)
		}
		if len(chunk) > maxPlainTextChunkBytes {
			t.Fatalf("chunk %d exceeds %d bytes: len=%d chunk=%q", i, maxPlainTextChunkBytes, len(chunk), chunk)
		}
		if !utf8.ValidString(chunk) {
			t.Fatalf("chunk %d is invalid UTF-8: %q", i, chunk)
		}
	}
}
