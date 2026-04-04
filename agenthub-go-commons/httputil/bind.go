package httputil

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// BindJSON decodes the request body into v and returns an error if decoding fails.
func BindJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is required")
	}
	defer r.Body.Close()

	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

// Validate validates the struct v using go-playground/validator tags.
// Returns a human-readable error string on failure.
func Validate(v any) error {
	return validate.Struct(v)
}

// BindAndValidate decodes the request body and validates the result.
func BindAndValidate(r *http.Request, v any) error {
	if err := BindJSON(r, v); err != nil {
		return err
	}
	return Validate(v)
}
