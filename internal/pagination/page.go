package pagination

import (
	"math"
	"net/http"
	"strconv"
)

// Page represents a paginated result set.
type Page[T any] struct {
	Content       []T   `json:"content"`
	TotalElements int64 `json:"totalElements"`
	TotalPages    int   `json:"totalPages"`
	Page          int   `json:"page"`
	Size          int   `json:"size"`
	First         bool  `json:"first"`
	Last          bool  `json:"last"`
	Empty         bool  `json:"empty"`
}

// PageRequest holds pagination parameters extracted from an HTTP request.
type PageRequest struct {
	Page int
	Size int
}

// Offset returns the SQL OFFSET value for this page.
func (p PageRequest) Offset() int {
	return p.Page * p.Size
}

// ParsePageRequest extracts pagination parameters from an HTTP request.
func ParsePageRequest(r *http.Request) PageRequest {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size <= 0 || size > 100 {
		size = 20
	}
	if page < 0 {
		page = 0
	}
	return PageRequest{Page: page, Size: size}
}

// MapPage converts a Page[A] to a Page[B] by applying a mapping function.
func MapPage[A, B any](p Page[A], fn func(A) B) Page[B] {
	mapped := make([]B, len(p.Content))
	for i, v := range p.Content {
		mapped[i] = fn(v)
	}
	p2 := p
	return Page[B]{
		Content:       mapped,
		TotalElements: p2.TotalElements,
		TotalPages:    p2.TotalPages,
		Page:          p2.Page,
		Size:          p2.Size,
		First:         p2.First,
		Last:          p2.Last,
		Empty:         p2.Empty,
	}
}

// NewPage constructs a Page[T] from content, total count and pagination params.
func NewPage[T any](content []T, total int64, req PageRequest) Page[T] {
	totalPages := int(math.Ceil(float64(total) / float64(req.Size)))
	if totalPages == 0 {
		totalPages = 1
	}
	return Page[T]{
		Content:       content,
		TotalElements: total,
		TotalPages:    totalPages,
		Page:          req.Page,
		Size:          req.Size,
		First:         req.Page == 0,
		Last:          req.Page >= totalPages-1,
		Empty:         len(content) == 0,
	}
}
