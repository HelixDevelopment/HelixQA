package orchestrator

import (
	"strings"
	"testing"
)

func TestNewMinioClient_BuildsPublicBase(t *testing.T) {
	c, err := NewMinioClient("s3.example.com", "a", "b", true)
	if err != nil {
		t.Fatal(err)
	}
	if c.public != "https://s3.example.com" {
		t.Errorf("public base = %q", c.public)
	}
	if !strings.Contains(c.GetObjectURL("my-bucket", "sessions/x.png"), "https://s3.example.com/my-bucket/sessions/x.png") {
		t.Errorf("GetObjectURL wrong: %q", c.GetObjectURL("my-bucket", "sessions/x.png"))
	}
}

func TestNewMinioClient_Insecure(t *testing.T) {
	c, err := NewMinioClient("localhost:9000", "a", "b", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(c.GetObjectURL("b", "k"), "http://") {
		t.Errorf("URL should use http:// on insecure mode")
	}
}

func TestMinioClient_WithPublicURLBase(t *testing.T) {
	c, _ := NewMinioClient("s3.example.com", "a", "b", true)
	c.WithPublicURLBase("https://cdn.example.com")
	if got := c.GetObjectURL("bucket", "file.png"); got != "https://cdn.example.com/bucket/file.png" {
		t.Errorf("CDN override not applied: %q", got)
	}
}

func TestMinioClient_GetObjectURLStripsLeadingSlash(t *testing.T) {
	c, _ := NewMinioClient("s3.example.com", "a", "b", true)
	got := c.GetObjectURL("bucket", "/path/to/x.png")
	if strings.Count(got, "//") != 1 { // only the scheme's double slash
		t.Errorf("expected single scheme '//' in %q", got)
	}
}

func TestMinioClient_RawAccessor(t *testing.T) {
	c, _ := NewMinioClient("s3.example.com", "a", "b", true)
	if c.Raw() == nil {
		t.Fatal("Raw() must expose the underlying client")
	}
}
