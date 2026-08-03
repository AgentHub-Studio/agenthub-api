package agentic

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertLLMToolsToAI_PreservesDocumentSearchMetadataFilterGrammar(t *testing.T) {
	converted := convertLLMToolsToAI([]LLMTool{documentSearchTool(nil)})
	require.Len(t, converted, 1)

	parameters := converted[0].Function.Parameters
	properties, ok := parameters["properties"].(map[string]any)
	require.True(t, ok)
	metadataFilter, ok := properties["metadataFilter"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "#/$defs/metadataFilter", metadataFilter["$ref"])

	defs, ok := parameters["$defs"].(map[string]any)
	require.True(t, ok)
	filter, ok := defs["metadataFilter"].(map[string]any)
	require.True(t, ok)
	forms, ok := filter["oneOf"].([]any)
	require.True(t, ok)
	assert.Len(t, forms, 4)
}
