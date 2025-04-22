package minio

/*
 *	A Minio Client api
 *
 *
 */

import (
	"fmt"
	"log"
	"strings"

	ut "kyri56xcaesar/myThesis/internal/utils"

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

// ✅
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
	err = mc.createBucket(cfg.MINIO_DEFAULT_BUCKET)
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

// ✅
func (mc *MinioClient) CreateVolume(volume any) error {
	v, ok := volume.(ut.Volume)
	if !ok {
		return ut.NewError("failed to cast to a volume")
	}

	log.Printf("volume incoming: %+v", v)

	err := mc.createBucket(v.Name)
	if err != nil {
		log.Printf("failed to create a bucket on minio: %v", err)
		return err
	}
	return nil
}

// ✅
func (mc *MinioClient) Insert(t any) error {
	object, ok := t.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}

	err := mc.putObject(
		object.Vname,
		object.Name,
		object.Reader,
		object.Size,
	)
	if err != nil {
		log.Printf("failed to iniate stream upload to minio: %v", err)
		return err
	}

	return nil
}

// ✅ .. maybe can enhance with more which factors
func (mc *MinioClient) SelectVolumes(which map[string]any) ([]any, error) {
	res, err := mc.listBuckets()
	if err != nil {
		log.Printf("failed to retrieve buckets: %v", err)
		return nil, err
	}

	prefix, _ := which["vid"].(string)

	var r []any
	for _, b := range res {
		volume := ut.Volume{
			Name:         b.Name,
			CreationDate: b.CreationDate.String(),
		}
		if prefix == "" {
			r = append(r, volume)
		} else if strings.Contains(b.Name, prefix) {
			r = append(r, volume)
		}

	}

	return r, nil
}

// ✅
func (mc *MinioClient) SelectObjects(which map[string]any) ([]any, error) {

	vN, is := which["vname"]
	if !is {
		return nil, fmt.Errorf("must specify volume")
	}
	vName, is := vN.(string)
	if !is {
		return nil, fmt.Errorf("bad volume identifier")
	}
	// fix vName
	prefix := which["prefix"].(string)
	p := strings.Split(strings.TrimPrefix(prefix, "/"), "/")
	if len(p) > 0 {
		prefix = p[len(p)-1]
	}

	objectCh, cancel, err := mc.listObjects(vName, prefix)
	if err != nil {
		log.Printf("failed to retrieve the lits of objects: %v", err)
		return nil, err
	}
	defer cancel()

	var objects []any
	for object := range objectCh {
		if object.Err != nil {
			fmt.Println(object.Err)
			return nil, object.Err
		}
		rsrc := ut.Resource{
			Name:       object.Key,
			Size:       object.Size,
			Type:       "object",
			Vname:      vName,
			Updated_at: object.LastModified.String(),
		}

		objects = append(objects, rsrc)
	}

	return objects, nil
}

func (mc *MinioClient) Stat(t any) any {
	object, ok := t.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}

	mc.statObject(object.Vname, object.Name)

	return nil
}

// ✅
func (mc *MinioClient) Remove(t any) error {
	resource, ok := t.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}

	err := mc.removeObject(resource.Vname, resource.Name)

	return err
}

// ✅
func (mc *MinioClient) RemoveVolume(t any) error {
	var bucketname string

	// check if the argument passed is either an entire volume
	// or just the identifier (name)
	// either case, get the name
	volume, ok := t.(ut.Volume)
	if !ok {
		bucketname, ok = t.(string)
		if !ok {
			return ut.NewError("failed to cast to a volume/id")
		}
	} else {
		bucketname = volume.Name
	}

	err := mc.removeBucket(bucketname)
	if err != nil {
		log.Printf("failed to remove bucket")
	}

	return err
}

// ✅
func (mc *MinioClient) Download(t *any) error {
	b := *t
	resource, ok := b.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}

	reader, err := mc.getObject(resource.Vname, resource.Name)
	if err != nil {

	}

	resource.Reader = reader

	return err
}

func (mc *MinioClient) Copy(s, d any) error {
	src, ok := s.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}
	dst, ok := d.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}

	uploadInfo, err := mc.copyObject(minio.CopySrcOptions{Bucket: src.Vname, Object: src.Name}, minio.CopyDestOptions{Bucket: dst.Vname, Object: dst.Name})
	if err != nil {
		return err
	}

	log.Printf("upload info: %+v", uploadInfo)
	return nil
}

/* find + copy + delete in this case*/
func (mc *MinioClient) Update(t any) error {
	return nil
}
