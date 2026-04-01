package settings

import (
	"encoding/json"
	"time"
)

// SettingResponse is the JSON response envelope for a setting.
type SettingResponse struct {
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Description string          `json:"description"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// ResponseFrom converts a Setting entity to SettingResponse.
func ResponseFrom(s Setting) SettingResponse {
	return SettingResponse(s)
}

// UpdateSettingRequest is the JSON body for creating or updating a setting.
type UpdateSettingRequest struct {
	Value       json.RawMessage `json:"value"`
	Description *string         `json:"description,omitempty"`
}
