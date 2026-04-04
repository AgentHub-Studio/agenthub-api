package agentic

import (
	"fmt"
	"math/big"
	"strings"
)

// Tagged ID encoding compatible with Anthropic's tagged_id format.
//
// Inspired by Claude Code's taggedId.ts — produces IDs like
// "user_01PaGUP2rbg1XDh7Z9W1CEpd" from a UUID string. The format is
// {tag}_{version}{base58(uuid_as_128bit_int)}. Base58 avoids confusing
// characters (no 0/O/l/I). Useful for API-compatible identifiers,
// session IDs, and audit trails.

const (
	base58Chars   = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	taggedVersion = "01"
	encodedLength = 22 // ceil(128 / log2(58))
)

var base58Base = big.NewInt(int64(len(base58Chars)))

// base58Encode encodes a 128-bit unsigned integer as a fixed-length base58 string.
func base58EncodeUint128(n *big.Int) string {
	result := make([]byte, encodedLength)
	for i := range result {
		result[i] = base58Chars[0]
	}

	val := new(big.Int).Set(n)
	rem := new(big.Int)
	i := encodedLength - 1
	for val.Sign() > 0 && i >= 0 {
		val.DivMod(val, base58Base, rem)
		result[i] = base58Chars[int(rem.Int64())]
		i--
	}

	return string(result)
}

// base58DecodeUint128 decodes a fixed-length base58 string to a 128-bit integer.
func base58DecodeUint128(s string) (*big.Int, error) {
	if len(s) != encodedLength {
		return nil, fmt.Errorf("expected %d chars, got %d", encodedLength, len(s))
	}

	n := new(big.Int)
	for _, c := range s {
		idx := strings.IndexRune(base58Chars, c)
		if idx < 0 {
			return nil, fmt.Errorf("invalid base58 character %q", c)
		}
		n.Mul(n, base58Base)
		n.Add(n, big.NewInt(int64(idx)))
	}
	return n, nil
}

// uuidToInt128 parses a UUID string (with or without hyphens) to a big.Int.
func uuidToInt128(uuid string) (*big.Int, error) {
	hex := strings.ReplaceAll(uuid, "-", "")
	if len(hex) != 32 {
		return nil, fmt.Errorf("invalid UUID hex length: %d", len(hex))
	}
	n, ok := new(big.Int).SetString(hex, 16)
	if !ok {
		return nil, fmt.Errorf("invalid hex string: %s", hex)
	}
	return n, nil
}

// int128ToUUID converts a big.Int back to a UUID string with hyphens.
func int128ToUUID(n *big.Int) string {
	hex := fmt.Sprintf("%032x", n)
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex[0:8], hex[8:12], hex[12:16], hex[16:20], hex[20:32])
}

// ToTaggedID converts a UUID to a tagged ID in the API's format.
// Example: ToTaggedID("user", "550e8400-e29b-41d4-a716-446655440000")
// returns "user_01..." (a compact, URL-safe identifier).
func ToTaggedID(tag, uuid string) (string, error) {
	n, err := uuidToInt128(uuid)
	if err != nil {
		return "", err
	}
	return tag + "_" + taggedVersion + base58EncodeUint128(n), nil
}

// ParseTaggedID extracts the tag and UUID from a tagged ID string.
// Returns the tag, the original UUID (with hyphens), and any error.
func ParseTaggedID(id string) (tag, uuid string, err error) {
	idx := strings.Index(id, "_")
	if idx < 0 {
		return "", "", fmt.Errorf("missing tag separator '_' in %q", id)
	}
	tag = id[:idx]
	rest := id[idx+1:]

	if len(rest) < 2 {
		return "", "", fmt.Errorf("missing version prefix in %q", id)
	}
	version := rest[:2]
	if version != taggedVersion {
		return "", "", fmt.Errorf("unsupported version %q", version)
	}

	encoded := rest[2:]
	n, err := base58DecodeUint128(encoded)
	if err != nil {
		return "", "", fmt.Errorf("decode error: %w", err)
	}

	return tag, int128ToUUID(n), nil
}
