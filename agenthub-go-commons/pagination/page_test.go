package pagination_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-go-commons/pagination"
)

func TestFromRequest(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		req := pagination.FromRequest(r)
		assert.Equal(t, 0, req.Page)
		assert.Equal(t, 20, req.Size)
		assert.Equal(t, "", req.SortBy)
		assert.Equal(t, "asc", req.SortDir)
	})

	t.Run("custom params", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/?page=2&size=50&sort=name,desc", nil)
		req := pagination.FromRequest(r)
		assert.Equal(t, 2, req.Page)
		assert.Equal(t, 50, req.Size)
		assert.Equal(t, "name", req.SortBy)
		assert.Equal(t, "desc", req.SortDir)
	})

	t.Run("max size capped at 100", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/?size=500", nil)
		req := pagination.FromRequest(r)
		assert.Equal(t, 100, req.Size)
	})

	t.Run("negative page defaults to 0", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/?page=-5", nil)
		req := pagination.FromRequest(r)
		assert.Equal(t, 0, req.Page)
	})

	t.Run("sort asc explicit", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/?sort=createdAt,asc", nil)
		req := pagination.FromRequest(r)
		assert.Equal(t, "createdAt", req.SortBy)
		assert.Equal(t, "asc", req.SortDir)
	})

	t.Run("sort field only without direction", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/?sort=name", nil)
		req := pagination.FromRequest(r)
		assert.Equal(t, "name", req.SortBy)
		assert.Equal(t, "asc", req.SortDir)
	})
}

func TestPageRequestOffset(t *testing.T) {
	req := pagination.PageRequest{Page: 3, Size: 20}
	assert.Equal(t, 60, req.Offset())
}

func TestPageRequestOrderClause(t *testing.T) {
	allowed := map[string]string{
		"name":      "name",
		"createdAt": "created_at",
	}

	t.Run("valid field asc", func(t *testing.T) {
		req := pagination.PageRequest{SortBy: "name", SortDir: "asc"}
		assert.Equal(t, "name ASC", req.OrderClause(allowed))
	})

	t.Run("valid field desc", func(t *testing.T) {
		req := pagination.PageRequest{SortBy: "createdAt", SortDir: "desc"}
		assert.Equal(t, "created_at DESC", req.OrderClause(allowed))
	})

	t.Run("invalid field returns empty", func(t *testing.T) {
		req := pagination.PageRequest{SortBy: "injected; DROP TABLE", SortDir: "asc"}
		assert.Equal(t, "", req.OrderClause(allowed))
	})

	t.Run("empty sort by returns empty", func(t *testing.T) {
		req := pagination.PageRequest{SortBy: "", SortDir: "asc"}
		assert.Equal(t, "", req.OrderClause(allowed))
	})
}

func TestNewPage(t *testing.T) {
	t.Run("single page result", func(t *testing.T) {
		items := []string{"a", "b", "c"}
		req := pagination.PageRequest{Page: 0, Size: 20}
		page := pagination.NewPage(items, req, 3)

		assert.Equal(t, 3, len(page.Content))
		assert.Equal(t, int64(3), page.TotalElements)
		assert.Equal(t, 1, page.TotalPages)
		assert.Equal(t, 0, page.Number)
		assert.Equal(t, 20, page.Size)
		assert.Equal(t, 3, page.NumberOfElements)
		assert.True(t, page.First)
		assert.True(t, page.Last)
		assert.False(t, page.Empty)
	})

	t.Run("empty result", func(t *testing.T) {
		req := pagination.PageRequest{Page: 0, Size: 20}
		page := pagination.NewPage([]string{}, req, 0)

		assert.True(t, page.Empty)
		assert.True(t, page.First)
		assert.Equal(t, int64(0), page.TotalElements)
		assert.Equal(t, 0, page.TotalPages)
	})

	t.Run("middle page of many", func(t *testing.T) {
		items := []string{"f", "g", "h"}
		req := pagination.PageRequest{Page: 1, Size: 3}
		page := pagination.NewPage(items, req, 9)

		assert.Equal(t, 3, page.TotalPages)
		assert.Equal(t, 1, page.Number)
		assert.False(t, page.First)
		assert.False(t, page.Last)
	})

	t.Run("last page", func(t *testing.T) {
		items := []string{"x"}
		req := pagination.PageRequest{Page: 2, Size: 3}
		page := pagination.NewPage(items, req, 7)

		assert.Equal(t, 3, page.TotalPages)
		assert.False(t, page.First)
		assert.True(t, page.Last)
	})
}
