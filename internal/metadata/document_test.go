package metadata

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDocument_AcceptsSupportedValuesAtTwoLevels(t *testing.T) {
	got, err := ParseDocument([]byte(`{"source":"manual","year":2026,"published":true,"tags":["release","api"],"customer":{"region":"br","tier":2,"active":true,"labels":["priority"]}}`))

	require.NoError(t, err)
	assert.JSONEq(t, `{"source":"manual","year":2026,"published":true,"tags":["release","api"],"customer":{"region":"br","tier":2,"active":true,"labels":["priority"]}}`, string(got))
}

func TestParseDocument_RejectsUnsupportedValues(t *testing.T) {
	for name, raw := range map[string]string{
		"third metadata level":    `{"source":{"owner":{"name":"manual"}}}`,
		"empty nested object":     `{"source":{}}`,
		"nested null":             `{"source":{"name":null}}`,
		"nested mixed array":      `{"source":{"tags":["release",1]}}`,
		"null":                    `{"source":null}`,
		"mixed array":             `{"tags":["release",1]}`,
		"root array":              `["release"]`,
		"empty field":             `{" ":"manual"}`,
		"duplicate field":         `{"source":"first","source":"second"}`,
		"escaped duplicate field": `{"source":"first","\u0073ource":"second"}`,
		"null character field":    `{"source\u0000name":"manual"}`,
		"null character value":    `{"source":"manual\u0000draft"}`,
		"null character tag":      `{"tags":["release\u0000draft"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseDocument([]byte(raw))
			require.Error(t, err)
		})
	}
}

func TestParseFieldPath_AcceptsOneOrTwoLevels(t *testing.T) {
	path, err := ParseFieldPath("customer.region")

	require.NoError(t, err)
	assert.Equal(t, []string{"customer", "region"}, path)

	path, err = ParseFieldPath("source")
	require.NoError(t, err)
	assert.Equal(t, []string{"source"}, path)
}

func TestParseFieldPath_RejectsInvalidLevels(t *testing.T) {
	for name, field := range map[string]string{
		"empty":          "",
		"empty segment":  "customer.",
		"third level":    "customer.profile.region",
		"null character": "customer\x00region",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseFieldPath(field)
			require.Error(t, err)
		})
	}
}

func TestParseDocument_RejectsPostgresJSONBNullCharacter(t *testing.T) {
	_, err := ParseDocument([]byte(`{"source":"manual\u0000draft"}`))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "U+0000")
}

func TestParseDocument_OmittedDefaultsToObject(t *testing.T) {
	got, err := ParseDocument(nil)

	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(got))
}

func TestParseDocument_RejectsOversizeValue(t *testing.T) {
	tooLarge := append([]byte(`{"source":"`), bytes.Repeat([]byte("x"), maxDocumentMetadataBytes)...)
	tooLarge = append(tooLarge, []byte(`"}`)...)

	_, err := ParseDocument(tooLarge)
	require.Error(t, err)
}

func TestParseDocument_RejectsNumbersOutsidePostgresJSONBRange(t *testing.T) {
	for name, raw := range map[string]string{
		"too many integer digits":    `{"year":1e131072}`,
		"too many fractional digits": `{"ratio":1e-16384}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseDocument([]byte(raw))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "PostgreSQL JSONB numeric range")
		})
	}
}

func TestParseDocument_AcceptsPostgresJSONBNumericBoundaries(t *testing.T) {
	_, err := ParseDocument([]byte(`{"integer":1e131071,"fraction":1e-16383}`))

	require.NoError(t, err)
}

func FuzzParseDocument(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{"source":"manual","year":2026,"published":true,"tags":["release"]}`),
		[]byte(`{"source":{"name":"manual"}}`),
		[]byte(`{"source":{"owner":{"name":"manual"}}}`),
		[]byte(`{"tags":["release",1]}`),
		[]byte(`{"source":"first","source":"second"}`),
		[]byte(`{"source":"manual\u0000draft"}`),
		[]byte(`{"year":1e131071}`),
		nil,
		[]byte{0xff, 0xfe},
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		normalized, err := ParseDocument(raw)
		if err != nil {
			return
		}

		if !json.Valid(normalized) {
			t.Fatal("accepted metadata must normalize to valid JSON")
		}
		decoder := json.NewDecoder(bytes.NewReader(normalized))
		decoder.UseNumber()
		var values map[string]any
		if err := decoder.Decode(&values); err != nil {
			t.Fatalf("accepted metadata must normalize to an object: %v", err)
		}
		if values == nil {
			t.Fatal("accepted metadata must normalize to a non-nil object")
		}
	})
}

func TestIsScalar(t *testing.T) {
	assert.True(t, IsScalar("manual"))
	assert.True(t, IsScalar(true))
	assert.True(t, IsScalar(json.Number("2026")))
	assert.False(t, IsScalar([]any{"release"}))
}
