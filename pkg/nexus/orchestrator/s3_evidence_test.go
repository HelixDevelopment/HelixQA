package orchestrator

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type fakeS3 struct {
	puts []fakePut
	list []S3Object
	url  string
	err  error
}

type fakePut struct {
	bucket, key, contentType string
	size                     int64
	body                     []byte
}

func (f *fakeS3) PutObject(_ context.Context, bucket, key string, body io.Reader, size int64, ct string) error {
	if f.err != nil {
		return f.err
	}
	raw, _ := io.ReadAll(body)
	f.puts = append(f.puts, fakePut{bucket, key, ct, size, raw})
	return nil
}
func (f *fakeS3) ListObjects(_ context.Context, _, _ string) ([]S3Object, error) {
	return f.list, f.err
}
func (f *fakeS3) GetObjectURL(_, key string) string {
	if f.url != "" {
		return f.url + "/" + key
	}
	return "s3://fake/" + key
}

func TestS3EvidenceStore_Validation(t *testing.T) {
	if _, err := NewS3EvidenceStore(nil, "b", ""); err == nil {
		t.Fatal("nil client must error")
	}
	if _, err := NewS3EvidenceStore(&fakeS3{}, "", ""); err == nil {
		t.Fatal("empty bucket must error")
	}
}

func TestS3EvidenceStore_PutRoutes(t *testing.T) {
	c := &fakeS3{}
	store, _ := NewS3EvidenceStore(c, "bucket", "sessions")
	url, err := store.Put("x/frame.png", []byte("PNGBYTES"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(url, "/sessions/x/frame.png") {
		t.Errorf("url = %q", url)
	}
	if len(c.puts) != 1 {
		t.Fatalf("puts = %d", len(c.puts))
	}
	if c.puts[0].contentType != "image/png" {
		t.Errorf("content type = %q", c.puts[0].contentType)
	}
	if string(c.puts[0].body) != "PNGBYTES" {
		t.Errorf("body = %q", c.puts[0].body)
	}
}

func TestS3EvidenceStore_PutStreamSetsNegativeSize(t *testing.T) {
	c := &fakeS3{}
	store, _ := NewS3EvidenceStore(c, "bucket", "")
	_, err := store.PutStream("sessions/a/log.txt", bytes.NewBufferString("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if c.puts[0].size != -1 {
		t.Errorf("size = %d, want -1", c.puts[0].size)
	}
	if c.puts[0].contentType != "text/plain; charset=utf-8" {
		t.Errorf("content type = %q", c.puts[0].contentType)
	}
}

func TestS3EvidenceStore_ListMapsObjects(t *testing.T) {
	ts := time.Now()
	c := &fakeS3{list: []S3Object{{Key: "sessions/a/log.txt", Size: 5, UpdatedAt: ts}}}
	store, _ := NewS3EvidenceStore(c, "bucket", "")
	items, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items = %d", len(items))
	}
	if items[0].Size != 5 || !items[0].CreatedAt.Equal(ts) {
		t.Errorf("mapping wrong: %+v", items[0])
	}
}

func TestS3EvidenceStore_PutPropagatesError(t *testing.T) {
	c := &fakeS3{err: errors.New("403")}
	store, _ := NewS3EvidenceStore(c, "b", "")
	if _, err := store.Put("x.png", []byte("x")); err == nil {
		t.Fatal("expected error")
	}
}

func TestContentTypeFor(t *testing.T) {
	cases := map[string]string{
		"a.PNG":        "image/png",
		"a.jpg":        "image/jpeg",
		"a.jpeg":       "image/jpeg",
		"a.webp":       "image/webp",
		"a.mp4":        "video/mp4",
		"a.json":       "application/json",
		"a.LOG":        "text/plain; charset=utf-8",
		"a.pdf":        "application/pdf",
		"unknown.blob": "application/octet-stream",
	}
	for in, want := range cases {
		if got := contentTypeFor(in); got != want {
			t.Errorf("contentTypeFor(%q) = %q, want %q", in, got, want)
		}
	}
}
