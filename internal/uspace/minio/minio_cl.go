// Package minio defines a minio client
package minio

/*
 *	A Minio Client api
 *
 *
 */

import (
	"context"
	"errors"
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
	localDir = "minio_local" // perhaps some form of locality
)

var (
	defaultSignDuration              = time.Hour * 24 * 2
	objectSizeThreshold        int64 = 400_000_000
	defaultObjectSizeThreshold int64 = 400_000_000

	onlyPresignedUpload = false
	fetchstat           = false
)

// Client encapsules information needed for having a client for Minio
type Client struct {
	accessKey     string
	secretKey     string
	endpoint      string
	useSSL        bool
	objectLocking bool
	// retentionPeriod int
	client *minio.Client

	defaultLocalSpacePath string
	defaultBucketName     string
}

// NewMinioClient creates and returns a new Client instance using the provided configuration.
// ✅
func NewMinioClient(cfg ut.EnvConfig) Client {
	var err error
	onlyPresignedUpload = cfg.PresignedUploadOnly
	fetchstat = cfg.MinioFetchStat
	objectSizeThreshold, err = strconv.ParseInt(cfg.ObjectSizeThreshold, 10, 64)
	if err != nil {
		log.Printf("failed to parse object threshold, fallingthrough to default")
		objectSizeThreshold = defaultObjectSizeThreshold
	}
	var endpoint string
	if cfg.Profile == "baremetal" {
		endpoint = strings.TrimPrefix(cfg.MinioNodeportEndpoint, "http://")
	} else {
		endpoint = strings.TrimPrefix(cfg.MinioEndpoint, "http://")
	}

	mc := Client{
		accessKey:             cfg.MinioAccessKey,
		secretKey:             cfg.MinioSecretKey,
		endpoint:              endpoint,
		useSSL:                cfg.MinioUseSSL == "true",
		objectLocking:         cfg.MinioObjectLocking,
		defaultBucketName:     cfg.MinioDefaultBucket,
		defaultLocalSpacePath: cfg.LocalVolumesDefaultPath + "/" + localDir + "/",
	}

	client, err := minio.New(mc.endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(mc.accessKey, mc.secretKey, ""),
		Secure: mc.useSSL,
	})
	if err != nil {
		log.Fatal("failed to instantiate a new minio client: ", err)
	}
	mc.client = client

	return mc
}

// CreateVolume creates a new bucket (volume) in Minio.
// ✅
func (mc *Client) CreateVolume(volume any) error {
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

// Insert uploads an object to Minio, using presigned upload if necessary.
// ✅
func (mc *Client) Insert(t any) (context.CancelFunc, error) {
	object, ok := t.(ut.Resource)
	if !ok {
		return nil, ut.NewError("failed to cast")
	}
	// maybe use a presigned link for upload for a certain size threshold?
	if object.Size > objectSizeThreshold || onlyPresignedUpload {
		r, err := mc.Share("put", t)
		if err != nil {
			log.Printf("failed to get a presigned link: %v", err)

			return nil, err
		}

		url, ok := r.(*url.URL)
		if !ok {
			log.Printf("failed to cast to url pointer")

			return nil, errors.New("failed to cast to url pointer")
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url.String(), object.Reader)
		if err != nil {
			log.Printf("failed to create a new request: %v", err)

			return nil, err
		}
		req.ContentLength = object.Size

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Printf("failed to perform request: %v", err)

			return nil, err
		}
		defer func() {
			err := resp.Body.Close()
			if err != nil {
				log.Printf("failed to close resp body: %v", err)
			}
		}()

		// respBody, err := io.ReadAll(resp.Body)
		// if err != nil {
		// 	log.Printf("failed to read response body: %v", err)
		// return nil, err
		// }
		// defer resp.Body.Close()
		// log.Printf("response: %v", string(respBody))

		if resp.StatusCode >= 300 {
			log.Printf("bad response")

			return nil, errors.New("failed to upload to minio via link")
		}

		return nil, nil
	}
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

// SelectVolumes lists and returns available volumes (buckets) matching the filter.
// ✅ .. maybe can enhance with more which factors
func (mc *Client) SelectVolumes(which map[string]any) (any, error) {
	res, err := mc.listBuckets()
	if err != nil {
		log.Printf("failed to retrieve buckets: %v", err)

		return nil, err
	}

	prefix, _ := which["vid"].(string)

	var r []any
	for _, b := range res {
		volume := ut.Volume{
			Name:      b.Name,
			CreatedAt: b.CreationDate.String(),
		}
		if prefix == "" {
			r = append(r, volume)
		} else if strings.Contains(b.Name, prefix) {
			r = append(r, volume)
		}
	}

	return r, nil
}

// SelectObjects lists and returns objects in a specified volume, optionally filtered by prefix.
// ✅
func (mc *Client) SelectObjects(which map[string]any) (any, error) {
	vN, is := which["vname"]
	if !is {
		return nil, errors.New("must specify volume")
	}
	vName, is := vN.(string)
	if !is {
		return nil, errors.New("bad volume identifier")
	}
	// fix vName
	prefix := which["prefix"].(string)
	p := strings.Split(strings.TrimPrefix(prefix, "/"), "/")
	if len(p) > 0 {
		prefix = p[len(p)-1]
	}

	objectCh, cancel := mc.listObjects(vName, prefix)
	defer cancel()

	var objects []ut.Resource
	for object := range objectCh {
		if object.Err != nil {
			fmt.Println(object.Err)

			return nil, object.Err
		}
		rsrc := ut.Resource{
			Name:      object.Key,
			Size:      object.Size,
			Type:      "object",
			Vname:     vName,
			UpdatedAt: object.LastModified.String(),
		}

		objects = append(objects, rsrc)
	}

	return objects, nil
}

// Stat retrieves metadata information for a given object, optionally fetching the object locally.
// ✅
func (mc *Client) Stat(t any) (any, error) {
	object, ok := t.(ut.Resource)
	if !ok {
		return nil, ut.NewError("failed to cast")
	}

	if fetchstat {
		cancel, err := mc.fGetObject(object.Vname, object.Name, mc.defaultLocalSpacePath+object.Name)
		if err != nil {
			return nil, err
		}
		defer cancel()

		info, err := os.Stat(mc.defaultLocalSpacePath + object.Name)
		if err != nil {
			return nil, err
		}

		return info, nil
	}

	return mc.statObject(object.Vname, object.Name)
}

// Remove deletes an object from Minio.
// ✅
func (mc *Client) Remove(t any) error {
	resource, ok := t.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}

	return mc.removeObject(resource.Vname, resource.Name)
}

