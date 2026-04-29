// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Real-MinIO container integration test for S3EvidenceStore. Runs
// only under `go test -tags=integration ./pkg/nexus/orchestrator/...`
// so the default unit-test path stays free of container dependencies.
//
// P10 closure from docs/nexus/remaining-work.md: exercises the
// PutObject + ListObjects + GetObjectURL happy paths against a real
// MinIO container via testcontainers-go so the mock-only coverage
// gets a matching live-fire sibling.
//
// When neither Podman nor Docker is available, the test is skipped
// so CI running on a bare-metal host without a container runtime
// stays green.

package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// minioContainer boots a MinIO container with deterministic
// credentials + returns the endpoint + credentials + a cleanup fn.
func minioContainer(t *testing.T, ctx context.Context) (string, string, string, func()) {
	t.Helper()
	const accessKey = "helixqatest"
	const secretKey = "helixqatestsecret"

	req := testcontainers.ContainerRequest{
		Image:        "docker.io/minio/minio:latest",
		ExposedPorts: []string{"9000/tcp"},
		Cmd:          []string{"server", "/data"},
		Env: map[string]string{
			"MINIO_ROOT_USER":     accessKey,
			"MINIO_ROOT_PASSWORD": secretKey,
		},
		WaitingFor: wait.ForLog("1 Online").WithStartupTimeout(90 * time.Second),
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Skipf("testcontainers: %v (docker/podman socket required)", err)  // SKIP-OK: #legacy-skip-untriaged-2026-04-29
	}
	host, err := c.Host(ctx)
	if err != nil {
		_ = c.Terminate(ctx)
		t.Fatalf("container host: %v", err)
	}
	port, err := c.MappedPort(ctx, "9000/tcp")
	if err != nil {
		_ = c.Terminate(ctx)
		t.Fatalf("mapped port: %v", err)
	}
	endpoint := fmt.Sprintf("%s:%s", host, port.Port())
	cleanup := func() { _ = c.Terminate(context.Background()) }
	return endpoint, accessKey, secretKey, cleanup
}

// minioClientAdapter wraps the official MinIO SDK into the narrow
// S3Client contract expected by S3EvidenceStore. Lives in the test
// file so the library stays SDK-agnostic.
type minioClientAdapter struct {
	cli      *minio.Client
	endpoint string
}

func (m *minioClientAdapter) PutObject(ctx context.Context, bucket, key string, body io.Reader, size int64, contentType string) error {
	_, err := m.cli.PutObject(ctx, bucket, key, body, size, minio.PutObjectOptions{ContentType: contentType})
	return err
}

func (m *minioClientAdapter) ListObjects(ctx context.Context, bucket, prefix string) ([]S3Object, error) {
	var out []S3Object
	for obj := range m.cli.ListObjects(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		out = append(out, S3Object{
			Key:       obj.Key,
			Size:      obj.Size,
			UpdatedAt: obj.LastModified,
		})
	}
	return out, nil
}

func (m *minioClientAdapter) GetObjectURL(bucket, key string) string {
	return fmt.Sprintf("http://%s/%s/%s", m.endpoint, bucket, url.PathEscape(key))
}

// TestS3EvidenceStore_P10_PutAndListAgainstRealMinIO locks in the
// P10 closure: the mock-only happy-path coverage gets a parallel
// live-fire sibling here.
func TestS3EvidenceStore_P10_PutAndListAgainstRealMinIO(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	endpoint, accessKey, secretKey, cleanup := minioContainer(t, ctx)
	defer cleanup()

	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}

	bucket := "helixqa-p10"
	if err := cli.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		// Ignore "already exists" races across retries.
		if !strings.Contains(err.Error(), "Bucket already") {
			t.Fatalf("MakeBucket: %v", err)
		}
	}

	adapter := &minioClientAdapter{cli: cli, endpoint: endpoint}
	store, err := NewS3EvidenceStore(adapter, bucket, "session-p10")
	if err != nil {
		t.Fatal(err)
	}

	// Put byte payload.
	pngURL, err := store.Put("frames/frame-001.png", []byte{0x89, 0x50, 0x4e, 0x47})
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !strings.Contains(pngURL, bucket) || !strings.Contains(pngURL, "frame-001.png") {
		t.Errorf("unexpected URL: %q", pngURL)
	}

	// Put via stream.
	logURL, err := store.PutStream("logs/run.log", bytes.NewReader([]byte("line 1\nline 2\n")))
	if err != nil {
		t.Fatalf("PutStream: %v", err)
	}
	if !strings.Contains(logURL, "run.log") {
		t.Errorf("stream URL missing key: %q", logURL)
	}

	// List must return both objects with non-zero sizes + timestamps.
	items, err := store.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("List len = %d, want 2 (items=%+v)", len(items), items)
	}
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.Name] = true
		if item.Size <= 0 {
			t.Errorf("item %s has size %d", item.Name, item.Size)
		}
		if item.CreatedAt.IsZero() {
			t.Errorf("item %s has zero mtime", item.Name)
		}
		if item.URL == "" {
			t.Errorf("item %s has empty URL", item.Name)
		}
	}
	for _, want := range []string{"session-p10/frames/frame-001.png", "session-p10/logs/run.log"} {
		if !seen[want] {
			t.Errorf("expected key %q in listing, got %+v", want, items)
		}
	}

	// Verify bytes round-trip: GET the frame back via the MinIO SDK.
	obj, err := cli.GetObject(ctx, bucket, "session-p10/frames/frame-001.png", minio.GetObjectOptions{})
	if err != nil {
		t.Fatalf("GetObject: %v", err)
	}
	defer obj.Close()
	buf, err := io.ReadAll(obj)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(buf, []byte{0x89, 0x50, 0x4e, 0x47}) {
		t.Errorf("byte round-trip mismatch, got %x", buf)
	}
}

// TestS3EvidenceStore_P10_RejectsMissingBucket proves the adapter
// surfaces a real MinIO error (not a panic or silent nil) when the
// bucket does not exist.
func TestS3EvidenceStore_P10_RejectsMissingBucket(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	endpoint, accessKey, secretKey, cleanup := minioContainer(t, ctx)
	defer cleanup()

	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		t.Fatalf("minio.New: %v", err)
	}
	adapter := &minioClientAdapter{cli: cli, endpoint: endpoint}
	store, _ := NewS3EvidenceStore(adapter, "does-not-exist", "")

	_, err = store.Put("nope.txt", []byte("x"))
	if err == nil {
		t.Fatal("expected error putting to missing bucket")
	}
	if !strings.Contains(err.Error(), "s3 put:") {
		t.Errorf("error missing wrapped prefix: %v", err)
	}
}
