// Package storage wraps the S3 bucket holding photos and call recordings.
package storage

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"

	"cameo/internal/config"
)

type Service struct {
	// internal performs server-side object operations.
	internal *minio.Client
	// public presigns URLs against the endpoint clients can reach.
	public *minio.Client
	bucket string
}

// New builds both clients and creates the bucket when it does not exist.
func New(ctx context.Context, cfg *config.Storage) (*Service, error) {
	internal, err := cfg.Client()
	if err != nil {
		return nil, fmt.Errorf("create storage client: %w", err)
	}

	public, err := cfg.PublicClient()
	if err != nil {
		return nil, fmt.Errorf("create public storage client: %w", err)
	}

	exists, err := internal.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("check bucket %q: %w", cfg.Bucket, err)
	}
	if !exists {
		err = internal.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region})
		if code := minio.ToErrorResponse(err).Code; err != nil && code != minio.BucketAlreadyOwnedByYou && code != minio.BucketAlreadyExists {
			return nil, fmt.Errorf("create bucket %q: %w", cfg.Bucket, err)
		}
	}

	return &Service{internal: internal, public: public, bucket: cfg.Bucket}, nil
}

// PresignPut returns a URL the client uploads the object to with a plain PUT.
func (s *Service) PresignPut(ctx context.Context, key string, expiry time.Duration) (*url.URL, error) {
	return s.public.PresignedPutObject(ctx, s.bucket, key, expiry)
}

// PresignGet returns a URL the client downloads the object from.
func (s *Service) PresignGet(ctx context.Context, key string, expiry time.Duration) (*url.URL, error) {
	return s.public.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
}

// Stat returns the object's metadata. A missing object yields an error whose
// minio.ToErrorResponse(err).Code is minio.NoSuchKey.
func (s *Service) Stat(ctx context.Context, key string) (minio.ObjectInfo, error) {
	return s.internal.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
}

func (s *Service) Remove(ctx context.Context, key string) error {
	return s.internal.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// RemovePrefix removes every object whose key starts with prefix.
func (s *Service) RemovePrefix(ctx context.Context, prefix string) error {
	objects := s.internal.ListObjectsIter(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true})
	results, err := s.internal.RemoveObjectsWithIter(ctx, s.bucket, objects, minio.RemoveObjectsOptions{})
	if err != nil {
		return fmt.Errorf("remove prefix %q: %w", prefix, err)
	}

	for result := range results {
		if result.Err != nil {
			return fmt.Errorf("remove %q: %w", result.ObjectName, result.Err)
		}
	}
	return nil
}

// FPut uploads the local file at path.
func (s *Service) FPut(ctx context.Context, key, path, contentType string) error {
	_, err := s.internal.FPutObject(ctx, s.bucket, key, path, minio.PutObjectOptions{ContentType: contentType})
	return err
}