// RemoveVolume deletes a volume (bucket) from Minio.
// ✅
func (mc *Client) RemoveVolume(t any) error {
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

	return mc.removeBucket(bucketname)
}

// Download retrieves an object from Minio and prepares it for reading.
// ✅
func (mc *Client) Download(t *any) (context.CancelFunc, error) {
	value := *t
	resourcePtr, ok := value.(*ut.Resource)
	if !ok {
		return nil, ut.NewError("failed to cast to *Resource")
	}

	minioObj, cancelFn, err := mc.getObject(resourcePtr.Vname, resourcePtr.Name)
	if err != nil && minioObj == nil {
		log.Printf("failed to get object from minio: %v", err)

		return nil, err
	}

	s, err := minioObj.Stat()
	if err != nil {
		log.Printf("failed to stat the minio object: %v", err)

		return nil, err
	}

	resourcePtr.Size = s.Size
	resourcePtr.Reader = minioObj

	return cancelFn, err
}

// Copy copies the given object to a new destination
func (mc *Client) Copy(s, d any) error {
	src, ok := s.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}
	dst, ok := d.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}

	uploadInfo, err := mc.copyObject(minio.CopySrcOptions{Bucket: src.Vname, Object: src.Name},
		minio.CopyDestOptions{Bucket: dst.Vname, Object: dst.Name})
	if err != nil {
		return err
	}

	log.Printf("upload info: %+v", uploadInfo)

	return nil
}

// Update function is tbd
func (mc *Client) Update(_ map[string]string) error {
	return nil
}

// DefaultVolume returns the default local or remote volume path or bucket name.
// ✅
func (mc *Client) DefaultVolume(local bool) string {
	if local {
		return mc.defaultLocalSpacePath
	}

	return mc.defaultBucketName
}

// Share generates a presigned URL for uploading or downloading an object.
// ✅
func (mc *Client) Share(method string, t any) (any, error) {
	resource, ok := t.(ut.Resource)
	if !ok {
		log.Printf("failed to cast to resource")

		return nil, ut.NewError("bad object, failed to cast to resource")
	}

	switch method {
	case "get":
		url, err := mc.getPresignedObject(resource.Vname, resource.Name, defaultSignDuration)
		if err != nil {
			log.Printf("failed to retrieve object sign")
		}

		return url, err

	case "put":
		url, err := mc.putPresignedObject(resource.Vname, resource.Name, defaultSignDuration)
		if err != nil {
			log.Printf("failed to retrieve object sign")
		}

		return url, err

	default:
		log.Printf("invalid method")

		return nil, ut.NewError("bad method")
	}
}
