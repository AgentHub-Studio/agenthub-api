package agentic_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AgentHub-Studio/agenthub-api/internal/domain/chat/agentic"
)

func TestCircularBuffer_NewPanicsOnZero(t *testing.T) {
	assert.Panics(t, func() { agentic.NewCircularBuffer[int](0) })
}

func TestCircularBuffer_NewPanicsOnNegative(t *testing.T) {
	assert.Panics(t, func() { agentic.NewCircularBuffer[int](-1) })
}

func TestCircularBuffer_EmptyState(t *testing.T) {
	buf := agentic.NewCircularBuffer[string](5)
	assert.Equal(t, 0, buf.Len())
	assert.Equal(t, 5, buf.Cap())
	assert.False(t, buf.IsFull())
	assert.Nil(t, buf.ToArray())
	assert.Nil(t, buf.GetRecent(3))
}

func TestCircularBuffer_AddAndLen(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](3)
	buf.Add(10)
	assert.Equal(t, 1, buf.Len())
	buf.Add(20)
	assert.Equal(t, 2, buf.Len())
	buf.Add(30)
	assert.Equal(t, 3, buf.Len())
	assert.True(t, buf.IsFull())
}

func TestCircularBuffer_ToArray_BeforeWrap(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](5)
	buf.Add(1)
	buf.Add(2)
	buf.Add(3)
	assert.Equal(t, []int{1, 2, 3}, buf.ToArray())
}

func TestCircularBuffer_ToArray_AfterWrap(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](3)
	buf.Add(1)
	buf.Add(2)
	buf.Add(3)
	buf.Add(4) // evicts 1
	buf.Add(5) // evicts 2
	assert.Equal(t, []int{3, 4, 5}, buf.ToArray())
}

func TestCircularBuffer_GetRecent_Partial(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](5)
	buf.Add(10)
	buf.Add(20)
	buf.Add(30)
	buf.Add(40)
	buf.Add(50)

	recent := buf.GetRecent(2)
	assert.Equal(t, []int{40, 50}, recent)
}

func TestCircularBuffer_GetRecent_MoreThanSize(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](10)
	buf.Add(1)
	buf.Add(2)
	recent := buf.GetRecent(100)
	assert.Equal(t, []int{1, 2}, recent)
}

func TestCircularBuffer_GetRecent_AfterWrap(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](3)
	buf.Add(1)
	buf.Add(2)
	buf.Add(3)
	buf.Add(4) // evicts 1
	recent := buf.GetRecent(2)
	assert.Equal(t, []int{3, 4}, recent)
}

func TestCircularBuffer_GetRecent_Zero(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](3)
	buf.Add(1)
	assert.Nil(t, buf.GetRecent(0))
}

func TestCircularBuffer_AddAll(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](5)
	buf.AddAll([]int{10, 20, 30, 40, 50})
	assert.Equal(t, 5, buf.Len())
	assert.Equal(t, []int{10, 20, 30, 40, 50}, buf.ToArray())
}

func TestCircularBuffer_AddAll_Overflow(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](3)
	buf.AddAll([]int{1, 2, 3, 4, 5})
	assert.Equal(t, 3, buf.Len())
	assert.Equal(t, []int{3, 4, 5}, buf.ToArray())
}

func TestCircularBuffer_Clear(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](3)
	buf.AddAll([]int{1, 2, 3})
	require.Equal(t, 3, buf.Len())

	buf.Clear()
	assert.Equal(t, 0, buf.Len())
	assert.False(t, buf.IsFull())
	assert.Nil(t, buf.ToArray())
}

func TestCircularBuffer_ClearThenReuse(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](3)
	buf.AddAll([]int{1, 2, 3})
	buf.Clear()
	buf.Add(99)
	assert.Equal(t, []int{99}, buf.ToArray())
}

func TestCircularBuffer_Peek_Empty(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](3)
	_, ok := buf.Peek()
	assert.False(t, ok)
}

func TestCircularBuffer_Peek_ReturnsOldest(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](3)
	buf.AddAll([]int{10, 20, 30})
	val, ok := buf.Peek()
	assert.True(t, ok)
	assert.Equal(t, 10, val)

	buf.Add(40) // evicts 10
	val, ok = buf.Peek()
	assert.True(t, ok)
	assert.Equal(t, 20, val)
}

func TestCircularBuffer_PeekNewest_Empty(t *testing.T) {
	buf := agentic.NewCircularBuffer[string](3)
	_, ok := buf.PeekNewest()
	assert.False(t, ok)
}

func TestCircularBuffer_PeekNewest_ReturnsLatest(t *testing.T) {
	buf := agentic.NewCircularBuffer[string](3)
	buf.Add("a")
	buf.Add("b")
	val, ok := buf.PeekNewest()
	assert.True(t, ok)
	assert.Equal(t, "b", val)

	buf.Add("c")
	buf.Add("d") // wraps, evicts "a"
	val, ok = buf.PeekNewest()
	assert.True(t, ok)
	assert.Equal(t, "d", val)
}

func TestCircularBuffer_StringType(t *testing.T) {
	buf := agentic.NewCircularBuffer[string](2)
	buf.Add("hello")
	buf.Add("world")
	buf.Add("foo") // evicts "hello"
	assert.Equal(t, []string{"world", "foo"}, buf.ToArray())
}

func TestCircularBuffer_StructType(t *testing.T) {
	type entry struct {
		Name string
		Val  int
	}
	buf := agentic.NewCircularBuffer[entry](2)
	buf.Add(entry{"a", 1})
	buf.Add(entry{"b", 2})
	buf.Add(entry{"c", 3})

	arr := buf.ToArray()
	assert.Len(t, arr, 2)
	assert.Equal(t, "b", arr[0].Name)
	assert.Equal(t, "c", arr[1].Name)
}

func TestCircularBuffer_SingleCapacity(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](1)
	buf.Add(1)
	assert.Equal(t, []int{1}, buf.ToArray())
	buf.Add(2)
	assert.Equal(t, []int{2}, buf.ToArray())
	assert.Equal(t, 1, buf.Len())
	assert.True(t, buf.IsFull())
}

func TestCircularBuffer_LargeCapacity(t *testing.T) {
	buf := agentic.NewCircularBuffer[int](1000)
	for i := 0; i < 1500; i++ {
		buf.Add(i)
	}
	assert.Equal(t, 1000, buf.Len())
	arr := buf.ToArray()
	assert.Equal(t, 500, arr[0])   // oldest
	assert.Equal(t, 1499, arr[999]) // newest
}
