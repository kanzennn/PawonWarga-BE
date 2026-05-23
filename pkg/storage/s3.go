package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"PawonWarga-BE/internal/config"
	"github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
)

type S3Storage struct {
	client        *minio.Client
	bucket        string
	publicBaseURL string
}

// NewS3 works with any S3-compatible provider (idcloudhost, AWS, Minio,
// DigitalOcean Spaces, Cloudflare R2, Backblaze B2, etc.).
//
// minio-go is used instead of AWS SDK v2 because it always sends the actual
// SHA256 payload hash, which is required by Ceph-based providers (idcloudhost)
// that reject the UNSIGNED-PAYLOAD shortcut that AWS SDK v2 uses by default.
//
// To migrate providers, only the environment variables need to change.
func NewS3(cfg *config.StorageConfig) (*S3Storage, error) {
	endpoint, secure := parseEndpoint(cfg.Endpoint)

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}

	return &S3Storage{
		client:        client,
		bucket:        cfg.Bucket,
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
	}, nil
}

func (s *S3Storage) Upload(ctx context.Context, input UploadInput) (string, error) {
	buf, err := io.ReadAll(input.Body)
	if err != nil {
		return "", fmt.Errorf("s3 read body: %w", err)
	}

	_, err = s.client.PutObject(ctx, s.bucket, input.Key, bytes.NewReader(buf), int64(len(buf)),
		minio.PutObjectOptions{ContentType: input.ContentType},
	)
	if err != nil {
		return "", fmt.Errorf("s3 upload: %w", err)
	}

	return fmt.Sprintf("%s/%s", s.publicBaseURL, input.Key), nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}

// parseEndpoint strips the scheme from the endpoint URL and returns
// whether TLS should be used. minio-go takes host:port without scheme.
func parseEndpoint(endpoint string) (host string, secure bool) {
	switch {
	case strings.HasPrefix(endpoint, "https://"):
		return strings.TrimPrefix(endpoint, "https://"), true
	case strings.HasPrefix(endpoint, "http://"):
		return strings.TrimPrefix(endpoint, "http://"), false
	default:
		return endpoint, true // default to TLS
	}
}
