// internal/store/errors_p03.go
package store

import "errors"

// ErrNoRow is returned by P0-3 repositories when a row does not exist (callers never see database/sql errors).
var ErrNoRow = errors.New("store: no such row")

// ErrDuplicate is returned when an insert would duplicate a unique key.
var ErrDuplicate = errors.New("store: duplicate")

// ErrConflict is returned when a reorder input is not exactly the current live set (stale client view).
var ErrConflict = errors.New("store: conflict")
