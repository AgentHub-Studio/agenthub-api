package approval

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func FuzzDecodeSingleJSONRejectsTrailingValue(f *testing.F) {
	for _, seed := range []string{
		`{"approved":true}`,
		`{"approved":true}{"approved":false}`,
		" \n {\"approved\":true} \t ",
		"",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, body string) {
		var request RespondRequest
		err := decodeSingleJSON(strings.NewReader(body), &request)
		if err != nil {
			return
		}

		decoder := json.NewDecoder(strings.NewReader(body))
		var first RespondRequest
		if err := decoder.Decode(&first); err != nil {
			t.Fatalf("accepted malformed JSON: %v", err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			t.Fatalf("accepted trailing JSON value: %v", err)
		}
	})
}
