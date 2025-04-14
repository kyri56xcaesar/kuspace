package userspace

import (
	"fmt"

	"kyri56xcaesar/myThesis/internal/userspace/fslite"
)

/*
 *
 *
 */
type StorageSystem interface {
	Insert(t any) error
	Select(t any) ([]any, error)
	SelectOne(t any) (any, error)
	Update(t any) error
	Remove(t any) error
}

func StorageFactory(storageType string, srv *UService) (StorageSystem, error) {
	switch storageType {
	case "default", "local", "fslite":
		fslite := fslite.NewFsLite(srv.config)
		return &fslite, nil
	case "minio":
		return nil, fmt.Errorf("minio storage type not implemeneted yet")
	default:
		return nil, fmt.Errorf("unknown storage type: %s", storageType)
	}

}
