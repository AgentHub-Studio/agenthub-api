// Package httputil provides standardized HTTP response helpers, error types,
// request binding, and validation for AgentHub Go services.
//
// Usage:
//
//	httputil.JSON(w, http.StatusOK, response)
//	httputil.Error(w, http.StatusNotFound, "agent not found")
//	httputil.BindJSON(r, &req); httputil.Validate(&req)
package httputil
