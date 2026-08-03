package knowledge

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var metadataFilterSQLPlaceholder = regexp.MustCompile(`\$(\d+)`)

func TestParseMetadataFilter_BooleanGroupsAndTagOperators(t *testing.T) {
	filter, err := ParseMetadataFilter(json.RawMessage(`{
  "all": [
    {"field":"source","op":"eq","value":"manual"},
    {"any":[
      {"field":"tags","op":"containsAny","value":["release","api"]},
      {"not":{"field":"retired","op":"exists"}}
    ]}
  ]
}`))

	require.NoError(t, err)
	sql, args := filter.SQL(2)
	assert.Contains(t, sql, "d.metadata #> $3::text[]")
	assert.Contains(t, sql, "?| $6::text[]")
	assert.Contains(t, sql, "NOT ((d.metadata #> $7::text[]) IS NOT NULL)")
	assert.Equal(t, []any{[]string{"source"}, `"manual"`, []string{"tags"}, []string{"release", "api"}, []string{"retired"}}, args)
}

func TestParseMetadataFilter_InPreservesTypedEquality(t *testing.T) {
	filter, err := ParseMetadataFilter(json.RawMessage(`{"field":"year","op":"in","value":[2025,2026]}`))

	require.NoError(t, err)
	sql, args := filter.SQL(0)
	assert.Equal(t, "((d.metadata #> $1::text[]) IN ($2::jsonb, $3::jsonb))", sql)
	assert.Equal(t, []any{[]string{"year"}, "2025", "2026"}, args)
}

func TestParseMetadataFilter_EqAcceptsStringArray(t *testing.T) {
	filter, err := ParseMetadataFilter(json.RawMessage(`{"field":"tags","op":"eq","value":["release","api"]}`))

	require.NoError(t, err)
	sql, args := filter.SQL(0)
	assert.Equal(t, "((d.metadata #> $1::text[]) = $2::jsonb)", sql)
	assert.Equal(t, []any{[]string{"tags"}, `["release","api"]`}, args)
}

func TestParseMetadataFilter_EqStringArrayRespectsValueLimit(t *testing.T) {
	values := make([]string, maxMetadataFilterValues+1)
	for index := range values {
		values[index] = fmt.Sprintf("tag-%d", index)
	}
	raw, err := json.Marshal(map[string]any{
		"field": "tags",
		"op":    "eq",
		"value": values,
	})
	require.NoError(t, err)

	_, err = ParseMetadataFilter(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at most 64 items")
}

func TestParseMetadataFilter_RejectsInvalidShapesAndValues(t *testing.T) {
	for name, raw := range map[string]string{
		"nested field object":   `{"field":"source.name","op":"eq","value":{"name":"manual"}}`,
		"exists with value":     `{"field":"source","op":"exists","value":true}`,
		"mixed predicate group": `{"all":[],"field":"source","op":"exists"}`,
		"in contains array":     `{"field":"tags","op":"in","value":[["release"]]}`,
		"contains non string":   `{"field":"tags","op":"containsAll","value":["release",2]}`,
		"contains null":         `{"field":"tags","op":"containsAll","value":[null]}`,
		"unknown operator":      `{"field":"source","op":"matches","value":"manual"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseMetadataFilter(json.RawMessage(raw))
			require.Error(t, err)
		})
	}
}

func TestParseMetadataFilter_AcceptsParameterizedSQLLikeOperatorsAtTwoLevels(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		sql  string
		args []any
	}{
		{
			name: "not equal",
			raw:  `{"field":"customer.region","op":"neq","value":"br"}`,
			sql:  "((d.metadata #> $1::text[]) <> $2::jsonb)",
			args: []any{[]string{"customer", "region"}, `"br"`},
		},
		{
			name: "not in",
			raw:  `{"field":"customer.tier","op":"notIn","value":[1,2]}`,
			sql:  "((d.metadata #> $1::text[]) NOT IN ($2::jsonb, $3::jsonb))",
			args: []any{[]string{"customer", "tier"}, "1", "2"},
		},
		{
			name: "not exists",
			raw:  `{"field":"customer.archived","op":"notExists"}`,
			sql:  "((d.metadata #> $1::text[]) IS NULL)",
			args: []any{[]string{"customer", "archived"}},
		},
		{
			name: "numeric comparison",
			raw:  `{"field":"customer.tier","op":"gte","value":2}`,
			sql:  "((jsonb_typeof(d.metadata #> $1::text[]) = 'number') AND ((d.metadata #>> $1::text[])::numeric >= $2::numeric))",
			args: []any{[]string{"customer", "tier"}, "2"},
		},
		{
			name: "case insensitive pattern",
			raw:  `{"field":"customer.region","op":"ilike","value":"BR%"}`,
			sql:  "((jsonb_typeof(d.metadata #> $1::text[]) = 'string') AND ((d.metadata #>> $1::text[]) ILIKE $2))",
			args: []any{[]string{"customer", "region"}, "BR%"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseMetadataFilter(json.RawMessage(tt.raw))
			require.NoError(t, err)

			sql, args := filter.SQL(0)
			assert.Equal(t, tt.sql, sql)
			assert.Equal(t, tt.args, args)
		})
	}
}

func TestParseMetadataFilter_RejectsInvalidSQLLikeOperatorValues(t *testing.T) {
	for name, raw := range map[string]string{
		"third field level":     `{"field":"customer.profile.region","op":"eq","value":"br"}`,
		"numeric string":        `{"field":"customer.tier","op":"gt","value":"2"}`,
		"pattern number":        `{"field":"customer.region","op":"like","value":2}`,
		"not exists with value": `{"field":"customer.archived","op":"notExists","value":false}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseMetadataFilter(json.RawMessage(raw))
			require.Error(t, err)
		})
	}
}

