package abtest

import (
	"strings"
	"testing"
)

func FuzzDecodeSingleJSONRejectsTrailingValue(f *testing.F) {
	f.Add("")
	f.Add(" \n\t")
	f.Add(`{"name":"second"}`)
	f.Add("garbage")

	f.Fuzz(func(t *testing.T, suffix string) {
		var request struct {
			Name string `json:"name"`
		}
		err := decodeSingleJSON(strings.NewReader(`{"name":"first"}`+suffix), &request)
		if isJSONWhitespaceOnly(suffix) {
			if err != nil {
				t.Fatalf("whitespace suffix must be accepted: %v", err)
			}
			return
		}
		if err == nil {
			t.Fatalf("non-empty suffix must be rejected: %q", suffix)
		}
	})
}

func isJSONWhitespaceOnly(value string) bool {
	for _, char := range value {
		switch char {
		case ' ', '\n', '\r', '\t':
		default:
			return false
		}
	}
	return true
}
