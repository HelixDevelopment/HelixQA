//go:build vision
// +build vision

package core

import (
	"image"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig_Defaults(t *testing.T) {
	c := DefaultConfig()
	if c == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if !c.OpenCV.Enabled {
		t.Error("OpenCV should be enabled by default")
	}
	if c.OpenCV.FeatureDetection.Algorithm != "orb" {
		t.Errorf("default detector = %q, want orb", c.OpenCV.FeatureDetection.Algorithm)
	}
	if c.OpenCV.FeatureDetection.MaxFeatures != 500 {
		t.Errorf("default MaxFeatures = %d, want 500", c.OpenCV.FeatureDetection.MaxFeatures)
	}
	if c.OCR.Primary != "tesseract" {
		t.Errorf("default primary OCR = %q, want tesseract", c.OCR.Primary)
	}
	if c.Regression.SimilarityThreshold != 0.95 {
		t.Errorf("default similarity threshold = %f, want 0.95", c.Regression.SimilarityThreshold)
	}
	// A few sub-configs that must be wired with non-zero values.
	if c.Performance.MaxWorkers != 4 {
		t.Errorf("default MaxWorkers = %d, want 4", c.Performance.MaxWorkers)
	}
	if c.Models.OmniParser.Timeout == 0 {
		t.Error("OmniParser timeout should be a non-zero default")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{"defaults valid", func(c *Config) {}, false},
		{
			"invalid primary OCR",
			func(c *Config) { c.OCR.Primary = "bogus" },
			true,
		},
		{
			"invalid fallback OCR",
			func(c *Config) { c.OCR.Fallback = "bogus" },
			true,
		},
		{
			"empty fallback OCR allowed",
			func(c *Config) { c.OCR.Fallback = "" },
			false,
		},
		{
			"invalid detector algorithm",
			func(c *Config) { c.OpenCV.FeatureDetection.Algorithm = "surf" },
			true,
		},
		{
			"valid akaze detector",
			func(c *Config) { c.OpenCV.FeatureDetection.Algorithm = "akaze" },
			false,
		},
		{
			"similarity threshold too high",
			func(c *Config) { c.Regression.SimilarityThreshold = 1.5 },
			true,
		},
		{
			"similarity threshold negative",
			func(c *Config) { c.Regression.SimilarityThreshold = -0.1 },
			true,
		},
		{
			"opencv disabled short-circuits all other validation",
			func(c *Config) {
				c.OpenCV.Enabled = false
				c.OCR.Primary = "bogus" // would normally fail, but ignored
			},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := DefaultConfig()
			tt.mutate(c)
			err := c.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSaveLoadConfig_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.json")

	orig := DefaultConfig()
	orig.OCR.Primary = "paddle"
	orig.OpenCV.FeatureDetection.MaxFeatures = 1234
	orig.Regression.SimilarityThreshold = 0.88

	if err := SaveConfig(orig, p); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	loaded, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if loaded.OCR.Primary != "paddle" {
		t.Errorf("round-trip OCR.Primary = %q, want paddle", loaded.OCR.Primary)
	}
	if loaded.OpenCV.FeatureDetection.MaxFeatures != 1234 {
		t.Errorf("round-trip MaxFeatures = %d, want 1234", loaded.OpenCV.FeatureDetection.MaxFeatures)
	}
	if loaded.Regression.SimilarityThreshold != 0.88 {
		t.Errorf("round-trip threshold = %f, want 0.88", loaded.Regression.SimilarityThreshold)
	}
}

func TestLoadConfig_YAML(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "config.yaml")
	yamlData := "" +
		"opencv:\n" +
		"  enabled: true\n" +
		"  feature_detection:\n" +
		"    algorithm: akaze\n" +
		"    max_features: 999\n" +
		"ocr:\n" +
		"  primary: rapid\n"
	if err := os.WriteFile(p, []byte(yamlData), 0o644); err != nil {
		t.Fatal(err)
	}

	c, err := LoadConfig(p)
	if err != nil {
		t.Fatalf("LoadConfig(yaml): %v", err)
	}
	if c.OpenCV.FeatureDetection.Algorithm != "akaze" {
		t.Errorf("yaml algorithm = %q, want akaze", c.OpenCV.FeatureDetection.Algorithm)
	}
	if c.OpenCV.FeatureDetection.MaxFeatures != 999 {
		t.Errorf("yaml max_features = %d, want 999", c.OpenCV.FeatureDetection.MaxFeatures)
	}
	if c.OCR.Primary != "rapid" {
		t.Errorf("yaml primary = %q, want rapid", c.OCR.Primary)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

func TestRectangle_Center(t *testing.T) {
	r := Rectangle{Rectangle: image.Rect(10, 20, 30, 60)}
	c := r.Center()
	// Center = Min + (Dx/2, Dy/2) = (10+10, 20+20) = (20, 40)
	if c.X != 20 || c.Y != 40 {
		t.Errorf("Center = (%d,%d), want (20,40)", c.X, c.Y)
	}
}
