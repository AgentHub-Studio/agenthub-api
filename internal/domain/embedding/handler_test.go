package embedding

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
)

func TestHandler_EmbedFastEmbedReturnsVector(t *testing.T) {
	body := `{"text":"hello world","provider":"fastembed"}`
	rec := postEmbedding(t, body, Config{DefaultProvider: ProviderFastEmbed})

	require.Equal(t, http.StatusOK, rec.Code)
	var parsed Result
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&parsed))
	require.Equal(t, ProviderFastEmbed, parsed.Provider)
	require.Equal(t, ProviderFastEmbed, parsed.SourceProvider)
	require.Equal(t, 1024, parsed.Dimension)
	require.Len(t, parsed.Vector, 1024)
}

func TestHandler_EmbedAcceptsNestedBodyPayload(t *testing.T) {
	body := `{"body":{"text":"hello world","provider":"fastembed"}}`
	rec := postEmbedding(t, body, Config{DefaultProvider: ProviderFastEmbed})

	require.Equal(t, http.StatusOK, rec.Code)
	var parsed Result
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&parsed))
	require.Len(t, parsed.Vector, 1024)
}

func TestHandler_DefaultProviderFastEmbed(t *testing.T) {
	body := `{"text":"hello world"}`
	rec := postEmbedding(t, body, Config{DefaultProvider: ProviderFastEmbed})

	require.Equal(t, http.StatusOK, rec.Code)
	var parsed Result
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&parsed))
	require.Equal(t, ProviderFastEmbed, parsed.Provider)
}

func TestHandler_PythonProviderFallsBackToFastEmbedWhenConfigured(t *testing.T) {
	fastRec := postEmbedding(t, `{"text":"Paris is the capital of France","provider":"fastembed"}`, Config{DefaultProvider: ProviderFastEmbed})
	pythonRec := postEmbedding(t, `{"text":"Paris is the capital of France","provider":"python"}`, Config{DefaultProvider: ProviderFastEmbed})

	require.Equal(t, http.StatusOK, fastRec.Code)
	require.Equal(t, http.StatusOK, pythonRec.Code)

	var fastResult Result
	var pythonResult Result
	require.NoError(t, json.NewDecoder(fastRec.Body).Decode(&fastResult))
	require.NoError(t, json.NewDecoder(pythonRec.Body).Decode(&pythonResult))
	require.Greater(t, cosine(fastResult.Vector, pythonResult.Vector), 0.98)
	require.Equal(t, ProviderPython, pythonResult.Provider)
	require.Equal(t, ProviderFastEmbed, pythonResult.SourceProvider)
}

func TestHandler_RejectsUnknownProvider(t *testing.T) {
	rec := postEmbedding(t, `{"text":"hello","provider":"bogus"}`, Config{DefaultProvider: ProviderFastEmbed})
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestHandler_RejectsTrailingJSONWithoutCallingEmbeddingProvider(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"embedding":[0.1],"model":"test","dimension":1}`))
	}))
	t.Cleanup(upstream.Close)

	rec := postEmbedding(t, `{"text":"hello","provider":"python"} {"text":"ignored"}`, Config{
		DefaultProvider: ProviderPython,
		PythonURL:       upstream.URL,
	})

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Zero(t, calls)
}

func postEmbedding(t *testing.T, body string, cfg Config) *httptest.ResponseRecorder {
	t.Helper()

	r := chi.NewRouter()
	NewHandler(NewService(cfg)).RegisterRoutes(r)

	req := httptest.NewRequest(http.MethodPost, "/api/embeddings", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func cosine(a, b []float32) float64 {
	var dot, n1, n2 float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		n1 += float64(a[i]) * float64(a[i])
		n2 += float64(b[i]) * float64(b[i])
	}
	return dot / (math.Sqrt(n1) * math.Sqrt(n2))
}
