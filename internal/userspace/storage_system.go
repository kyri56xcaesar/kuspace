package userspace

import (
	"fmt"

	"kyri56xcaesar/myThesis/internal/userspace/fslite"
	"kyri56xcaesar/myThesis/internal/userspace/minio"
)

/*
 *
 *
 */
type StorageSystem interface {
	Insert(t []any) error
	Select(sel, table, by, byvalue string, limit int) ([]any, error)
	SelectOne(sel, table, by, byvalue string) (any, error)
	Update(t any) error
	Remove(t any) error
}

func StorageShipment(storageType string, srv *UService) (StorageSystem, error) {
	switch storageType {
	case "default", "local", "fslite":
		fslite := fslite.NewFsLite(srv.config)
		return &fslite, nil
	case "minio":
		return minio.GetOrCreateMinioClient(srv.config)

	default:
		return nil, fmt.Errorf("unknown storage type: %s", storageType)
	}

}
