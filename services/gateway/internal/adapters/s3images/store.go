// Package s3images stores submitted images in an S3-compatible bucket (decision record 0003).
package s3images

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Config locates the store. The AWS SDK is used only as a generic S3 protocol client.
type Config struct {
	Endpoint, AccessKey, SecretKey, Region, Bucket string
}

// Store implements ports.ImageStore.
type Store struct {
	client *s3.Client
	bucket string
}

// New returns a store for cfg; call EnsureBucket before use.
func New(cfg Config) *Store {
	client := s3.New(s3.Options{
		BaseEndpoint: aws.String(cfg.Endpoint),
		Region:       cfg.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		UsePathStyle: true, // SeaweedFS and most self-hosted stores don't do virtual-host buckets
	})
	return &Store{client: client, bucket: cfg.Bucket}
}

// EnsureBucket creates the bucket if it doesn't exist.
func (s *Store) EnsureBucket(ctx context.Context) error {
	_, err := s.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(s.bucket)})
	var owned *types.BucketAlreadyOwnedByYou
	var exists *types.BucketAlreadyExists
	if err == nil || errors.As(err, &owned) || errors.As(err, &exists) {
		return nil
	}
	return fmt.Errorf("create bucket %s: %w", s.bucket, err)
}

// Put stores data under key and returns its s3:// URI.
func (s *Store) Put(ctx context.Context, key string, data []byte) (string, error) {
	if _, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	}); err != nil {
		return "", fmt.Errorf("put s3://%s/%s: %w", s.bucket, key, err)
	}
	return fmt.Sprintf("s3://%s/%s", s.bucket, key), nil
}

// Get returns an object's bytes (used by tests and tools).
func (s *Store) Get(ctx context.Context, key string) ([]byte, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	defer out.Body.Close()
	var buf bytes.Buffer
	_, err = buf.ReadFrom(out.Body)
	return buf.Bytes(), err
}
