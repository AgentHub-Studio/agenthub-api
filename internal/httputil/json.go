package httputil

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// DecodeSingleJSON decodes exactly one JSON value and rejects trailing values
// and duplicate object keys, which would otherwise be resolved ambiguously by
// encoding/json.
func DecodeSingleJSON(body io.Reader, dst any) error {
	raw, err := DecodeSingleRawJSON(body)
	if err != nil {
		return err
	}

	return json.Unmarshal(raw, dst)
}

// DecodeSingleRawJSON returns exactly one validated JSON value. Object keys
// must be unique at every nesting level, while callers keep control of the
// final decoding options such as json.Decoder.UseNumber.
func DecodeSingleRawJSON(body io.Reader) (json.RawMessage, error) {
	decoder := json.NewDecoder(body)
	var raw json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("unexpected additional JSON value")
		}
		return nil, err
	}

	if err := rejectDuplicateObjectKeys(raw); err != nil {
		return nil, err
	}

	return raw, nil
}

// DecodeOptionalSingleJSON accepts an empty body or exactly one JSON value.
func DecodeOptionalSingleJSON(body io.Reader, dst any) error {
	err := DecodeSingleJSON(body, dst)
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func rejectDuplicateObjectKeys(raw json.RawMessage) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return scanJSONValue(decoder)
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}

	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}

	switch delimiter {
	case '{':
		keys := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("invalid JSON object key")
			}
			if _, exists := keys[key]; exists {
				return errors.New("duplicate JSON object key")
			}
			keys[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return errors.New("invalid JSON delimiter")
	}

	_, err = decoder.Token()
	return err
}
