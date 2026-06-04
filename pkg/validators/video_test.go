package validators

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func haveBinary(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func writeIntoDir(t *testing.T, dir, name, content string) error {
	t.Helper()
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
}

// makeMP4 creates a real short H.264 mp4 using ffmpeg (testsrc, ~1s, 320x240).
func makeMP4(t *testing.T, path string) {
	t.Helper()
	cmd := exec.Command("ffmpeg",
		"-y",
		"-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=10",
		"-pix_fmt", "yuv420p",
		path,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("ffmpeg failed: %v\n%s", err, out)
	}
}

func TestVideoValidator_Supports(t *testing.T) {
	v := NewVideoValidator("")
	if !v.Supports("clip.mp4") {
		t.Error("expected Supports(.mp4) = true")
	}
	if v.Supports("clip.png") {
		t.Error("expected Supports(.png) = false")
	}
}

func TestNewVideoValidator_DefaultPath(t *testing.T) {
	v := NewVideoValidator("")
	if v.ffprobePath != "ffprobe" {
		t.Errorf("empty path should default to 'ffprobe', got %q", v.ffprobePath)
	}
	v2 := NewVideoValidator("/custom/ffprobe")
	if v2.ffprobePath != "/custom/ffprobe" {
		t.Errorf("custom path not preserved, got %q", v2.ffprobePath)
	}
}

// TestVideoValidator_NoFFprobe verifies the honest failure path when ffprobe is
// absent: IsValid=false with an explanatory error + install hint.
func TestVideoValidator_NoFFprobe(t *testing.T) {
	// Point at a binary that definitely does not exist.
	v := NewVideoValidator("/nonexistent/ffprobe-xyz")
	p := writeBytes(t, "x.mp4", []byte("not really a video"))
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsValid {
		t.Error("missing ffprobe should yield IsValid=false")
	}
	foundErr := false
	for _, e := range res.Errors {
		if e == "ffprobe not found - cannot validate video file" {
			foundErr = true
		}
	}
	if !foundErr {
		t.Errorf("expected ffprobe-not-found error, got %v", res.Errors)
	}
}

func TestVideoValidator_RealMP4(t *testing.T) {
	if !haveBinary("ffmpeg") || !haveBinary("ffprobe") {
		t.Skip("ffmpeg/ffprobe not installed — SKIP-OK: video validation needs a real codec to produce a fixture; no equivalent without one")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "clip.mp4")
	makeMP4(t, p)

	v := NewVideoValidator("")
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsValid {
		t.Fatalf("valid mp4 marked invalid: %v", res.Errors)
	}
	// Probe must have parsed the video stream dimensions.
	if res.Metadata["width"] != "320" {
		t.Errorf("width = %v, want 320", res.Metadata["width"])
	}
	if res.Metadata["height"] != "240" {
		t.Errorf("height = %v, want 240", res.Metadata["height"])
	}
	if res.Metadata["codec"] != "h264" {
		t.Errorf("codec = %v, want h264", res.Metadata["codec"])
	}
	dur, ok := res.Metadata["duration_seconds"].(float64)
	if !ok || dur <= 0 {
		t.Errorf("duration_seconds = %v, want > 0", res.Metadata["duration_seconds"])
	}
	if res.Metadata["size_bytes"].(int64) <= 0 {
		t.Error("size_bytes should be > 0 for a real mp4")
	}
}

func TestVideoValidator_EmptyFile(t *testing.T) {
	if !haveBinary("ffprobe") {
		t.Skip("ffprobe not installed — SKIP-OK: empty-file path runs after the ffprobe-availability gate")
	}
	v := NewVideoValidator("")
	p := writeBytes(t, "empty.mp4", []byte{})
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsValid {
		t.Error("empty video should be IsValid=false")
	}
	foundErr := false
	for _, e := range res.Errors {
		if e == "video file is empty (0 bytes)" {
			foundErr = true
		}
	}
	if !foundErr {
		t.Errorf("expected empty-video error, got %v", res.Errors)
	}
}

func TestExtractJSONString(t *testing.T) {
	tests := []struct {
		name string
		data string
		key  string
		want string
	}{
		{"quoted string", `{"codec_name": "h264", "x": 1}`, "codec_name", "h264"},
		{"unquoted number", `{"width": 320, "y": 2}`, "width", "320"},
		{"number at end of object", `{"height": 240}`, "height", "240"},
		{"missing key", `{"a": "b"}`, "codec_name", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSONString(tt.data, tt.key)
			if got != tt.want {
				t.Errorf("extractJSONString(%q,%q) = %q, want %q", tt.data, tt.key, got, tt.want)
			}
		})
	}
}

func TestValidateVideoDirectory(t *testing.T) {
	if !haveBinary("ffmpeg") || !haveBinary("ffprobe") {
		t.Skip("ffmpeg/ffprobe not installed — SKIP-OK: directory video validation needs a real fixture")
	}
	dir := t.TempDir()
	makeMP4(t, filepath.Join(dir, "a.mp4"))
	// A non-video file that must be skipped.
	if err := writeIntoDir(t, dir, "notes.txt", "ignore"); err != nil {
		t.Fatal(err)
	}

	results, err := ValidateVideoDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 video result, got %d", len(results))
	}
	if results[0].AssetType != AssetTypeVideo {
		t.Errorf("type = %s, want video", results[0].AssetType)
	}
	if !results[0].IsValid {
		t.Errorf("real mp4 should be valid: %v", results[0].Errors)
	}
}
