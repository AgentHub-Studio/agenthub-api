package chat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// HashModelConfig returns a stable SHA-256 digest for a model config JSON payload.
func HashModelConfig(config json.RawMessage) string {
	if len(config) == 0 {
		config = json.RawMessage(`{}`)
	}
	var v interface{}
	if err := json.Unmarshal(config, &v); err != nil {
		sum := sha256.Sum256(config)
		return hex.EncodeToString(sum[:])
	}
	normalised, err := json.Marshal(v)
	if err != nil {
		sum := sha256.Sum256(config)
		return hex.EncodeToString(sum[:])
	}
	sum := sha256.Sum256(normalised)
	return hex.EncodeToString(sum[:])
}
