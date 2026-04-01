// Package pagination provides helpers for paginated API responses.
package pagination

import (
	"net/http"
	"strconv"
)

const (
	defaultPage = 0
	defaultSize = 20
	maxSize     = 100
)

// PageRequest holds pagination parameters.
type PageRequest struct {
	Page int
	Size int
}

// Offset returns the SQL offset for the given page and size.
func (pr PageRequest) Offset() int {
	return pr.Page * pr.Size
}

// ParsePageRequest extracts pagination parameters from a request's query string.
func ParsePageRequest(r *http.Request) PageRequest {
	page := parseInt(r.URL.Query().Get("page"), defaultPage)
	size := parseInt(r.URL.Query().Get("size"), defaultSize)

	if page < 0 {
		page = 0
	}
	if size <= 0 {
		size = defaultSize
	}
	if size > maxSize {
		size = maxSize
	}

	return PageRequest{Page: page, Size: size}
}

// Page is a generic paginated result.
type Page[T any] struct {
	Content          []T  `json:"content"`
	Page             int  `json:"page"`
	Size             int  `json:"size"`
	TotalElements    int  `json:"totalElements"`
	TotalPages       int  `json:"totalPages"`
	First            bool `json:"first"`
	Last             bool `json:"last"`
}

// NewPage constructs a Page from a content slice, PageRequest, and total count.
func NewPage[T any](content []T, pr PageRequest, total int) Page[T] {
	totalPages := 0
	if pr.Size > 0 {
		totalPages = (total + pr.Size - 1) / pr.Size
	}
	return Page[T]{
		Content:       content,
		Page:          pr.Page,
		Size:          pr.Size,
		TotalElements: total,
		TotalPages:    totalPages,
		First:         pr.Page == 0,
		Last:          pr.Page >= totalPages-1,
	}
}

func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
