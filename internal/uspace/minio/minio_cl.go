package minio

/*
 *	A Minio Client api
 *
 *
 */

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	ut "kyri56xcaesar/kuspace/internal/utils"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	region   = "eu-central-1"
	localDir = "minio_local"
)

var (
	default_sign_duration              = time.Duration(time.Hour * 24 * 2)
	OBJECT_SIZE_THRESHOLD        int64 = 400_000_000
	DEFALT_OBJECT_SIZE_THRESHOLD int64 = 400_000_000

	ONLY_PRESIGNED_UPLOAD bool = false
	FETCHSTAT             bool = false
)

type MinioClient struct {
	accessKey     string
	secretKey     string
	endpoint      string
	useSSL        bool
	objectLocking bool
	// retentionPeriod int
	client *minio.Client

	default_local_space_path string
	default_bucket_name      string
}

// ✅
func NewMinioClient(cfg ut.EnvConfig) MinioClient {
	var err error
	ONLY_PRESIGNED_UPLOAD = cfg.ONLY_PRESIGNED_UPLOAD
	FETCHSTAT = cfg.MINIO_FETCH_STAT
	OBJECT_SIZE_THRESHOLD, err = strconv.ParseInt(cfg.OBJECT_SIZE_THRESHOLD, 10, 64)
	if err != nil {
		log.Printf("failed to parse object threshold, fallingthrough to default")
		OBJECT_SIZE_THRESHOLD = DEFALT_OBJECT_SIZE_THRESHOLD
	}

	mc := MinioClient{
		accessKey:                cfg.MINIO_ACCESS_KEY,
		secretKey:                cfg.MINIO_SECRET_KEY,
		endpoint:                 cfg.MINIO_ENDPOINT,
		useSSL:                   cfg.MINIO_USE_SSL == "true",
		objectLocking:            cfg.MINIO_OBJECT_LOCKING,
		default_bucket_name:      cfg.MINIO_DEFAULT_BUCKET,
		default_local_space_path: cfg.LOCAL_VOLUMES_DEFAULT_PATH + "/" + localDir + "/",
	}

	client, err := minio.New(mc.endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(mc.accessKey, mc.secretKey, ""),
		Secure: mc.useSSL,
	})
	if err != nil {
		log.Fatal("failed to instantiate a new minio client: ", err)
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
	log.Printf("default bucket ready: %s", cfg.MINIO_DEFAULT_BUCKET)

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
func (mc *MinioClient) Insert(t any) (context.CancelFunc, error) {
	object, ok := t.(ut.Resource)
	if !ok {
		return nil, ut.NewError("failed to cast")
	}

	log.Printf("about to insert object: %+v", object)

	// maybe use a presigned link for upload for a certain size threshold?
	if object.Size > OBJECT_SIZE_THRESHOLD || ONLY_PRESIGNED_UPLOAD {
		signedurl, err := mc.Share("put", t)
		if err != nil {
			log.Printf("failed to get a presigned link: %v", err)
			return nil, err
		}
		url, ok := signedurl.(url.URL)
		if !ok {
			log.Printf("failed to cast to URL")
			return nil, fmt.Errorf("bad presigned url")
		}

		resp, err := http.DefaultClient.Post(http.MethodPut, url.String(), object.Reader)
		if err != nil {
			log.Printf("failed to perform request: %v", err)
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 300 {
			log.Printf("bad response")
			return nil, fmt.Errorf("failed to upload to minio via link")
		}
		return nil, nil

	} else {
		cancel, err := mc.putObject(
			object.Vname,
			object.Name,
			object.Reader,
			object.Size,
		)
		if err != nil {
			log.Printf("failed to iniate stream upload to minio: %v", err)
		}
		return cancel, err
	}

}

// ✅ .. maybe can enhance with more which factors
func (mc *MinioClient) SelectVolumes(which map[string]any) (any, error) {
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
func (mc *MinioClient) SelectObjects(which map[string]any) (any, error) {

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

// ✅
func (mc *MinioClient) Stat(t any) (any, error) {
	object, ok := t.(ut.Resource)
	if !ok {
		return nil, ut.NewError("failed to cast")
	}

	// check if the object exists

	fi, err := os.Stat(mc.default_local_space_path + object.Name)
	if err == nil {
		log.Printf("object exists")
		return fi, nil
	} else if os.IsNotExist(err) {
	} else {
		return nil, fmt.Errorf("error checking file existence: %v", err)
	}

	if FETCHSTAT {
		cancel, err := mc.fGetObject(object.Vname, object.Name, mc.default_local_space_path+object.Name)
		if err != nil {
			log.Printf("failed to get object from minio: %v", err)
			return nil, err
		}
		defer cancel()
		log.Printf("successfully downloaded the object: %s", object.Name)

		info, err := os.Stat(mc.default_local_space_path + object.Name)
		if err != nil {
			log.Printf("failed to stat the object: %v", err)
			return nil, err
		}
		return info, nil

	}

	info, err := mc.statObject(object.Vname, object.Name)
	if err != nil {
		log.Printf("faile to stat remote object: %v", err)
	}

	return info, err
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
func (mc *MinioClient) Download(t *any) (context.CancelFunc, error) {
	value := *t
	resourcePtr, ok := value.(*ut.Resource)
	if !ok {
		return nil, ut.NewError("failed to cast to *Resource")
	}

	reader, cancelFn, err := mc.getObject(resourcePtr.Vname, resourcePtr.Name)
	if err != nil && reader == nil {
		log.Printf("failed to get object from minio: %v", err)
		return nil, err
	}

	resourcePtr.Reader = reader

	return cancelFn, err
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

// ✅
func (mc *MinioClient) DefaultVolume(local bool) string {
	if local {
		return mc.default_local_space_path
	} else {
		return mc.default_bucket_name
	}
}

// ✅
func (mc *MinioClient) Share(method string, t any) (any, error) {
	resource, ok := t.(ut.Resource)
	if !ok {
		log.Printf("failed to cast to resource")
		return nil, ut.NewError("bad object, failed to cast to resource")
	}

	switch method {
	case "get":
		url, err := mc.getPresignedObject(resource.Vname, resource.Name, default_sign_duration)
		if err != nil {
			log.Printf("failed to retrieve object sign")
		}
		return url, err

	case "put":
		url, err := mc.putPresignedObject(resource.Vname, resource.Name, default_sign_duration)
		if err != nil {
			log.Printf("failed to retrieve object sign")
		}
		return url, err

	default:
		log.Printf("invalid method")
		return nil, ut.NewError("bad method")
	}
}
