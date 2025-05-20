package uspace

import (
	"context"
	"kyri56xcaesar/kuspace/internal/uspace/minio"
	"kyri56xcaesar/kuspace/pkg/fslite"
	"log"
)

/*
 *
 *
 */
type StorageSystem interface {
	DefaultVolume(local bool) string

	CreateVolume(volume any) error

	SelectVolumes(how map[string]any) (any, error)
	SelectObjects(how map[string]any) (any, error)

	Insert(t any) (context.CancelFunc, error)
	Download(t *any) (context.CancelFunc, error)

	Stat(t any) (any, error)

	Remove(t any) error
	RemoveVolume(t any) error

	Update(t map[string]string) error
	Copy(s, d any) error

	Share(method string, t any) (any, error)
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
