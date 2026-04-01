// Package crypto provides AES-256-GCM symmetric encryption helpers.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// ErrInvalidKey is returned when the provided key length is not 32 bytes.
var ErrInvalidKey = errors.New("crypto: key must be 32 bytes for AES-256")

// Encrypt encrypts plaintext using AES-256-GCM with the given key.
// The key must be exactly 32 bytes. Returns a base64-encoded string
// containing the nonce prepended to the ciphertext.
// Returns plaintext unchanged when key is empty (no-op mode for development).
func Encrypt(key, plaintext string) (string, error) {
	if key == "" {
		return plaintext, nil
	}
	if len(key) != 32 {
		return "", ErrInvalidKey
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("crypto: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext produced by Encrypt.
// Returns ciphertext unchanged when key is empty (no-op mode for development).
func Decrypt(key, ciphertext string) (string, error) {
	if key == "" {
		return ciphertext, nil
	}
	if len(key) != 32 {
		return "", ErrInvalidKey
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("crypto: base64 decode: %w", err)
	}

	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", fmt.Errorf("crypto: create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("crypto: ciphertext too short")
	}

	nonce, data := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", fmt.Errorf("crypto: decrypt: %w", err)
	}

	return string(plaintext), nil
}
