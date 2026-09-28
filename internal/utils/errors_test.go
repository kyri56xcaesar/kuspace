package utils

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestHTTPStatusFollowsTheWrappedKind(t *testing.T) {
	specific := fmt.Errorf("volume %w", ErrExists) // like fslite.ErrVolumeExists
	cases := map[error]int{
		nil: http.StatusOK,
		fmt.Errorf("get %q: %w", "a", ErrNotFound):  http.StatusNotFound,
		fmt.Errorf("%w: team", specific):            http.StatusConflict,
		fmt.Errorf("delete: %w", ErrNotEmpty):       http.StatusConflict,
		fmt.Errorf("%w: storage quota", ErrNoSpace): http.StatusInsufficientStorage,
		fmt.Errorf("minio: %w", ErrUnavailable):     http.StatusServiceUnavailable,
		ErrForbidden:                                http.StatusForbidden,
		ErrInvalid:                                  http.StatusBadRequest,
		errors.New("disk on fire"):                  http.StatusInternalServerError,
	}
	for err, want := range cases {
		if got := HTTPStatus(err); got != want {
			t.Errorf("HTTPStatus(%v) = %d, want %d", err, got, want)
		}
	}
	if !errors.Is(fmt.Errorf("%w: team", specific), ErrExists) {
		t.Error("a package sentinel wrapping a kind must match the kind")
	}
}
