package knowledgebase

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func FuzzDecodeJSONRequestRejectsTrailingValue(f *testing.F) {
	for _, suffix := range [][]byte{
		[]byte(`{"query":"ignored"}`),
		[]byte(`null`),
		[]byte(`malformed`),
		[]byte("\n\t"),
	} {
		f.Add(suffix)
	}

	f.Fuzz(func(t *testing.T, suffix []byte) {
		if len(bytes.TrimSpace(suffix)) == 0 {
			return
		}

		body := append([]byte(`{"query":"release notes"}`), suffix...)
		req := httptest.NewRequest(http.MethodPost, "/api/knowledge-bases/search", bytes.NewReader(body))
		var target struct {
			Query string `json:"query"`
		}
		if err := decodeJSONRequest(req, &target); err == nil {
			t.Fatal("expected a trailing value to be rejected")
		}
	})
}
