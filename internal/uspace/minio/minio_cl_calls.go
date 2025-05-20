package minio

/*
	a set of functions calls as client to a minio service
*/

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"time"

	"github.com/minio/minio-go/v7"
)

/* bucket crud */
func (mc *MinioClient) createBucket(bucketname string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exists, err := mc.client.BucketExists(ctx, bucketname)
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

func (mc *MinioClient) listBuckets() ([]minio.BucketInfo, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	buckets, err := mc.client.ListBuckets(ctx)
	if err != nil {
		// log.Printf("error listing buckets: %v", err)
		return nil, err
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

func (mc *MinioClient) bucketExists(bucketname string) (bool, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exists, err := mc.client.BucketExists(ctx, bucketname)
	if err != nil {
		log.Printf("failed to check if bucket exists: %v", err)
		return false, err
	}
	return exists, nil
}

func (mc *MinioClient) removeBucket(bucketname string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := mc.client.RemoveBucket(ctx, bucketname)
	if err != nil {
		log.Printf("error removing bucket: %v", err)
	}

	return err
}

func (mc *MinioClient) listObjects(bucketname, prefix string) (<-chan minio.ObjectInfo, context.CancelFunc, error) {
	// List all objects from a bucket-name with a matching prefix.
	ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	objectCh := mc.client.ListObjects(ctx, bucketname, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})

	return objectCh, cancel, nil
}

func (mc *MinioClient) listIncompleteUploads(bucketname, prefix string, isRecursive bool) {
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

// bucket control

// direct object to/from fs minio
func (mc *MinioClient) fPutObject(bucketname, objectname, filepath string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// determine content-type
	contentType := "application/json"

	uploadInfo, err := mc.client.FPutObject(ctx, bucketname, objectname, filepath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		log.Printf("failed to fput object to minio: %v", err)
		return
	}

	log.Println("successfully uploaded object: ", uploadInfo)
}

func (mc *MinioClient) fGetObject(bucketname, objectname, filepath string) (context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(context.Background())

	err := mc.client.FGetObject(ctx, bucketname, objectname, filepath, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("failed to get object from minio and save it locally")
		cancel()
		return nil, err
	}

	return cancel, nil
}

// object crud
// stream of the object from minio, similar to FGetObject but without save
func (mc *MinioClient) getObject(bucketname, objectname string) (*minio.Object, context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(context.Background())

	object, err := mc.client.GetObject(ctx, bucketname, objectname, minio.GetObjectOptions{})
	if err != nil {
		log.Printf("failed to retrieve object stream from minio")
		cancel()
		return nil, nil, err
	}

	return object, cancel, nil
	// idk what to do with the stream yet... we'll see!
}

// stream of the object to minio
func (mc *MinioClient) putObject(bucketname, objectname string, reader io.Reader, objectSize int64) (context.CancelFunc, error) {
	ctx, cancel := context.WithCancel(context.Background())

	uploadInfo, err := mc.client.PutObject(ctx, bucketname, objectname, reader, objectSize, minio.PutObjectOptions{})
	if err != nil {
		log.Printf("failed to put object to minio: %v", err)
		cancel()
		return nil, err
	}

	log.Printf("upload info: %+v", uploadInfo)
	return cancel, nil
}

func (mc *MinioClient) statObject(bucketname, objectname string) (minio.ObjectInfo, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	objInfo, err := mc.client.StatObject(ctx, bucketname, objectname, minio.StatObjectOptions{})
	if err != nil {
		log.Println("failed to stat object on minio: ", err)
		return objInfo, err
	}

	log.Println("object statted: ", objInfo)
	return objInfo, nil
}

func (mc *MinioClient) copyObject(origin minio.CopySrcOptions, output minio.CopyDestOptions) (minio.UploadInfo, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	uploadInfo, err := mc.client.CopyObject(ctx, output, origin)
	if err != nil {
		log.Print("failed to initiate copy on minio: ", err)
	}
	return uploadInfo, err
}

func (mc *MinioClient) removeObject(bucketname, objectname string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := mc.client.RemoveObject(ctx, bucketname, objectname, minio.RemoveObjectOptions{})
	if err != nil {
		log.Print("failed to remove object from minio: ", err)
	}

	return err
}

func (mc *MinioClient) removeObjects(bucketname string, objects <-chan minio.ObjectInfo) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := minio.RemoveObjectsOptions{
		GovernanceBypass: true,
	}

	for rErr := range mc.client.RemoveObjects(ctx, bucketname, objects, opts) {
		log.Print("error deleting objects: ", rErr)
	}
}

func (mc *MinioClient) selectObjectContent(bucketname, objectname string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	opts := minio.SelectObjectOptions{
		Expression:     "select count(*) from s3object",
		ExpressionType: minio.QueryExpressionTypeSQL,
		InputSerialization: minio.SelectObjectInputSerialization{
			CompressionType: minio.SelectCompressionNONE,
			CSV: &minio.CSVInputOptions{
				FileHeaderInfo:  minio.CSVFileHeaderInfoNone,
				RecordDelimiter: "\n",
				FieldDelimiter:  ",",
			},
		},
		OutputSerialization: minio.SelectObjectOutputSerialization{
			CSV: &minio.CSVOutputOptions{
				RecordDelimiter: "\n",
				FieldDelimiter:  ",",
			},
		},
	}

	reader, err := mc.client.SelectObjectContent(ctx, bucketname, objectname, opts)
	if err != nil {
		log.Fatalln(err)
	}
	defer reader.Close()

	if _, err := io.Copy(os.Stdout, reader); err != nil {
		log.Fatalln(err)
	}
}

// object control
func (mc *MinioClient) getObjectAttributes(bucketname, objectname string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	objectAttributes, err := mc.client.GetObjectAttributes(
		ctx,
		bucketname,
		objectname,
		minio.ObjectAttributesOptions{
			VersionID: "object-version-id",
			MaxParts:  100,
		})

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(objectAttributes)
}

func (mc *MinioClient) getPresignedObject(bucketname, objectname string, duration time.Duration) (*url.URL, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reqParams := make(url.Values)
	reqParams.Set("response-content-disposition", fmt.Sprintf("attachment; filename=\"%s\"", objectname))

	presignedURL, err := mc.client.PresignedGetObject(ctx, bucketname, objectname, duration, reqParams)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return presignedURL, nil
}

func (mc *MinioClient) putPresignedObject(bucketname, objectname string, duration time.Duration) (*url.URL, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	presignedURL, err := mc.client.PresignedPutObject(ctx, bucketname, objectname, duration)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	return presignedURL, nil
}
