package agentic

import "testing"

func TestIsReadOnlyOperation_AllowedVerbs(t *testing.T) {
	cases := []string{
		`{"operation":"list"}`,
		`{"operation":"LIST"}`,
		`{"operation":" get "}`,
		`{"operation":"read","resource":"agent"}`,
		`{"operation":"show","id":"abc"}`,
		`{"operation":"describe"}`,
		`{"operation":"search","query":"x"}`,
		`{"operation":"count"}`,
		// Field aliases.
		`{"action":"list"}`,
		`{"op":"get"}`,
		`{"verb":"search"}`,
		`{"method":"fetch"}`,
	}
	for _, c := range cases {
		if !isReadOnlyOperation(c) {
			t.Errorf("expected true for %q", c)
		}
	}
}

func TestIsReadOnlyOperation_DestructiveOrUnknown(t *testing.T) {
	cases := []string{
		`{"operation":"delete"}`,
		`{"operation":"create"}`,
		`{"operation":"update"}`,
		`{"operation":"drop"}`,
		`{"operation":"publish"}`,
		`{"operation":"archive"}`,
		`{"operation":"clone"}`,    // mutating
		`{"operation":"reset"}`,    // unclear — don't silently allow
		`{"operation":"rebuild"}`,  // destructive in practice
	}
	for _, c := range cases {
		if isReadOnlyOperation(c) {
			t.Errorf("expected false for destructive/unknown %q", c)
		}
	}
}

func TestIsReadOnlyOperation_NoOperationField(t *testing.T) {
	cases := []string{
		`{}`,
		`{"id":"abc"}`,
		`{"query":"list"}`, // "list" is in value but not an operation field
	}
	for _, c := range cases {
		if isReadOnlyOperation(c) {
			t.Errorf("expected false when operation field is missing, got true for %q", c)
		}
	}
}

func TestIsReadOnlyOperation_MalformedInput(t *testing.T) {
	cases := []string{
		``,
		`   `,
		`not-json`,
		`{"operation":`,
		`null`,
		`"list"`,                     // bare string
		`{"operation":123}`,          // non-string value
		`[{"operation":"list"}]`,     // array at top level — reject
	}
	for _, c := range cases {
		if isReadOnlyOperation(c) {
			t.Errorf("expected false for malformed %q", c)
		}
	}
}

func TestIsReadOnlyOperation_OperationAsObjectIgnored(t *testing.T) {
	// "operation" must be a plain string. Nested shapes are not tool calls
	// we recognise — keep the conservative default.
	if isReadOnlyOperation(`{"operation":{"verb":"list"}}`) {
		t.Errorf("nested operation object must not be treated as read-only")
	}
}
