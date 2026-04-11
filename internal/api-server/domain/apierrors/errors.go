package apierrors

import "errors"

// ErrNotFound indicates a missing domain resource (HTTP 404).
var ErrNotFound = errors.New("not found")