func TestParseMetadataFilter_SupportsEveryDefinedOperator(t *testing.T) {
	filters := []string{
		`{"field":"customer.region","op":"eq","value":"br"}`,
		`{"field":"customer.region","op":"neq","value":"br"}`,
		`{"field":"customer.region","op":"in","value":["br","us"]}`,
		`{"field":"customer.region","op":"notIn","value":["br","us"]}`,
		`{"field":"customer.region","op":"exists"}`,
		`{"field":"customer.region","op":"notExists"}`,
		`{"field":"customer.tier","op":"gt","value":1}`,
		`{"field":"customer.tier","op":"gte","value":1}`,
		`{"field":"customer.tier","op":"lt","value":3}`,
		`{"field":"customer.tier","op":"lte","value":3}`,
		`{"field":"customer.region","op":"like","value":"b%"}`,
		`{"field":"customer.region","op":"ilike","value":"B%"}`,
		`{"field":"customer.labels","op":"containsAny","value":["priority"]}`,
		`{"field":"customer.labels","op":"containsAll","value":["priority"]}`,
	}

	for _, raw := range filters {
		filter, err := ParseMetadataFilter(json.RawMessage(raw))
		require.NoError(t, err, raw)

		sql, args := filter.SQL(0)
		assert.NotEqual(t, "(FALSE)", sql, raw)
		assertMetadataFilterSQLParameters(t, sql, args)
	}
}

func TestParseMetadataFilter_RejectsDuplicateObjectKeysRecursively(t *testing.T) {
	for name, raw := range map[string]string{
		"predicate key":    `{"field":"source","field":"owner","op":"exists"}`,
		"nested child key": `{"all":[{"field":"source","op":"eq","value":"first","value":"second"}]}`,
		"escaped key":      `{"field":"source","op":"exists","\u006f\u0070":"eq"}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseMetadataFilter(json.RawMessage(raw))

			require.Error(t, err)
			assert.Contains(t, err.Error(), "duplicate JSON object key")
		})
	}
}

func TestParseMetadataFilter_RejectsNumbersOutsidePostgresJSONBRange(t *testing.T) {
	for name, raw := range map[string]string{
		"eq too many integer digits":    `{"field":"year","op":"eq","value":1e131072}`,
		"in too many fractional digits": `{"field":"ratio","op":"in","value":[1e-16384]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseMetadataFilter(json.RawMessage(raw))
			require.Error(t, err)
			assert.Contains(t, err.Error(), "PostgreSQL JSONB numeric range")
		})
	}
}

