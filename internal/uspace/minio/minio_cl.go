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
func (mc *Client) CreateVolume(ctx context.Context, volume any) error {
	v, ok := volume.(ut.Volume)
	if !ok {
		return ut.NewError("failed to cast to a volume")
	}

	log.Printf("volume incoming: %+v", v)

	err := mc.createBucket(ctx, v.Name)
	if err != nil {
		log.Printf("failed to create a bucket on minio: %v", err)

		return err
	}

	return nil
}

// Insert uploads an object to Minio, through a presigned URL above
// objectSizeThreshold (or always, with onlyPresignedUpload).
func (mc *Client) Insert(ctx context.Context, t any) error {
	object, ok := t.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}
	if object.Size <= objectSizeThreshold && !onlyPresignedUpload {
		return mc.putObject(ctx, object.Vname, object.Name, object.Reader, object.Size)
	}

	u, err := mc.putPresignedObject(ctx, object.Vname, object.Name, defaultSignDuration)
	if err != nil {
		return fmt.Errorf("presign upload: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Minute*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), object.Reader)
	if err != nil {
		return err
	}
	req.ContentLength = object.Size
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("upload to minio via link: %s", resp.Status)
	}

	return nil
}

// SelectVolumes lists and returns available volumes (buckets) matching the filter.
// ✅ .. maybe can enhance with more which factors
func (mc *Client) SelectVolumes(ctx context.Context, which map[string]any) (any, error) {
	res, err := mc.listBuckets(ctx)
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
func (mc *Client) SelectObjects(ctx context.Context, which map[string]any) (any, error) {
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

	objectCh, cancel := mc.listObjects(ctx, vName, prefix)
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
func (mc *Client) Stat(ctx context.Context, t any) (any, error) {
	object, ok := t.(ut.Resource)
	if !ok {
		return nil, ut.NewError("failed to cast")
	}

	if fetchstat {
		cancel, err := mc.fGetObject(ctx, object.Vname, object.Name, mc.defaultLocalSpacePath+object.Name)
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

	return mc.statObject(ctx, object.Vname, object.Name)
}

// Remove deletes an object from Minio.
// ✅
func (mc *Client) Remove(ctx context.Context, t any) error {
	resource, ok := t.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}

	return mc.removeObject(ctx, resource.Vname, resource.Name)
}

// RemoveVolume deletes a volume (bucket) from Minio.
// ✅
func (mc *Client) RemoveVolume(ctx context.Context, t any) error {
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

	return mc.removeBucket(ctx, bucketname)
}

// Download retrieves an object from Minio and prepares it for reading.
// ✅
func (mc *Client) Download(ctx context.Context, t *any) (context.CancelFunc, error) {
	value := *t
	resourcePtr, ok := value.(*ut.Resource)
	if !ok {
		return nil, ut.NewError("failed to cast to *Resource")
	}

	minioObj, cancelFn, err := mc.getObject(ctx, resourcePtr.Vname, resourcePtr.Name)
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
func (mc *Client) Copy(ctx context.Context, s, d any) error {
	src, ok := s.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}
	dst, ok := d.(ut.Resource)
	if !ok {
		return ut.NewError("failed to cast")
	}

	uploadInfo, err := mc.copyObject(ctx, minio.CopySrcOptions{Bucket: src.Vname, Object: src.Name},
		minio.CopyDestOptions{Bucket: dst.Vname, Object: dst.Name})
	if err != nil {
		return err
	}

	log.Printf("upload info: %+v", uploadInfo)

	return nil
}

// Update function is tbd
func (mc *Client) Update(_ context.Context, _ map[string]string) error {
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
func (mc *Client) Share(ctx context.Context, method string, t any) (any, error) {
	resource, ok := t.(ut.Resource)
	if !ok {
		log.Printf("failed to cast to resource")

		return nil, ut.NewError("bad object, failed to cast to resource")
	}

	switch method {
	case "get":
		url, err := mc.getPresignedObject(ctx, resource.Vname, resource.Name, defaultSignDuration)
		if err != nil {
			log.Printf("failed to retrieve object sign")
		}

		return url, err

	case "put":
		url, err := mc.putPresignedObject(ctx, resource.Vname, resource.Name, defaultSignDuration)
		if err != nil {
			log.Printf("failed to retrieve object sign")
		}

		return url, err

	default:
		log.Printf("invalid method")

		return nil, ut.NewError("bad method")
	}
}

// PresignFor returns a presigned URL granting method ("get" or "put") on
// exactly one object for d. Jobs get these instead of storage credentials.
func (mc *Client) PresignFor(ctx context.Context, method string, r ut.Resource, d time.Duration) (*url.URL, error) {
	object := strings.TrimPrefix(r.Name, "/")
	switch method {
	case "get":
		return mc.client.PresignedGetObject(ctx, r.Vname, object, d, nil)
	case "put":
		return mc.client.PresignedPutObject(ctx, r.Vname, object, d)
	default:
		return nil, fmt.Errorf("presign: unsupported method %q", method)
	}
}
