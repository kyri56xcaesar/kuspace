package minio

/*
	a set of functions calls as client to a minio service
*/

import (
	"context"
	"fmt"
	"io"
	ut "kyri56xcaesar/kuspace/internal/utils"
	"log"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
)

/* bucket crud */
func (mc *Client) createBucket(ctx context.Context, bucketname string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	exists, err := mc.client.BucketExists(ctx, bucketname)
	if err != nil {
		log.Printf("failed to check if bucket exists: %v", err)

		return mapErr(err)
	}
	if exists {
		log.Printf("bucket %s already exists", bucketname)

		return fmt.Errorf("bucket %s %w", bucketname, ut.ErrExists)
	}
	err = mc.client.MakeBucket(ctx, bucketname, minio.MakeBucketOptions{
		Region:        region,
		ObjectLocking: mc.objectLocking,
	})
	if err != nil {
		log.Printf("error making bucket: %v", err)

		return mapErr(err)
	}

	return nil
}

func (mc *Client) listBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	buckets, err := mc.client.ListBuckets(ctx)
	if err != nil {
		// log.Printf("error listing buckets: %v", err)
		return nil, mapErr(err)
	}

	// log.Printf("buckets: %v", buckets)
	// var bucketInfos []BucketInfo
	// for _, bucket := range buckets {
	// 	bucketInfos = append(bucketInfos, BucketInfo{
	// 		Name:         bucket.Name,
	// 		CreationDate: bucket.CreationDate,
	// 	})
	// }

	return buckets, nil
}

func (mc *Client) removeBucket(ctx context.Context, bucketname string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := mc.client.RemoveBucket(ctx, bucketname)
	if err != nil {
		log.Printf("error removing bucket: %v", err)
	}

	return mapErr(err)
}

func (mc *Client) listObjects(ctx context.Context, bucketname, prefix string) (<-chan minio.ObjectInfo, context.CancelFunc) {
	// List all objects from a bucket-name with a matching prefix.
	ctx, cancel := context.WithCancel(ctx)
	// defer cancel()

	objectCh := mc.client.ListObjects(ctx, bucketname, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	return objectCh, cancel
}

// bucket control

func (mc *Client) fGetObject(ctx context.Context, bucketname, objectname, filepath string) (context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(ctx)

	err := mc.client.FGetObject(ctx, bucketname, objectname, filepath, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("failed to get object from minio and save it locally")
		cancel()

		return nil, mapErr(err)
	}

	return cancel, nil
}

// object crud
// stream of the object from minio, similar to FGetObject but without save
func (mc *Client) getObject(ctx context.Context, bucketname, objectname string) (*minio.Object, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(ctx)

	object, err := mc.client.GetObject(ctx, bucketname, objectname, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("failed to retrieve object stream from minio")
		cancel()

		return nil, nil, mapErr(err)
	}

	return object, cancel, nil
	// idk what to do with the stream yet... we'll see!
}

// stream of the object to minio
func (mc *Client) putObject(ctx context.Context, bucketname, objectname string, reader io.Reader, objectSize int64) error {
	uploadInfo, err := mc.client.PutObject(ctx, bucketname, objectname, reader, objectSize, minio.PutObjectOptions{})
	if err != nil {
		log.Printf("failed to put object to minio: %v", err)

		return mapErr(err)
	}
	log.Printf("upload info: %+v", uploadInfo)

	return nil
}

func (mc *Client) statObject(ctx context.Context, bucketname, objectname string) (minio.ObjectInfo, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	objInfo, err := mc.client.StatObject(ctx, bucketname, objectname, minio.StatObjectOptions{})
	if err != nil {
		log.Println("failed to stat object on minio: ", err)

		return objInfo, mapErr(err)
	}

	return objInfo, nil
}

func (mc *Client) copyObject(ctx context.Context, origin minio.CopySrcOptions, output minio.CopyDestOptions) (minio.UploadInfo, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	uploadInfo, err := mc.client.CopyObject(ctx, output, origin)
	if err != nil {
		log.Print("failed to initiate copy on minio: ", err)
	}

	return uploadInfo, mapErr(err)
}

func (mc *Client) removeObject(ctx context.Context, bucketname, objectname string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err := mc.client.RemoveObject(ctx, bucketname, objectname, minio.RemoveObjectOptions{})
	if err != nil {
		log.Print("failed to remove object from minio: ", err)
	}

	return mapErr(err)
}

func (mc *Client) getPresignedObject(ctx context.Context, bucketname, objectname string, duration time.Duration) (*url.URL, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", objectname))

	presignedURL, err := mc.client.PresignedGetObject(ctx, bucketname, objectname, duration, reqParams)
	if err != nil {
		log.Println(err)

		return nil, mapErr(err)
	}

	return presignedURL, nil
}

func (mc *Client) putPresignedObject(ctx context.Context, bucketname, objectname string, duration time.Duration) (*url.URL, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	presignedURL, err := mc.client.PresignedPutObject(ctx, bucketname, objectname, duration)
	if err != nil {
		log.Println(err)

		return nil, mapErr(err)
	}

	return presignedURL, nil
}
