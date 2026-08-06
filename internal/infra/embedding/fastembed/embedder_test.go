package fastembed

import (
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbed_ReturnsNormalized1024Vector(t *testing.T) {
	vec, err := New().Embed("Paris is the capital of France")
	require.NoError(t, err)
	require.Len(t, vec, Dimension)

	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	require.InDelta(t, 1.0, math.Sqrt(norm), 0.0001)
}

func TestEmbed_IsDeterministic(t *testing.T) {
	e := New()
	v1, err := e.Embed("hello world")
	require.NoError(t, err)
	v2, err := e.Embed("hello world")
	require.NoError(t, err)
	require.Equal(t, v1, v2)
}

func TestEmbed_RejectsEmptyText(t *testing.T) {
	_, err := New().Embed("  ...  ")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrEmptyText))
}

func TestFeatureIndexStaysWithinDimension(t *testing.T) {
	idx, ok := featureIndex("tok:overflow-boundary", Dimension)
	require.True(t, ok)
	require.GreaterOrEqual(t, idx, 0)
	require.Less(t, idx, Dimension)
}

func TestFeatureIndexRejectsEmptyDimension(t *testing.T) {
	idx, ok := featureIndex("tok:empty", 0)
	require.False(t, ok)
	require.Zero(t, idx)
}