func TestParseMetadataFilter_RejectsPostgresJSONBNullCharacter(t *testing.T) {
	for name, raw := range map[string]string{
		"field":          `{"field":"source\u0000name","op":"exists"}`,
		"equality value": `{"field":"source","op":"eq","value":"manual\u0000draft"}`,
		"in value":       `{"field":"source","op":"in","value":["manual\u0000draft"]}`,
		"tag value":      `{"field":"tags","op":"containsAny","value":["release\u0000draft"]}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseMetadataFilter(json.RawMessage(raw))

			require.Error(t, err)
			assert.Contains(t, err.Error(), "U+0000")
		})
	}
}

func TestParseMetadataFilter_EnforcesEncodedSizeLimit(t *testing.T) {
	const prefix = `{"field":"source","op":"eq","value":"`
	const suffix = `"}`
	valueLength := MaxMetadataFilterBytes - len(prefix) - len(suffix)

	atLimit := json.RawMessage(prefix + strings.Repeat("x", valueLength) + suffix)
	filter, err := ParseMetadataFilter(atLimit)
	require.NoError(t, err)
	assert.NotNil(t, filter)

	overLimit := json.RawMessage(prefix + strings.Repeat("x", valueLength+1) + suffix)
	_, err = ParseMetadataFilter(overLimit)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maximum size")
}

func TestParseMetadataFilter_AcceptsPostgresJSONBNumericBoundaries(t *testing.T) {
	filter, err := ParseMetadataFilter(json.RawMessage(`{"all":[{"field":"integer","op":"eq","value":1e131071},{"field":"fraction","op":"in","value":[1e-16383]}]}`))

	require.NoError(t, err)
	assert.NotNil(t, filter)
}

func TestParseMetadataFilter_EmptyInHasZeroMatchPredicate(t *testing.T) {
	filter, err := ParseMetadataFilter(json.RawMessage(`{"field":"source","op":"in","value":[]}`))

	require.NoError(t, err)
	sql, args := filter.SQL(0)
	assert.Equal(t, "(FALSE)", sql)
	assert.Empty(t, args)
}

func TestParseMetadataFilter_EmptyTagListsHaveExplicitSetSemantics(t *testing.T) {
	tests := []struct {
		name string
		op   string
		sql  string
	}{
		{name: "contains any", op: metadataOpContainsAny, sql: "(FALSE)"},
		{name: "contains all", op: metadataOpContainsAll, sql: "(TRUE)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter, err := ParseMetadataFilter(json.RawMessage(`{"field":"tags","op":"` + tt.op + `","value":[]}`))

			require.NoError(t, err)
			sql, args := filter.SQL(0)
			assert.Equal(t, tt.sql, sql)
			assert.Empty(t, args)
		})
	}
}

func TestParseMetadataFilter_RejectsExcessiveComplexity(t *testing.T) {
	deeplyNested := `{"field":"active","op":"exists"}`
	for range maxMetadataFilterDepth {
		deeplyNested = `{"not":` + deeplyNested + `}`
	}

	tooManyChildren := make([]string, maxMetadataFilterChildren+1)
	for i := range tooManyChildren {
		tooManyChildren[i] = `{"field":"active","op":"exists"}`
	}

	tooManyValues := make([]string, maxMetadataFilterValues+1)
	for i := range tooManyValues {
		tooManyValues[i] = fmt.Sprintf("%d", i)
	}

	maxPredicateGroup := func(count int) string {
		predicates := make([]string, count)
		for i := range predicates {
			predicates[i] = fmt.Sprintf(`{"field":"f%d","op":"exists"}`, i)
		}
		return `{"any":[` + strings.Join(predicates, ",") + `]}`
	}
	tooManyPredicates := `{"all":[` + strings.Join([]string{
		maxPredicateGroup(maxMetadataFilterChildren),
		maxPredicateGroup(maxMetadataFilterChildren),
		`{"field":"last","op":"exists"}`,
	}, ",") + `]}`

	for name, raw := range map[string]string{
		"nesting depth":    deeplyNested,
		"group children":   `{"all":[` + strings.Join(tooManyChildren, ",") + `]}`,
		"predicate values": `{"field":"year","op":"in","value":[` + strings.Join(tooManyValues, ",") + `]}`,
		"total predicates": tooManyPredicates,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParseMetadataFilter(json.RawMessage(raw))
			require.Error(t, err)
		})
	}
}

func TestMetadataFilter_SQLKeepsUntrustedInputParameterized(t *testing.T) {
	const field = "source') OR TRUE --"
	filter, err := ParseMetadataFilter(json.RawMessage(`{"field":"source') OR TRUE --","op":"eq","value":"manual' OR 1=1 --"}`))

	require.NoError(t, err)
	sql, args := filter.SQL(0)
	assert.Equal(t, "((d.metadata #> $1::text[]) = $2::jsonb)", sql)
	assert.NotContains(t, sql, field)
	assert.NotContains(t, sql, "manual' OR 1=1")
	assert.Equal(t, []any{[]string{field}, `"manual' OR 1=1 --"`}, args)
}

func FuzzParseMetadataFilter(f *testing.F) {
	for _, seed := range []string{
		`{"field":"source","op":"eq","value":"manual"}`,
		`{"all":[{"field":"year","op":"in","value":[2025,2026]}]}`,
		`{"any":[{"field":"tags","op":"containsAny","value":["release"]}]}`,
		`{"not":{"field":"retired","op":"exists"}}`,
		`{"field":"source') OR TRUE --","op":"eq","value":"manual' OR 1=1 --"}`,
		`{"all":[{"field":"source","op":"in","value":[]}]}`,
		`{"field":"tags","op":"containsAll","value":[null]}`,
		`{"field":"source","op":"eq","value":"manual\u0000draft"}`,
		`{"field":"source","op":"eq","value":"` + strings.Repeat("x", MaxMetadataFilterBytes) + `"}`,
		`null`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, raw string) {
		filter, err := ParseMetadataFilter(json.RawMessage(raw))
		if err != nil || filter == nil {
			return
		}

		sql, args := filter.SQL(0)
		if sql == "" {
			t.Fatal("accepted filter must generate a SQL predicate")
		}
		assertMetadataFilterSQLParameters(t, sql, args)
	})
}

func assertMetadataFilterSQLParameters(t *testing.T, sql string, args []any) {
	t.Helper()
	used := make([]bool, len(args)+1)
	for _, match := range metadataFilterSQLPlaceholder.FindAllStringSubmatch(sql, -1) {
		index, err := strconv.Atoi(match[1])
		if err != nil || index < 1 || index > len(args) {
			t.Fatalf("SQL placeholder %q is outside the argument range %d", match[0], len(args))
		}
		used[index] = true
	}
	for index := 1; index <= len(args); index++ {
		if !used[index] {
			t.Fatalf("argument %d has no SQL placeholder", index)
		}
	}
}
