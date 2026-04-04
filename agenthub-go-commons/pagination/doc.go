// Package pagination provides the generic Page[T] type and PageRequest
// helper for AgentHub Go services, compatible with Spring's Pageable format.
//
// Usage:
//
//	req := pagination.FromRequest(r)   // parses ?page=0&size=20&sort=name,asc
//	page := pagination.NewPage(items, req, total)
package pagination
