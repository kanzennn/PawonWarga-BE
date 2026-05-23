package storage

import (
	"context"
	"io"
)

// Storage is the provider-agnostic interface for object storage.
// Swap the implementation (S3, GCS, Minio, etc.) without touching callers.
type Storage interface {
	// Upload stores an object and returns its public URL.
	Upload(ctx context.Context, input UploadInput) (url string, err error)
	// Delete removes an object by its key.
	Delete(ctx context.Context, key string) error
}

type UploadInput struct {
	Key         string    // object path inside the bucket, e.g. "profiles/1/abc123.jpg"
	Body        io.Reader
	Size        int64
	ContentType string
}
