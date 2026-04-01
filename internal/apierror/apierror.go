// Package apierror defines sentinel errors for domain operations.
package apierror

import "errors"

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")
