package uspace

import (
	"context"
	"log"

	"kyri56xcaesar/kuspace/internal/uspace/minio"
	"kyri56xcaesar/kuspace/pkg/fslite"
)

// StorageSystem interface describes what a struct aspiring to integrate
// and become a storage for this system should implement
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

// StorageShipment delivers the desired StorageSystem struct according to configuration
func StorageShipment(storageType string, srv *UService) StorageSystem {
	switch storageType {
	case "default", "local", "fslite":
		fslite := fslite.NewFsLite(srv.config)

		return &fslite
	case "minio", "remote":
		minioCl := minio.NewMinioClient(srv.config)

		return &minioCl
	default:
		log.Fatal("not a valid storage system, cannot operate")

		return nil
	}
}
