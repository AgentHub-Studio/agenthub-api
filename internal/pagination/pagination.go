package pagination

import (
	"net/http"
	"strconv"
)

// PageRequest holds the pagination and sorting parameters.
type PageRequest struct {
	Page int
	Size int
	Sort string
}

// Page is a generic paginated response.
type Page[T any] struct {
	Content          []T   `json:"content"`
	TotalElements    int64 `json:"totalElements"`
	TotalPages       int   `json:"totalPages"`
	Page             int   `json:"page"`
	Size             int   `json:"size"`
	NumberOfElements int   `json:"numberOfElements"`
	First            bool  `json:"first"`
	Last             bool  `json:"last"`
}

// NewPage constructs a Page from a slice of items and total count.
func NewPage[T any](content []T, total int64, req PageRequest) Page[T] {
	totalPages := 0
	if req.Size > 0 {
		totalPages = int((total + int64(req.Size) - 1) / int64(req.Size))
	}

	return Page[T]{
		Content:          content,
		TotalElements:    total,
		TotalPages:       totalPages,
		Page:             req.Page,
		Size:             req.Size,
		NumberOfElements: len(content),
		First:            req.Page == 0,
		Last:             req.Page >= totalPages-1,
	}
}

// ParsePageRequest extracts pagination parameters from an HTTP request's query string.
// Defaults: page=0, size=20.
func ParsePageRequest(r *http.Request) PageRequest {
	q := r.URL.Query()

	page := 0
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			page = n
		}
	}

	size := 20
	if v := q.Get("size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 200 {
			size = n
		}
	}

	sort := q.Get("sort")

	return PageRequest{Page: page, Size: size, Sort: sort}
}

// Offset returns the SQL offset for the given page request.
func (r PageRequest) Offset() int {
	return r.Page * r.Size
}
