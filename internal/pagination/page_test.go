package pagination_test

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/AgentHub-Studio/agenthub-api/internal/pagination"
)

func TestNewPage(t *testing.T) {
	content := []int{1, 2, 3}
	page := pagination.NewPage(content, 30, pagination.PageRequest{Page: 1, Size: 10})
	assert.Equal(t, content, page.Content)
	assert.Equal(t, int64(30), page.TotalElements)
	assert.Equal(t, 3, page.TotalPages)
	assert.Equal(t, 1, page.Page)
	assert.Equal(t, 10, page.Size)
}

func TestParsePageRequest_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	req := pagination.ParsePageRequest(r)
	assert.Equal(t, 0, req.Page)
	assert.Equal(t, 20, req.Size)
}

func TestParsePageRequest_CustomValues(t *testing.T) {
	r := httptest.NewRequest("GET", "/?page=2&size=10", nil)
	req := pagination.ParsePageRequest(r)
	assert.Equal(t, 2, req.Page)
	assert.Equal(t, 10, req.Size)
}

func TestParsePageRequest_SizeCap(t *testing.T) {
	r := httptest.NewRequest("GET", "/?size=500", nil)
	req := pagination.ParsePageRequest(r)
	assert.Equal(t, 20, req.Size)
}

func TestParsePageRequest_NegativePage(t *testing.T) {
	r := httptest.NewRequest("GET", "/?page=-1", nil)
	req := pagination.ParsePageRequest(r)
	assert.Equal(t, 0, req.Page)
}

func TestPageRequest_Offset(t *testing.T) {
	req := pagination.PageRequest{Page: 3, Size: 10}
	assert.Equal(t, 30, req.Offset())
}
