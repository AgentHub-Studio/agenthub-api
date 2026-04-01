package snapshot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractSchema_FlatObject(t *testing.T) {
	data := []byte(`{"id":"uuid","name":"test","active":true,"count":42}`)
	schema, err := ExtractSchema(data)
	assert.NoError(t, err)
	assert.Equal(t, TypeString, schema["id"])
	assert.Equal(t, TypeString, schema["name"])
	assert.Equal(t, TypeBoolean, schema["active"])
	assert.Equal(t, TypeNumber, schema["count"])
}

func TestExtractSchema_NestedObject(t *testing.T) {
	data := []byte(`{"user":{"id":"uuid","email":"test@example.com"},"status":"active"}`)
	schema, err := ExtractSchema(data)
	assert.NoError(t, err)
	assert.Equal(t, TypeObject, schema["user"])
	assert.Equal(t, TypeString, schema["user.id"])
	assert.Equal(t, TypeString, schema["user.email"])
	assert.Equal(t, TypeString, schema["status"])
}

func TestExtractSchema_Array(t *testing.T) {
	data := []byte(`{"items":[{"id":"1"},{"id":"2"}],"total":2}`)
	schema, err := ExtractSchema(data)
	assert.NoError(t, err)
	assert.Equal(t, TypeArray, schema["items"])
	assert.Equal(t, TypeNumber, schema["total"])
}

func TestExtractSchema_NullField(t *testing.T) {
	data := []byte(`{"id":"uuid","deletedAt":null}`)
	schema, err := ExtractSchema(data)
	assert.NoError(t, err)
	assert.Equal(t, TypeNull, schema["deletedAt"])
}

func TestTypeKindOf(t *testing.T) {
	assert.Equal(t, TypeString, typeKindOf("hello"))
	assert.Equal(t, TypeNumber, typeKindOf(float64(42)))
	assert.Equal(t, TypeBoolean, typeKindOf(true))
	assert.Equal(t, TypeArray, typeKindOf([]any{}))
	assert.Equal(t, TypeObject, typeKindOf(map[string]any{}))
	assert.Equal(t, TypeNull, typeKindOf(nil))
}
