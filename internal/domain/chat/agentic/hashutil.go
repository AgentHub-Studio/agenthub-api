package agentic

import (
	"crypto/sha256"
	"encoding/hex"
	"hash/fnv"
)

// Non-cryptographic and cryptographic hash utilities.
//
// Inspired by Claude Code's hash.ts — provides deterministic
// string hashing for cache keys, content change detection,
// and pair hashing without string concatenation.

// DJB2Hash computes the djb2 hash of a string, returning a
// signed 32-bit integer. Deterministic and fast, suitable for
// hash maps and cache directory names.
func DJB2Hash(s string) int32 {
	var hash int32
	for i := 0; i < len(s); i++ {
		hash = ((hash << 5) - hash + int32(s[i]))
	}
	return hash
}

// FNV1aHash computes the FNV-1a 64-bit hash of a string.
// Better distribution than DJB2 for hash table use.
func FNV1aHash(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// SHA256Hex returns a SHA-256 hex digest of the content.
// Suitable for content change detection.
func SHA256Hex(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// HashPair hashes two strings without concatenation, using
// a null byte separator to disambiguate ("ts","code") from
// ("tsc","ode").
func HashPair(a, b string) string {
	h := sha256.New()
	h.Write([]byte(a))
	h.Write([]byte{0})
	h.Write([]byte(b))
	return hex.EncodeToString(h.Sum(nil))
}
