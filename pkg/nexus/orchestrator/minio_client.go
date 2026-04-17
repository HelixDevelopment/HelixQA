package orchestrator

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinioClient wraps the official minio-go v7 SDK and satisfies the
// S3Client contract the S3EvidenceStore depends on. Operators construct
// a MinioClient once per deployment and share it across evidence + any
// other S3-compatible needs (MinIO, AWS S3, Backblaze B2, Cloudflare R2).
type MinioClient struct {
	client *minio.Client
	public string // public URL base for presigned-free retrieval (optional)
}

// NewMinioClient dials the configured endpoint. Secure toggles TLS;
// accessKey + secretKey authenticate the operator. The returned client
// is ready to drive S3EvidenceStore.
func NewMinioClient(endpoint, accessKey, secretKey string, secure bool) (*MinioClient, error) {
	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: secure,
	})
	if err != nil {
		return nil, fmt.Errorf("minio: %w", err)
	}
	scheme := "https"
	if !secure {
		scheme = "http"
	}
	return &MinioClient{
		client: cli,
		public: scheme + "://" + strings.TrimRight(endpoint, "/"),
	}, nil
}

// WithPublicURLBase lets operators override the URL prefix returned by
// GetObjectURL — useful when the bucket is served behind a CDN that
// differs from the S3 endpoint.
func (c *MinioClient) WithPublicURLBase(base string) *MinioClient {
	c.public = strings.TrimRight(base, "/")
	return c
}

// PutObject satisfies S3Client.
func (c *MinioClient) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error {
	_, err := c.client.PutObject(ctx, bucket, key, body, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("minio put %s/%s: %w", bucket, key, err)
	}
	return nil
}

// ListObjects satisfies S3Client.
func (c *MinioClient) ListObjects(ctx context.Context, bucket, prefix string) ([]S3Object, error) {
	ch := c.client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	out := make([]S3Object, 0, 32)
	for o := range ch {
		if o.Err != nil {
			return nil, fmt.Errorf("minio list: %w", o.Err)
		}
		out = append(out, S3Object{
			Key:       o.Key,
			Size:      o.Size,
			UpdatedAt: o.LastModified,
		})
	}
	return out, nil
}

// GetObjectURL returns a stable path URL for the configured backend.
// Callers who need presigned URLs should call the underlying client
// directly (`MinioClient.Raw()`).
func (c *MinioClient) GetObjectURL(bucket, key string) string {
	return c.public + "/" + bucket + "/" + strings.TrimLeft(key, "/")
}

// Raw exposes the underlying minio-go client so advanced callers can
// reach features beyond the narrow S3Client contract (presigned URLs,
// lifecycle rules, replication, etc.).
func (c *MinioClient) Raw() *minio.Client { return c.client }

var _ S3Client = (*MinioClient)(nil)
