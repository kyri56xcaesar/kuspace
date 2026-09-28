package utils

import (
	"errors"
	"net/http"
)

/*
Kinds of failure the services agree on.

Code that fails wraps one of these (fmt.Errorf("volume %q: %w", name,
ErrNotFound), or a package sentinel that wraps it); code that handles a
failure asks errors.Is(err, ErrNotFound). Nobody decides what happened by
reading an error's text: messages change, and "already exists" matched
by strings.Contains breaks the day someone rewords it.

HTTPStatus maps a kind to the one status every handler answers with.
*/
var (
	ErrNotFound    = errors.New("not found")
	ErrExists      = errors.New("already exists")
	ErrNotEmpty    = errors.New("not empty")
	ErrInvalid     = errors.New("invalid")
	ErrForbidden   = errors.New("forbidden")
	ErrNoSpace     = errors.New("no space")    // a quota or capacity would be exceeded
	ErrUnavailable = errors.New("unavailable") // a dependency can't be reached
)

// HTTPStatus is the status that answers err: the kind it wraps, or 500.
func HTTPStatus(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrExists), errors.Is(err, ErrNotEmpty):
		return http.StatusConflict
	case errors.Is(err, ErrInvalid):
		return http.StatusBadRequest
	case errors.Is(err, ErrForbidden):
		return http.StatusForbidden
	case errors.Is(err, ErrNoSpace):
		return http.StatusInsufficientStorage
	case errors.Is(err, ErrUnavailable):
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}
