package minio

import (
	"errors"
	"fmt"
	"net"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/minio/minio-go/v7"
)

// mapErr translates MinIO's answers into the shared kinds (ut.ErrNotFound,
// ut.ErrExists, ...), keeping the original error in the chain. Callers used
// to recognise "not empty" or "already exists" by the message text.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	var kind error
	switch minio.ToErrorResponse(err).Code {
	case "NoSuchKey", "NoSuchBucket", "NoSuchUpload":
		kind = ut.ErrNotFound
	case "BucketAlreadyOwnedByYou", "BucketAlreadyExists":
		kind = ut.ErrExists
	case "BucketNotEmpty":
		kind = ut.ErrNotEmpty
	case "XMinioStorageFull":
		kind = ut.ErrNoSpace
	case "AccessDenied", "InvalidAccessKeyId", "SignatureDoesNotMatch":
		kind = ut.ErrUnavailable // our credentials, not the caller's fault
	}
	var ne net.Error
	if kind == nil && errors.As(err, &ne) {
		kind = ut.ErrUnavailable
	}
	if kind == nil {
		return err
	}

	return fmt.Errorf("%w: %w", kind, err)
}
