package storage

import (
	"context"
	"io"
)

// Storage is the provider-agnostic interface for object storage.
// Swap the implementation (S3, GCS, Minio, etc.) without touching callers.
type Storage interface {
	// Upload stores an object at the given key.
	Upload(ctx context.Context, input UploadInput) error
	// Delete removes an object by its key.
	Delete(ctx context.Context, key string) error
	// PublicURL returns the current public URL for an object key. It is
	// computed from the live config on every call rather than persisted,
	// so URLs handed to clients always reflect whichever provider is
	// configured right now — callers should store the key, not the URL.
	PublicURL(key string) string
}

type UploadInput struct {
	Key         string    // object path inside the bucket, e.g. "profiles/1/abc123.jpg"
	Body        io.Reader
	Size        int64
	ContentType string
}
