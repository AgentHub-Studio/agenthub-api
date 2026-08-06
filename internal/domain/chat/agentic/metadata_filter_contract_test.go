package agentic

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDocumentSearchMetadataFilterContract_EqAcceptsDocumentSupportedValues(t *testing.T) {
	var schema map[string]any
	require.NoError(t, json.Unmarshal(documentSearchTool(nil).InputSchema, &schema))

	defs, ok := schema["$defs"].(map[string]any)
	require.True(t, ok)
	predicate, ok := defs["metadataPredicate"].(map[string]any)
	require.True(t, ok)
	forms, ok := predicate["oneOf"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, forms)

	eq, ok := forms[0].(map[string]any)
	require.True(t, ok)
	properties, ok := eq["properties"].(map[string]any)
	require.True(t, ok)
	value, ok := properties["value"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "#/$defs/metadataEqValue", value["$ref"])

	eqValue, ok := defs["metadataEqValue"].(map[string]any)
	require.True(t, ok)
	allowedValues, ok := eqValue["oneOf"].([]any)
	require.True(t, ok)
	require.Len(t, allowedValues, 3)
	assert.Equal(t, "#/$defs/metadataScalar", allowedValues[0].(map[string]any)["$ref"])
	assert.Equal(t, "#/$defs/metadataStringArray", allowedValues[1].(map[string]any)["$ref"])
	assert.Equal(t, "#/$defs/metadataSecondLevelObject", allowedValues[2].(map[string]any)["$ref"])
}
