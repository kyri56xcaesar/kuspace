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

	CreateVolume(ctx context.Context, volume any) error

	SelectVolumes(ctx context.Context, how map[string]any) (any, error)
	SelectObjects(ctx context.Context, how map[string]any) (any, error)

	Insert(ctx context.Context, t any) error
	// Download fills the resource t points at with a Reader; the returned
	// func releases it (call it once the reader is done).
	Download(ctx context.Context, t *any) (context.CancelFunc, error)

	Stat(ctx context.Context, t any) (any, error)

	Remove(ctx context.Context, t any) error
	RemoveVolume(ctx context.Context, t any) error

	Update(ctx context.Context, t map[string]string) error
	Copy(ctx context.Context, s, d any) error

	Share(ctx context.Context, method string, t any) (any, error)
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
