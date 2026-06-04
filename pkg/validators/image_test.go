package validators

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// makeImage builds an opaque w x h image with a simple gradient so encoders
// produce non-trivial output.
func makeImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	return img
}

func writePNG(t *testing.T, name string, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, makeImage(w, h)); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return writeBytes(t, name, buf.Bytes())
}

func writeJPEG(t *testing.T, name string, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, makeImage(w, h), &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return writeBytes(t, name, buf.Bytes())
}

func writeGIF(t *testing.T, name string, w, h int) string {
	t.Helper()
	var buf bytes.Buffer
	if err := gif.Encode(&buf, makeImage(w, h), nil); err != nil {
		t.Fatalf("encode gif: %v", err)
	}
	return writeBytes(t, name, buf.Bytes())
}

func TestImageValidator_Supports(t *testing.T) {
	v := NewImageValidator()
	for _, p := range []string{"a.png", "a.jpg", "a.jpeg", "a.gif", "a.webp"} {
		if !v.Supports(p) {
			t.Errorf("expected Supports(%q) = true", p)
		}
	}
	if v.Supports("a.txt") {
		t.Error("expected Supports(.txt) = false")
	}
}

func TestImageValidator_PNG(t *testing.T) {
	v := NewImageValidator()
	p := writePNG(t, "shot.png", 1920, 1080)
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsValid {
		t.Fatalf("valid PNG marked invalid: %v", res.Errors)
	}
	if res.Metadata["format"] != "png" {
		t.Errorf("format = %v, want png", res.Metadata["format"])
	}
	if res.Metadata["width"].(int) != 1920 || res.Metadata["height"].(int) != 1080 {
		t.Errorf("dims = %vx%v, want 1920x1080", res.Metadata["width"], res.Metadata["height"])
	}
	// 1920x1080 is a known screenshot size, full HD, HD.
	if res.Metadata["is_screenshot_size"] != true {
		t.Error("expected is_screenshot_size=true for 1920x1080")
	}
	if res.Metadata["is_full_hd"] != true {
		t.Error("expected is_full_hd=true for 1920x1080")
	}
	if res.Metadata["is_4k"] != false {
		t.Error("expected is_4k=false for 1920x1080")
	}
	if res.Metadata["has_alpha"] != true {
		t.Error("PNG should report has_alpha=true")
	}
	ar := res.Metadata["aspect_ratio"].(float64)
	if ar < 1.7 || ar > 1.8 {
		t.Errorf("aspect_ratio = %f, want ~1.777", ar)
	}
}

func TestImageValidator_JPEG(t *testing.T) {
	v := NewImageValidator()
	p := writeJPEG(t, "photo.jpg", 640, 480)
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsValid {
		t.Fatalf("valid JPEG marked invalid: %v", res.Errors)
	}
	if res.Metadata["format"] != "jpeg" {
		t.Errorf("format = %v, want jpeg", res.Metadata["format"])
	}
	if res.Metadata["width"].(int) != 640 || res.Metadata["height"].(int) != 480 {
		t.Errorf("dims = %vx%v, want 640x480", res.Metadata["width"], res.Metadata["height"])
	}
	if res.Metadata["is_screenshot_size"] != false {
		t.Error("640x480 is not a known screenshot size")
	}
	if _, ok := res.Metadata["color_model"]; !ok {
		t.Error("expected color_model metadata for jpeg")
	}
}

func TestImageValidator_GIF(t *testing.T) {
	v := NewImageValidator()
	p := writeGIF(t, "anim.gif", 100, 100)
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsValid {
		t.Fatalf("valid GIF marked invalid: %v", res.Errors)
	}
	if res.Metadata["format"] != "gif" {
		t.Errorf("format = %v, want gif", res.Metadata["format"])
	}
	if res.Metadata["is_animated"] != false {
		t.Error("single-frame gif should report is_animated=false")
	}
}

