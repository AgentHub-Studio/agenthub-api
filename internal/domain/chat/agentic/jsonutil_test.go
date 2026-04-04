package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

// --- StripBOM ---

func TestStripBOM_NoBOM(t *testing.T) {
	assert.Equal(t, "hello", agentic.StripBOM("hello"))
}

func TestStripBOM_UTF8BOM(t *testing.T) {
	bom := "\xEF\xBB\xBF"
	assert.Equal(t, "hello", agentic.StripBOM(bom+"hello"))
}

func TestStripBOM_UnicodeBOM(t *testing.T) {
	assert.Equal(t, "hello", agentic.StripBOM("\uFEFFhello"))
}

func TestStripBOM_Empty(t *testing.T) {
	assert.Equal(t, "", agentic.StripBOM(""))
}

func TestStripBOM_OnlyBOM(t *testing.T) {
	assert.Equal(t, "", agentic.StripBOM("\xEF\xBB\xBF"))
}

// --- SafeParseJSON ---

func TestSafeParseJSON_ValidObject(t *testing.T) {
	result := agentic.SafeParseJSON(`{"name":"test"}`)
	require.NotNil(t, result)
	m, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "test", m["name"])
}

func TestSafeParseJSON_ValidArray(t *testing.T) {
	result := agentic.SafeParseJSON(`[1,2,3]`)
	require.NotNil(t, result)
	arr, ok := result.([]interface{})
	require.True(t, ok)
	assert.Len(t, arr, 3)
}

func TestSafeParseJSON_InvalidJSON(t *testing.T) {
	result := agentic.SafeParseJSON(`{invalid}`)
	assert.Nil(t, result)
}

func TestSafeParseJSON_Empty(t *testing.T) {
	result := agentic.SafeParseJSON("")
	assert.Nil(t, result)
}

func TestSafeParseJSON_Null(t *testing.T) {
	result := agentic.SafeParseJSON("null")
	assert.Nil(t, result)
}

func TestSafeParseJSON_Number(t *testing.T) {
	result := agentic.SafeParseJSON("42")
	require.NotNil(t, result)
	assert.Equal(t, float64(42), result)
}

func TestSafeParseJSON_WithBOM(t *testing.T) {
	result := agentic.SafeParseJSON("\xEF\xBB\xBF" + `{"ok":true}`)
	require.NotNil(t, result)
	m, ok := result.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, true, m["ok"])
}

// --- JSONParseCache ---

func TestJSONParseCache_CacheHit(t *testing.T) {
	cache := agentic.NewJSONParseCache(10)
	v1, ok1 := cache.Parse(`{"a":1}`)
	v2, ok2 := cache.Parse(`{"a":1}`)
	assert.True(t, ok1)
	assert.True(t, ok2)
	assert.Equal(t, v1, v2)
	assert.Equal(t, 1, cache.Size())
}

func TestJSONParseCache_InvalidCached(t *testing.T) {
	cache := agentic.NewJSONParseCache(10)
	_, ok1 := cache.Parse(`{bad}`)
	_, ok2 := cache.Parse(`{bad}`)
	assert.False(t, ok1)
	assert.False(t, ok2)
	assert.Equal(t, 1, cache.Size())
}

func TestJSONParseCache_Eviction(t *testing.T) {
	cache := agentic.NewJSONParseCache(2)
	cache.Parse(`"a"`)
	cache.Parse(`"b"`)
	cache.Parse(`"c"`) // evicts "a"
	assert.Equal(t, 2, cache.Size())
}

func TestJSONParseCache_LargeInputNotCached(t *testing.T) {
	cache := agentic.NewJSONParseCache(10)
	large := `"` + string(make([]byte, 9000)) + `"`
	// Will fail to parse (null bytes), but the point is size check
	cache.Parse(large)
	assert.Equal(t, 0, cache.Size())
}

// --- ParseJSONL ---

func TestParseJSONL_SingleLine(t *testing.T) {
	results := agentic.ParseJSONL(`{"a":1}`)
	require.Len(t, results, 1)
	m := results[0].(map[string]interface{})
	assert.Equal(t, float64(1), m["a"])
}

func TestParseJSONL_MultipleLines(t *testing.T) {
	data := "{\"a\":1}\n{\"b\":2}\n{\"c\":3}"
	results := agentic.ParseJSONL(data)
	assert.Len(t, results, 3)
}

func TestParseJSONL_BlankLines(t *testing.T) {
	data := "{\"a\":1}\n\n\n{\"b\":2}\n"
	results := agentic.ParseJSONL(data)
	assert.Len(t, results, 2)
}

func TestParseJSONL_MalformedSkipped(t *testing.T) {
	data := "{\"a\":1}\n{invalid}\n{\"c\":3}"
	results := agentic.ParseJSONL(data)
	assert.Len(t, results, 2)
}

func TestParseJSONL_Empty(t *testing.T) {
	results := agentic.ParseJSONL("")
	assert.Nil(t, results)
}

func TestParseJSONL_WithBOM(t *testing.T) {
	data := "\xEF\xBB\xBF{\"a\":1}\n{\"b\":2}"
	results := agentic.ParseJSONL(data)
	assert.Len(t, results, 2)
}

func TestParseJSONL_TrailingNewline(t *testing.T) {
	data := "{\"a\":1}\n{\"b\":2}\n"
	results := agentic.ParseJSONL(data)
	assert.Len(t, results, 2)
}

// --- ParseJSONLTyped ---

func TestParseJSONLTyped_Structs(t *testing.T) {
	type Item struct {
		Name string `json:"name"`
	}
	data := "{\"name\":\"alice\"}\n{\"name\":\"bob\"}"
	results := agentic.ParseJSONLTyped[Item](data)
	require.Len(t, results, 2)
	assert.Equal(t, "alice", results[0].Name)
	assert.Equal(t, "bob", results[1].Name)
}

func TestParseJSONLTyped_SkipsMalformed(t *testing.T) {
	type Item struct {
		Val int `json:"val"`
	}
	data := "{\"val\":1}\nnot-json\n{\"val\":3}"
	results := agentic.ParseJSONLTyped[Item](data)
	require.Len(t, results, 2)
	assert.Equal(t, 1, results[0].Val)
	assert.Equal(t, 3, results[1].Val)
}

func TestParseJSONLTyped_Empty(t *testing.T) {
	type Item struct{}
	results := agentic.ParseJSONLTyped[Item]("")
	assert.Nil(t, results)
}
