package workflow

import (
	"encoding/json"
	"io"
	"strings"
	"testing"
)

func FuzzDecodeSingleJSONRejectsTrailingValue(f *testing.F) {
	for _, seed := range []string{
		`{"input":{}}`,
		`{"input":{}}{"ignored":true}`,
		`{"input":{}} \n\t`,
		``,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, body string) {
		var dst any
		err := decodeSingleJSON(strings.NewReader(body), &dst)
		if err != nil {
			return
		}
		decoder := json.NewDecoder(strings.NewReader(body))
		var first any
		if err := decoder.Decode(&first); err != nil {
			t.Fatalf("accepted body must decode once: %v", err)
		}
		var second any
		if err := decoder.Decode(&second); err != io.EOF {
			t.Fatalf("accepted body contains a trailing value or invalid suffix: %v", err)
		}
	})
}