func TestImageValidator_EmptyFile(t *testing.T) {
	v := NewImageValidator()
	p := writeBytes(t, "empty.png", []byte{})
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsValid {
		t.Error("empty image should be IsValid=false")
	}
	if len(res.Errors) == 0 {
		t.Error("empty image should report an error")
	}
}

func TestImageValidator_CorruptData(t *testing.T) {
	v := NewImageValidator()
	// Non-empty but undecodable bytes with an image extension.
	p := writeBytes(t, "broken.png", []byte("this is not a real PNG payload at all"))
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	if res.IsValid {
		t.Error("undecodable image should be IsValid=false")
	}
	if len(res.Errors) == 0 {
		t.Error("undecodable image should report a decode error")
	}
}

func TestImageValidator_MissingFile(t *testing.T) {
	v := NewImageValidator()
	res, err := v.Validate(filepath.Join(t.TempDir(), "nope.png"))
	if err != nil {
		t.Fatal(err)
	}
	if res.IsValid {
		t.Error("missing image should be IsValid=false")
	}
}

func TestIsScreenshotSize(t *testing.T) {
	cases := []struct {
		w, h int
		want bool
	}{
		{1920, 1080, true},
		{1080, 1920, true}, // rotated orientation accepted
		{1280, 720, true},
		{3840, 2160, true},
		{2560, 1440, true},
		{640, 480, false},
		{1000, 1000, false},
	}
	for _, c := range cases {
		if got := isScreenshotSize(c.w, c.h); got != c.want {
			t.Errorf("isScreenshotSize(%d,%d) = %v, want %v", c.w, c.h, got, c.want)
		}
	}
}

func TestValidateImageDirectory(t *testing.T) {
	dir := t.TempDir()
	// Two real images + one non-image that must be ignored.
	var buf bytes.Buffer
	_ = png.Encode(&buf, makeImage(64, 64))
	if err := os.WriteFile(filepath.Join(dir, "a.png"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	buf.Reset()
	_ = jpeg.Encode(&buf, makeImage(64, 64), nil)
	if err := os.WriteFile(filepath.Join(dir, "b.jpg"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c.txt"), []byte("ignore me"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A subdirectory must be skipped.
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	results, err := ValidateImageDirectory(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 image results, got %d", len(results))
	}
	for _, r := range results {
		if r.AssetType != AssetTypeImage {
			t.Errorf("result %s has type %s, want image", r.AssetPath, r.AssetType)
		}
		if !r.IsValid {
			t.Errorf("result %s should be valid: %v", r.AssetPath, r.Errors)
		}
	}
}

func TestScreenshotValidator_DimensionMismatch(t *testing.T) {
	v := NewScreenshotValidator(1920, 1080)
	// Encode an image at the WRONG size to trigger the mismatch warning.
	p := writePNG(t, "wrong.png", 800, 600)
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range res.Warnings {
		if bytes.Contains([]byte(w), []byte("don't match expected")) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected dimension-mismatch warning, got %v", res.Warnings)
	}
}

func TestScreenshotValidator_SmallFileWarning(t *testing.T) {
	v := NewScreenshotValidator(0, 0) // disable dimension check
	// A 1x1 PNG is well under 10KB -> "unusually small" warning.
	p := writePNG(t, "tiny.png", 1, 1)
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, w := range res.Warnings {
		if bytes.Contains([]byte(w), []byte("unusually small")) {
			found = true
		}
	}
	if !found {
		t.Errorf("expected unusually-small warning, got %v", res.Warnings)
	}
}

func TestScreenshotValidator_DimensionMatchNoWarning(t *testing.T) {
	v := NewScreenshotValidator(640, 480)
	p := writePNG(t, "match.png", 640, 480)
	res, err := v.Validate(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range res.Warnings {
		if bytes.Contains([]byte(w), []byte("don't match expected")) {
			t.Errorf("unexpected dimension-mismatch warning for matching size: %v", res.Warnings)
		}
	}
}
