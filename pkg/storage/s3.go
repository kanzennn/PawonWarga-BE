package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"PawonWarga-BE/internal/config"
	"github.com/minio/minio-go/v7"
	miniocreds "github.com/minio/minio-go/v7/pkg/credentials"
)

// publicReadPolicy grants anonymous s3:GetObject on everything under
// "profiles/" — the only prefix this app ever writes to (see generateKey in
// internal/service/auth.go). minio-go's PutObjectOptions has no canned-ACL
// field (MinIO itself has no concept of per-object ACLs), so making an
// object readable at its public URL has to happen via a bucket policy
// instead of a per-upload flag.
const publicReadPolicy = `{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Effect": "Allow",
			"Principal": {"AWS": ["*"]},
			"Action": ["s3:GetObject"],
			"Resource": ["arn:aws:s3:::%s/profiles/*"]
		}
	]
}`

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
	// Required, not just recommended: PublicURL() blindly concatenates this
	// with the object key, so an empty value would silently hand out broken
	// links (e.g. "/profiles/1/abc.jpg" with no host) instead of failing loudly.
	if cfg.PublicBaseURL == "" {
		return nil, errors.New("STORAGE_PUBLIC_BASE_URL is required when storage is configured")
	}

	endpoint, secure := parseEndpoint(cfg.Endpoint)

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  miniocreds.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: secure,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("s3 client: %w", err)
	}

	// Best-effort: don't fail startup over this. Some providers/credentials
	// don't allow PutBucketPolicy (e.g. an access key without that
	// permission, or a policy dialect the provider doesn't accept), and the
	// rest of the app still works — uploaded files just won't be publicly
	// viewable at their URL until the bucket policy is fixed, same as today.
	policyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.SetBucketPolicy(policyCtx, cfg.Bucket, fmt.Sprintf(publicReadPolicy, cfg.Bucket)); err != nil {
		log.Printf("storage: could not set public-read bucket policy on %q (uploaded files may stay private): %v", cfg.Bucket, err)
	}

	return &S3Storage{
		client:        client,
		bucket:        cfg.Bucket,
		publicBaseURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
	}, nil
}

func (s *S3Storage) Upload(ctx context.Context, input UploadInput) error {
	buf, err := io.ReadAll(input.Body)
	if err != nil {
		return fmt.Errorf("s3 read body: %w", err)
	}

	_, err = s.client.PutObject(ctx, s.bucket, input.Key, bytes.NewReader(buf), int64(len(buf)),
		minio.PutObjectOptions{ContentType: input.ContentType},
	)
	if err != nil {
		return fmt.Errorf("s3 upload: %w", err)
	}

	return nil
}

func (s *S3Storage) Delete(ctx context.Context, key string) error {
	err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}

func (s *S3Storage) PublicURL(key string) string {
	return fmt.Sprintf("%s/%s", s.publicBaseURL, key)
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
