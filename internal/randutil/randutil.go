// Package randutil provides small crypto-backed random helpers for runtime
// sampling and jitter paths.
package randutil

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"math/big"
)

const float64Unit = 1 << 53

// Float64 returns a crypto-backed random float in [0, 1).
func Float64() float64 {
	var b [8]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		return 0.5
	}
	n := binary.BigEndian.Uint64(b[:]) >> 11
	return float64(n) / float64(float64Unit)
}

// Intn returns a crypto-backed integer in [0, n).
func Intn(n int) int {
	if n <= 0 {
		panic("randutil: non-positive Intn argument")
	}
	v, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return n / 2
	}
	return int(v.Int64())
}
