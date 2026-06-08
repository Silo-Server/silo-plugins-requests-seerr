package seerr

import "errors"

// ErrNotFound is returned by FindExistingRequest when no matching request is in
// the scanned page. It is a plugin-local sentinel (not an HTTP-layer error).
var ErrNotFound = errors.New("seerr: not found")
