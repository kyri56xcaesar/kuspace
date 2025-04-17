package userspace

import (
	"kyri56xcaesar/myThesis/internal/userspace/fslite"
	"kyri56xcaesar/myThesis/internal/userspace/minio"
	"log"
)

/*
 *
 *
 */
type StorageSystem interface {
	CreateVolume(volumeId any) error

	Insert(t any) error

	Select(sel, table, by, byvalue string, limit int) ([]any, error)
	SelectOne(sel, table, by, byvalue string) (any, error)
	Stat(t any) any

	Download(t any) error

	Remove(t any) error
	Update(t any) error
	Copy(t any) error
}

func StorageShipment(storageType string, srv *UService) StorageSystem {
	switch storageType {
	case "default", "local", "fslite":
		fslite := fslite.NewFsLite(srv.config)
		return &fslite
	case "minio", "remote":
		minio_cl := minio.NewMinioClient(srv.config)
		return &minio_cl
	default:
		log.Fatal("not a valid storage system, cannot operate")
		return nil
	}

}
