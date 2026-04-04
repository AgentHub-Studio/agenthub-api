package pagination

import (
	"net/http"
	"strconv"
	"strings"
)

// PageRequest holds pagination parameters parsed from query string.
// Compatible with Spring Pageable format: ?page=0&size=20&sort=name,asc
type PageRequest struct {
	Page    int    // 0-based page index
	Size    int    // page size (default 20, max 100)
	SortBy  string // field name
	SortDir string // "asc" or "desc"
}

// Page is a generic paginated response, compatible with Spring Page<T>.
type Page[T any] struct {
	Content          []T   `json:"content"`
	TotalElements    int64 `json:"totalElements"`
	TotalPages       int   `json:"totalPages"`
	Number           int   `json:"number"` // current page (0-based)
	Size             int   `json:"size"`
	NumberOfElements int   `json:"numberOfElements"` // elements in this page
	First            bool  `json:"first"`
	Last             bool  `json:"last"`
	Empty            bool  `json:"empty"`
}

const (
	defaultSize = 20
	maxSize     = 100
)

// FromRequest parses pagination parameters from an HTTP request's query string.
func FromRequest(r *http.Request) PageRequest {
	q := r.URL.Query()

	page, _ := strconv.Atoi(q.Get("page"))
	if page < 0 {
		page = 0
	}

	size, _ := strconv.Atoi(q.Get("size"))
	if size <= 0 {
		size = defaultSize
	}
	if size > maxSize {
		size = maxSize
	}

	sortBy := ""
	sortDir := "asc"
	if s := q.Get("sort"); s != "" {
		parts := strings.SplitN(s, ",", 2)
		sortBy = parts[0]
		if len(parts) == 2 && strings.ToLower(parts[1]) == "desc" {
			sortDir = "desc"
		}
	}

	return PageRequest{Page: page, Size: size, SortBy: sortBy, SortDir: sortDir}
}

// Offset returns the SQL OFFSET for this page request.
func (p PageRequest) Offset() int {
	return p.Page * p.Size
}

// OrderClause returns a safe ORDER BY clause string.
// allowedFields whitelist prevents SQL injection.
func (p PageRequest) OrderClause(allowedFields map[string]string) string {
	if p.SortBy == "" {
		return ""
	}
	col, ok := allowedFields[p.SortBy]
	if !ok {
		return ""
	}
	dir := "ASC"
	if p.SortDir == "desc" {
		dir = "DESC"
	}
	return col + " " + dir
}

// NewPage creates a Page[T] from content slice, request, and total count.
func NewPage[T any](content []T, req PageRequest, total int64) Page[T] {
	totalPages := 0
	if req.Size > 0 {
		totalPages = int((total + int64(req.Size) - 1) / int64(req.Size))
	}

	return Page[T]{
		Content:          content,
		TotalElements:    total,
		TotalPages:       totalPages,
		Number:           req.Page,
		Size:             req.Size,
		NumberOfElements: len(content),
		First:            req.Page == 0,
		Last:             req.Page >= totalPages-1,
		Empty:            len(content) == 0,
	}
}
