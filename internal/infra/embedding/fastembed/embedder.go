package fastembed

import (
	"errors"
	"math"
	"strings"
	"unicode"
)

const (
	Dimension = 1024
	ModelName = "agenthub-fastembed-e5-compatible-v1"
)

var ErrEmptyText = errors.New("fastembed: text is empty")

// Embedder generates deterministic local embeddings for the FastEmbed provider.
type Embedder struct{}

// New creates a FastEmbed-compatible local embedder.
func New() *Embedder {
	return &Embedder{}
}

// Embed converts text into a normalized 1024-dimensional vector.
func (e *Embedder) Embed(text string) ([]float32, error) {
	tokens := tokenize(text)
	if len(tokens) == 0 {
		return nil, ErrEmptyText
	}

	vec := make([]float32, Dimension)
	for _, token := range tokens {
		addFeature(vec, "tok:"+token, 1)
		if len(token) >= 4 {
			for i := 0; i+3 <= len(token); i++ {
				addFeature(vec, "tri:"+token[i:i+3], 0.35)
			}
		}
	}
	for i := 0; i+1 < len(tokens); i++ {
		addFeature(vec, "bi:"+tokens[i]+" "+tokens[i+1], 0.55)
	}

	normalize(vec)
	return vec, nil
}

func tokenize(text string) []string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return nil
	}

	tokens := make([]string, 0, 32)
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		if b.Len() > 0 {
			tokens = append(tokens, b.String())
			b.Reset()
		}
	}
	if b.Len() > 0 {
		tokens = append(tokens, b.String())
	}
	return tokens
}

func addFeature(vec []float32, feature string, weight float32) {
	idx, ok := featureIndex(feature, len(vec))
	if !ok {
		return
	}
	h := fnv64a(feature)
	if (h>>63)&1 == 0 {
		vec[idx] += weight
		return
	}
	vec[idx] -= weight
}

func featureIndex(feature string, dimension int) (int, bool) {
	if dimension <= 0 {
		return 0, false
	}

	idx64 := fnv64a(feature) % uint64(dimension)
	if idx64 > uint64(math.MaxInt) {
		return 0, false
	}

	return int(idx64), true
}

func normalize(vec []float32) {
	var sum float64
	for _, v := range vec {
		sum += float64(v) * float64(v)
	}
	if sum == 0 {
		vec[0] = 1
		return
	}
	norm := float32(math.Sqrt(sum))
	for i := range vec {
		vec[i] /= norm
	}
}

func fnv64a(s string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	var h uint64 = offset64
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= prime64
	}
	return h
}
