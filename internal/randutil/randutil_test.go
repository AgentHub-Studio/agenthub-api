package randutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFloat64Range(t *testing.T) {
	for range 100 {
		v := Float64()
		assert.GreaterOrEqual(t, v, 0.0)
		assert.Less(t, v, 1.0)
	}
}

func TestIntnRange(t *testing.T) {
	for range 100 {
		v := Intn(7)
		assert.GreaterOrEqual(t, v, 0)
		assert.Less(t, v, 7)
	}
}

func TestIntnPanicsForNonPositiveLimit(t *testing.T) {
	require.Panics(t, func() {
		Intn(0)
	})
}
