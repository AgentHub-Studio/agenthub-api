// Package metadata defines the constrained document metadata contract.
package metadata

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/AgentHub-Studio/agenthub-api/internal/httputil"
)

const (
	maxDocumentMetadataBytes = 16 << 10

	// PostgreSQL stores jsonb numbers as numeric. Values beyond these bounds
	// are valid JSON but fail only when PostgreSQL converts them to jsonb.
	maxPostgresNumericIntegerDigits    = 131072
	maxPostgresNumericFractionalDigits = 16383
)

// ParseDocument validates and normalizes document metadata. Metadata is a
// top-level JSON object whose values are strings, numbers, booleans, string
// arrays, or an object containing those leaf values. An omitted value is
// normalized to an empty object.
func ParseDocument(raw []byte) (json.RawMessage, error) {
	if len(raw) > maxDocumentMetadataBytes {
		return nil, fmt.Errorf("metadata exceeds maximum size of %d bytes", maxDocumentMetadataBytes)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{}`), nil
	}

	validated, err := httputil.DecodeSingleRawJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("metadata must contain one JSON object with unique fields: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(validated))
	decoder.UseNumber()

	var values map[string]any
	if err := decoder.Decode(&values); err != nil {
		return nil, fmt.Errorf("metadata must be a JSON object: %w", err)
	}
	if values == nil {
		return nil, fmt.Errorf("metadata must be a JSON object")
	}
	for field, value := range values {
		if err := validateFieldSegment(field); err != nil {
			return nil, err
		}
		if err := validateDocumentValue(value); err != nil {
			return nil, fmt.Errorf("metadata field %q: %w", field, err)
		}
	}

	normalized, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshal metadata: %w", err)
	}
	return json.RawMessage(normalized), nil
}

func validateDocumentValue(value any) error {
	if object, ok := value.(map[string]any); ok {
		if len(object) == 0 {
			return fmt.Errorf("nested objects must not be empty")
		}
		for field, nestedValue := range object {
			if err := validateFieldSegment(field); err != nil {
				return err
			}
			if err := validateValue(nestedValue); err != nil {
				return fmt.Errorf("nested field %q: %w", field, err)
			}
		}
		return nil
	}
	return validateValue(value)
}

func validateValue(value any) error {
	switch typed := value.(type) {
	case string:
		return validatePostgresJSONBString(typed)
	case bool:
		return nil
	case json.Number:
		return validatePostgresJSONBNumber(typed)
	case []any:
		for _, item := range typed {
			stringValue, ok := item.(string)
			if !ok {
				return fmt.Errorf("arrays must contain only strings")
			}
			if err := validatePostgresJSONBString(stringValue); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("value must be a string, number, boolean, or string array")
	}
}

func validatePostgresJSONBString(value string) error {
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("must not contain the U+0000 character")
	}
	return nil
}

func validatePostgresJSONBNumber(number json.Number) error {
	raw := number.String()
	if raw == "" {
		return fmt.Errorf("number must not be empty")
	}
	if raw[0] == '-' {
		raw = raw[1:]
	}
	if raw == "" {
		return fmt.Errorf("number must contain digits")
	}

	exponent := 0
	if exponentIndex := strings.IndexAny(raw, "eE"); exponentIndex >= 0 {
		if strings.ContainsAny(raw[exponentIndex+1:], "eE") {
			return fmt.Errorf("number must contain at most one exponent")
		}
		parsed, err := parsePostgresNumericExponent(raw[exponentIndex+1:])
		if err != nil {
			return err
		}
		exponent = parsed
		raw = raw[:exponentIndex]
	}

	integerPart := raw
	fractionalPart := ""
	if decimalIndex := strings.IndexByte(raw, '.'); decimalIndex >= 0 {
		if strings.IndexByte(raw[decimalIndex+1:], '.') >= 0 {
			return fmt.Errorf("number must contain at most one decimal point")
		}
		integerPart = raw[:decimalIndex]
		fractionalPart = raw[decimalIndex+1:]
	}
	if integerPart == "" || !isDecimalDigits(integerPart) || (fractionalPart != "" && !isDecimalDigits(fractionalPart)) {
		return fmt.Errorf("number must be a decimal JSON value")
	}

	digits := integerPart + fractionalPart
	leadingZeros := len(digits) - len(strings.TrimLeft(digits, "0"))
	if leadingZeros == len(digits) {
		return nil
	}

	decimalPoint := len(integerPart) + exponent - leadingZeros
	significantDigits := len(digits) - leadingZeros
	integerDigits := 0
	if decimalPoint > 0 {
		integerDigits = decimalPoint
	}
	fractionalDigits := 0
	if decimalPoint < significantDigits {
		fractionalDigits = significantDigits - decimalPoint
	}
	if integerDigits > maxPostgresNumericIntegerDigits || fractionalDigits > maxPostgresNumericFractionalDigits {
		return fmt.Errorf("number exceeds PostgreSQL JSONB numeric range")
	}
	return nil
}

func parsePostgresNumericExponent(raw string) (int, error) {
	if raw == "" {
		return 0, fmt.Errorf("number exponent must contain digits")
	}
	sign := 1
	if raw[0] == '+' || raw[0] == '-' {
		if raw[0] == '-' {
			sign = -1
		}
		raw = raw[1:]
	}
	if raw == "" || !isDecimalDigits(raw) {
		return 0, fmt.Errorf("number exponent must contain only digits")
	}
	raw = strings.TrimLeft(raw, "0")
	if raw == "" {
		return 0, nil
	}
	if len(raw) > 6 {
		return 0, fmt.Errorf("number exceeds PostgreSQL JSONB numeric range")
	}
	exponent, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("number exponent is invalid: %w", err)
	}
	return sign * exponent, nil
}

func isDecimalDigits(value string) bool {
	for _, char := range value {
		if char < '0' || char > '9' {
			return false
		}
	}
	return true
}

// ValidateValue exposes the metadata value restrictions to filter validation.
func ValidateValue(value any) error {
	return validateDocumentValue(value)
}

// ValidateFieldName exposes the metadata field restrictions to filter validation.
func ValidateFieldName(field string) error {
	if strings.TrimSpace(field) == "" {
		return fmt.Errorf("metadata field must not be empty")
	}
	return validatePostgresJSONBString(field)
}

// ParseFieldPath validates a dotted metadata path with one or two levels.
// Each segment is sent to PostgreSQL as a parameter, never interpolated into
// SQL text.
func ParseFieldPath(field string) ([]string, error) {
	parts := strings.Split(field, ".")
	if len(parts) > 2 {
		return nil, fmt.Errorf("metadata field path must contain at most two levels")
	}
	for _, part := range parts {
		if err := validateFieldSegment(part); err != nil {
			return nil, err
		}
	}
	return parts, nil
}

func validateFieldSegment(field string) error {
	if strings.Contains(field, ".") {
		return fmt.Errorf("metadata field segment must not contain a dot")
	}
	return ValidateFieldName(field)
}

// IsScalar reports whether a metadata value can be compared with the in
// operator. String arrays are intentionally excluded.
func IsScalar(value any) bool {
	switch value.(type) {
	case string, bool, json.Number:
		return true
	default:
		return false
	}
}
