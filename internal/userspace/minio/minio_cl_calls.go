package minio

import (
	"context"
	"fmt"
	"log"

	"github.com/minio/minio-go/v7"
)

func (mc *MinioClient) CreateBucket(bucketname string) error {

	exists, err := mc.client.BucketExists(context.Background(), bucketname)
	if err != nil {
		log.Printf("failed to check if bucket exists: %v", err)
		return err
	}
	if exists {
		log.Printf("bucket %s already exists", bucketname)
		return fmt.Errorf("bucket %s already exists", bucketname)

	}
	err = mc.client.MakeBucket(context.Background(), bucketname, minio.MakeBucketOptions{
		Region:        region,
		ObjectLocking: mc.objectLocking,
	})
	if err != nil {
		log.Printf("error making bucket: %v", err)
		return err
	}
	return nil
}

func (mc *MinioClient) ListBuckets() ([]minio.BucketInfo, error) {
	buckets, err := mc.client.ListBuckets(context.Background())
	if err != nil {
		log.Printf("error listing buckets: %v", err)
		return nil, err
	}

	log.Printf("buckets: %v", buckets)
	// var bucketInfos []BucketInfo
	// for _, bucket := range buckets {
	// 	bucketInfos = append(bucketInfos, BucketInfo{
	// 		Name:         bucket.Name,
	// 		CreationDate: bucket.CreationDate,
	// 	})
	// }
	return buckets, nil
}

func (mc *MinioClient) RemoveBucket(bucketname string) error {
	err := mc.client.RemoveBucket(context.Background(), bucketname)
	if err != nil {
		log.Printf("error removing bucket: %v", err)
	}

	return err
}

func (mc *MinioClient) ListObjects(bucketname, prefix string, opts minio.ListObjectsOptions) {
	// List all objects from a bucket-name with a matching prefix.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	objectCh := mc.client.ListObjects(ctx, bucketname, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	for object := range objectCh {
		if object.Err != nil {
			fmt.Println(object.Err)
			return
		}
		fmt.Println(object)
	}
}

func (mc *MinioClient) ListIncompleteUploads(bucketname, prefix string, isRecursive bool) {
	// List all incomplete uploads from a bucket-name with a matching prefix.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	objectCh := mc.client.ListIncompleteUploads(ctx, bucketname, prefix, isRecursive)
	for object := range objectCh {
		if object.Err != nil {
			fmt.Println(object.Err)
			return
		}
		fmt.Println(object)
	}
}
