package minio

import (
	ut "kyri56xcaesar/myThesis/internal/utils"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioClient struct {
	accessKey string
	secretKey string
	endpoint  string
	useSSL    bool
	client    *minio.Client
}

func NewMinioClient(cfg ut.EnvConfig) MinioClient {
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
		panic(err)
	}

	mc.client = client

	return mc
}
