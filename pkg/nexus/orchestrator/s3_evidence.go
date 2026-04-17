package orchestrator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

// S3Client is the narrow SDK contract the S3EvidenceStore needs from
// whichever S3-compatible client the operator chooses (AWS SDK v2,
// MinIO Go client, or a custom wrapper). Adapters implement this
// three-method interface so the evidence store stays SDK-agnostic.
type S3Client interface {
	PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error
	ListObjects(ctx context.Context, bucket, prefix string) ([]S3Object, error)
	GetObjectURL(bucket, key string) string
}

// S3Object describes one object returned by ListObjects.
type S3Object struct {
	Key       string
	Size      int64
	UpdatedAt time.Time
}

// S3EvidenceStore wraps an S3Client and satisfies EvidenceStore.
// Operators who prefer MinIO / AWS S3 / Backblaze B2 point this at
// their SDK through a one-off adapter.
type S3EvidenceStore struct {
	Bucket string
	Prefix string

	mu     sync.Mutex
	client S3Client
}

// NewS3EvidenceStore returns a store that writes into bucket under an
// optional prefix. The client may be shared with other subsystems.
func NewS3EvidenceStore(client S3Client, bucket, prefix string) (*S3EvidenceStore, error) {
	if client == nil {
		return nil, errors.New("s3 evidence: nil client")
	}
	if bucket == "" {
		return nil, errors.New("s3 evidence: bucket is required")
	}
	return &S3EvidenceStore{client: client, Bucket: bucket, Prefix: prefix}, nil
}

// Put writes data to prefix/name.
func (s *S3EvidenceStore) Put(name string, data []byte) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.key(name)
	if err := s.client.PutObject(context.Background(), s.Bucket, key, bytes.NewReader(data), int64(len(data)), contentTypeFor(name)); err != nil {
		return "", fmt.Errorf("s3 put: %w", err)
	}
	return s.client.GetObjectURL(s.Bucket, key), nil
}

// PutStream streams r to prefix/name. The size is unknown so the
// adapter passes -1; clients that require content length should
// buffer in the adapter.
func (s *S3EvidenceStore) PutStream(name string, r io.Reader) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.key(name)
	if err := s.client.PutObject(context.Background(), s.Bucket, key, r, -1, contentTypeFor(name)); err != nil {
		return "", fmt.Errorf("s3 put stream: %w", err)
	}
	return s.client.GetObjectURL(s.Bucket, key), nil
}

// List enumerates stored artefacts under the configured prefix.
func (s *S3EvidenceStore) List() ([]EvidenceItem, error) {
	items, err := s.client.ListObjects(context.Background(), s.Bucket, s.Prefix)
	if err != nil {
		return nil, fmt.Errorf("s3 list: %w", err)
	}
	out := make([]EvidenceItem, 0, len(items))
	for _, it := range items {
		out = append(out, EvidenceItem{
			Name:      it.Key,
			URL:       s.client.GetObjectURL(s.Bucket, it.Key),
			Size:      it.Size,
			CreatedAt: it.UpdatedAt,
		})
	}
	return out, nil
}

func (s *S3EvidenceStore) key(name string) string {
	if s.Prefix == "" {
		return name
	}
	return s.Prefix + "/" + name
}

func contentTypeFor(name string) string {
	switch {
	case hasSuffixFold(name, ".png"):
		return "image/png"
	case hasSuffixFold(name, ".jpg"), hasSuffixFold(name, ".jpeg"):
		return "image/jpeg"
	case hasSuffixFold(name, ".webp"):
		return "image/webp"
	case hasSuffixFold(name, ".mp4"):
		return "video/mp4"
	case hasSuffixFold(name, ".json"):
		return "application/json"
	case hasSuffixFold(name, ".log"), hasSuffixFold(name, ".txt"):
		return "text/plain; charset=utf-8"
	case hasSuffixFold(name, ".pdf"):
		return "application/pdf"
	}
	return "application/octet-stream"
}

func hasSuffixFold(s, suffix string) bool {
	if len(s) < len(suffix) {
		return false
	}
	a := s[len(s)-len(suffix):]
	return equalFold(a, suffix)
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		c1 := a[i]
		c2 := b[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}
