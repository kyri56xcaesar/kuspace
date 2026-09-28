package minio

import (
	"errors"
	"net"
	"testing"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/minio/minio-go/v7"
)

func TestMapErr(t *testing.T) {
	cases := map[string]error{
		"NoSuchKey":               ut.ErrNotFound,
		"NoSuchBucket":            ut.ErrNotFound,
		"BucketAlreadyOwnedByYou": ut.ErrExists,
		"BucketNotEmpty":          ut.ErrNotEmpty,
		"XMinioStorageFull":       ut.ErrNoSpace,
		"AccessDenied":            ut.ErrUnavailable,
	}
	for code, kind := range cases {
		err := mapErr(minio.ErrorResponse{Code: code, Message: "m"})
		if !errors.Is(err, kind) {
			t.Errorf("%s -> %v, want %v", code, err, kind)
		}
	}
	if err := mapErr(&net.OpError{Op: "dial", Err: errors.New("connection refused")}); !errors.Is(err, ut.ErrUnavailable) {
		t.Errorf("network error -> %v, want unavailable", err)
	}
	other := errors.New("something else")
	if !errors.Is(mapErr(other), other) || mapErr(other).Error() != other.Error() || mapErr(nil) != nil {
		t.Error("unknown errors must pass through unchanged")
	}
}
