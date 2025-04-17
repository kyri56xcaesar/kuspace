package minio

/*
 *	A Minio Client api
 *
 *
 */

import (
	"fmt"
	ut "kyri56xcaesar/myThesis/internal/utils"
	"log"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	region = "eu-central-1"
)

type MinioClient struct {
	accessKey     string
	secretKey     string
	endpoint      string
	useSSL        bool
	objectLocking bool
	// retentionPeriod int
	client *minio.Client
}

func NewMinioClient(cfg ut.EnvConfig) MinioClient {
	mc := MinioClient{
		accessKey:     cfg.MINIO_ACCESS_KEY,
		secretKey:     cfg.MINIO_SECRET_KEY,
		endpoint:      cfg.MINIO_ENDPOINT + ":" + cfg.MINIO_PORT,
		useSSL:        cfg.MINIO_USE_SSL == "true",
		objectLocking: cfg.MINIO_OBJECT_LOCKING,
	}

	client, err := minio.New(mc.endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(mc.accessKey, mc.secretKey, ""),
		Secure: mc.useSSL,
	})
	if err != nil {
		log.Fatal("failed to instantiate a new minio client")
	}
	mc.client = client

	// lets create a default bucket
	err = mc.CreateBucket(cfg.MINIO_DEFAULT_BUCKET)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			log.Printf("already exists... continuing")
		} else {
			log.Fatal("failed to create the default bucket: ", err)
		}
	}
	log.Printf("successfully created the default bucket")

	return mc
}

func (mc *MinioClient) CreateVolume(volume any) error {
	v := volume.(ut.Volume)

	log.Printf("volume incoming: %+v", v)

	err := mc.CreateBucket(v.Name)
	if err != nil {
		log.Printf("failed to create a bucket on minio: %v", err)
		return err
	}
	return nil
}

func (mc *MinioClient) Insert(t any) error {
	log.Printf("inserting an object in a bucket")

	object := t.(ut.Resource)

	err := mc.PutObject(
		object.Vname,
		object.Name,
		*object.Reader,
		object.Size,
	)
	if err != nil {
		log.Printf("failed to iniate stream upload to minio: %v", err)
		return err
	}

	return nil
}

func (mc *MinioClient) Select(sel, table, by, byvalue string, limit int) ([]any, error) {
	switch table {
	case "bucket", "volumes":
		res, err := mc.ListBuckets()
		if err != nil {
			log.Printf("failed to retrieve buckets: %v", err)
			return nil, err
		}
		var r []any

		for _, b := range res {
			var volume ut.Volume = ut.Volume{
				Name:         b.Name,
				CreationDate: b.CreationDate.String(),
			}
			r = append(r, volume)
		}
		log.Printf("res: %+v", res)
		log.Printf("r: %+v", r)
		return r, nil

	case "object":
		mc.ListObjects(table, byvalue)
		return nil, nil //fornow
	default:
		return nil, ut.NewError("bad option for 'table'")
	}
}

func (mc *MinioClient) SelectOne(sel, table, by, byvalue string) (any, error) {
	switch table {
	case "bucket", "volumes":
		res, err := mc.ListBuckets()
		if err != nil {
			log.Printf("failed to retrieve buckets: %v", err)
			return nil, err
		}
		var volume ut.Volume
		for _, b := range res {
			if b.Name == byvalue {
				volume.Name = b.Name
				volume.CreationDate = b.CreationDate.String()
				return volume, nil
			}
		}
		return nil, fmt.Errorf("bucket %q not found", byvalue)

	case "object":
		return nil, nil
	default:
		return nil, nil
	}
}

func (mc *MinioClient) Stat(t any) any {

	object := t.(ut.Resource)

	mc.StatObject(object.Vname, object.Name)

	return nil
}

func (mc *MinioClient) Remove(t any) error {
	return nil
}

/* copy + delete in this case*/
func (mc *MinioClient) Update(t any) error {
	return nil
}

func (mc *MinioClient) Download(t any) error {
	return nil
}

func (mc *MinioClient) Copy(t any) error {
	return nil
}
