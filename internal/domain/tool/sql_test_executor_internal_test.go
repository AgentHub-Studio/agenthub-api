package tool

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderSQLTestQuery_MatchesRuntimeParameterContract(t *testing.T) {
	query, args, err := renderSQLTestQuery("SELECT * FROM orders WHERE id = $1 AND state = {{input.state}}", map[string]any{
		"1":     42,
		"state": "paid",
	})

	require.NoError(t, err)
	assert.Equal(t, "SELECT * FROM orders WHERE id = $1 AND state = $2", query)
	assert.Equal(t, []any{42, "paid"}, args)
}

func TestRenderSQLTestQuery_RejectsMissingPositionalParameter(t *testing.T) {
	_, _, err := renderSQLTestQuery("SELECT * FROM orders WHERE id = $1 AND account_id = $3", map[string]any{"1": 42, "3": 7})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "contiguous")
}

func FuzzRenderSQLTestQueryDoesNotInlineInputValues(f *testing.F) {
	f.Add("42", "Robert'); DROP TABLE orders; --")
	f.Add("A-42", "Alice' OR 1=1 --")

	f.Fuzz(func(t *testing.T, id, name string) {
		id = "generated-id-" + safeSQLTestFuzzValue(id)
		name = "generated-name-" + safeSQLTestFuzzValue(name)
		query, args, err := renderSQLTestQuery(
			"SELECT * FROM orders WHERE id = $1 AND name = '{{input.name}}'",
			map[string]any{"1": id, "name": name},
		)

		require.NoError(t, err)
		assert.Equal(t, "SELECT * FROM orders WHERE id = $1 AND name = $2", query)
		assert.Equal(t, []any{id, name}, args)
		assert.NotContains(t, query, id)
		assert.NotContains(t, query, name)
	})
}

func safeSQLTestFuzzValue(value string) string {
	value = strings.ReplaceAll(value, "\x00", "")
	if len(value) > 96 {
		return value[:96]
	}
	return value
}
