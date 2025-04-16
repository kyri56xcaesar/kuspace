package minio

import (
	ut "kyri56xcaesar/myThesis/internal/utils"
	"log"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	region = "eu-central-1"
)

var (
	client *MinioClient
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

func GetOrCreateMinioClient(cfg ut.EnvConfig) (*MinioClient, error) {
	if client == nil {
		mc, err := NewMinioClient(cfg)
		if err != nil {
			return nil, err
		}

		client = &mc
	}
	return client, nil
}

func NewMinioClient(cfg ut.EnvConfig) (MinioClient, error) {
	mc := MinioClient{
		accessKey: cfg.MINIO_ACCESS_KEY,
		secretKey: cfg.MINIO_SECRET_KEY,
		endpoint:  cfg.MINIO_ENDPOINT + ":" + cfg.MINIO_PORT,
		useSSL:    cfg.MINIO_USE_SSL == "true",
	}

	client, err := minio.New(mc.endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(mc.accessKey, mc.secretKey, ""),
		Secure: mc.useSSL,
	})

	if err != nil {
		return MinioClient{}, err
	}

	mc.client = client

	return mc, nil
}

func (mc *MinioClient) Insert(t []any) error {
	log.Printf("creating a bucket o_O")
	return nil
}

func (mc *MinioClient) Select(sel, table, by, byvalue string, limit int) ([]any, error) {
	return nil, nil
}

func (mc *MinioClient) SelectOne(sel, table, by, byvalue string) (any, error) {
	return nil, nil
}

func (mc *MinioClient) Update(t any) error {
	return nil
}

func (mc *MinioClient) Remove(t any) error {
	return nil
}
