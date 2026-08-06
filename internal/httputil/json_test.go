package httputil

import (
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeSingleJSON(t *testing.T) {
	t.Run("accepts one value with trailing whitespace", func(t *testing.T) {
		var dst struct {
			Name string `json:"name"`
		}
		require.NoError(t, DecodeSingleJSON(strings.NewReader(" {\"name\":\"first\"} \n"), &dst))
		require.Equal(t, "first", dst.Name)
	})

	t.Run("rejects a second JSON value", func(t *testing.T) {
		var dst map[string]string
		require.Error(t, DecodeSingleJSON(strings.NewReader(`{"name":"first"}{"name":"second"}`), &dst))
	})

	t.Run("rejects duplicate object keys at every nesting level", func(t *testing.T) {
		for _, body := range []string{
			`{"name":"first","name":"second"}`,
			`{"name":"first","\u006eame":"second"}`,
			`{"filter":{"field":"source","field":"owner"}}`,
			`{"filters":[{"field":"source","field":"owner"}]}`,
		} {
			var dst map[string]any
			require.Error(t, DecodeSingleJSON(strings.NewReader(body), &dst), body)
		}
	})

	t.Run("accepts a JSONB boundary number", func(t *testing.T) {
		var dst struct {
			Value json.Number `json:"value"`
		}
		require.NoError(t, DecodeSingleJSON(strings.NewReader(`{"value":1e131071}`), &dst))
		require.Equal(t, json.Number("1e131071"), dst.Value)
	})
}

func TestDecodeOptionalSingleJSON(t *testing.T) {
	t.Run("accepts empty body", func(t *testing.T) {
		var dst map[string]string
		require.NoError(t, DecodeOptionalSingleJSON(strings.NewReader(" \n\t"), &dst))
	})

	t.Run("rejects malformed or concatenated body", func(t *testing.T) {
		var dst map[string]string
		require.Error(t, DecodeOptionalSingleJSON(strings.NewReader(`{"name":"first"} {"name":"second"}`), &dst))
	})
}

func FuzzDecodeSingleJSONRejectsTrailingValue(f *testing.F) {
	for _, seed := range []string{
		`{"name":"first"}`,
		`{"name":"first"}{"name":"second"}`,
		" \n {\"name\":\"first\"} \t ",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body string) {
		var destination map[string]any
		if err := DecodeSingleJSON(strings.NewReader(body), &destination); err != nil {
			return
		}

		decoder := json.NewDecoder(strings.NewReader(body))
		var first map[string]any
		if err := decoder.Decode(&first); err != nil {
			t.Fatalf("accepted malformed JSON: %v", err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			t.Fatalf("accepted trailing JSON value: %v", err)
		}
	})
}
