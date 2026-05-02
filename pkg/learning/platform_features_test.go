// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package learning

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlatformFeatureDetector_findAndroidTVDir_NotFound(t *testing.T) {
	// Create temp dir without Android TV
	tempDir := t.TempDir()
	detector := NewPlatformFeatureDetector(tempDir)

	result := detector.findAndroidTVDir()
	if result != "" {
		t.Errorf("expected empty string for no Android TV dir, got: %s", result)
	}
}

func TestPlatformFeatureDetector_findAndroidTVDir_WithCatalogizer(t *testing.T) {
	tempDir := t.TempDir()
	androidTVDir := filepath.Join(tempDir, "catalogizer-androidtv")
	os.MkdirAll(androidTVDir, 0755)

	detector := NewPlatformFeatureDetector(tempDir)
	result := detector.findAndroidTVDir()

	if result != androidTVDir {
		t.Errorf("expected %s, got: %s", androidTVDir, result)
	}
}

func TestPlatformFeatureDetector_findAndroidTVDir_WithTvProvider(t *testing.T) {
	tempDir := t.TempDir()
	// Create a directory with "tv" in name
	tvDir := filepath.Join(tempDir, "myapp-tv")
	os.MkdirAll(tvDir, 0755)
	// Create a Kotlin file with TvContractCompat reference
	testFile := filepath.Join(tvDir, "Test.kt")
	content := `package test
import androidx.tvprovider.media.tv.TvContractCompat
class Test {}`
	os.WriteFile(testFile, []byte(content), 0644)

	detector := NewPlatformFeatureDetector(tempDir)
	result := detector.findAndroidTVDir()

	if result == "" {
		t.Error("expected to find TV directory")
	}
}

func TestPlatformFeatureDetector_DetectAndroidTVChannels_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	detector := NewPlatformFeatureDetector(tempDir)

	feature := detector.DetectAndroidTVChannels()
	if feature != nil {
		t.Error("expected nil for no Android TV channels")
	}
}

func TestPlatformFeatureDetector_DetectAndroidTVChannels_Found(t *testing.T) {
	tempDir := t.TempDir()
	androidTVDir := filepath.Join(tempDir, "catalogizer-androidtv", "src", "main", "java", "test")
	os.MkdirAll(androidTVDir, 0755)

	// Create a Kotlin file with TvContractCompat
	testFile := filepath.Join(androidTVDir, "TvChannelRepository.kt")
	content := `package test
import androidx.tvprovider.media.tv.TvContractCompat
import androidx.tvprovider.media.tv.TvContractCompat.WatchNextPrograms
class TvChannelRepository {
    fun test() {
        val uri = TvContractCompat.Channels.CONTENT_URI
    }
}`
	os.WriteFile(testFile, []byte(content), 0644)

	detector := NewPlatformFeatureDetector(tempDir)
	feature := detector.DetectAndroidTVChannels()

	if feature == nil {
		t.Fatal("expected feature to be detected")
	}

	if feature.Name != "androidtv_channels" {
		t.Errorf("expected name 'androidtv_channels', got: %s", feature.Name)
	}

	if feature.Platform != "androidtv" {
		t.Errorf("expected platform 'androidtv', got: %s", feature.Platform)
	}

	if len(feature.SourceFiles) == 0 {
		t.Error("expected source files to be detected")
	}

	// Check metadata
	if scheme, ok := feature.Metadata["uri_scheme"]; !ok || scheme != "app" {
		t.Errorf("expected default uri_scheme='app', got: %v", scheme)
	}
}

func TestPlatformFeatureDetector_DetectAndroidTVChannels_WithURI(t *testing.T) {
	tempDir := t.TempDir()
	androidTVDir := filepath.Join(tempDir, "catalogizer-androidtv", "src")
	os.MkdirAll(androidTVDir, 0755)

	// Create file with URI scheme
	testFile := filepath.Join(androidTVDir, "DeepLink.kt")
	content := `package test
import androidx.tvprovider.media.tv.TvContractCompat
class DeepLink {
    val uri = "catalogizer://media/123?type=movie"
}`
	os.WriteFile(testFile, []byte(content), 0644)

	detector := NewPlatformFeatureDetector(tempDir)
	feature := detector.DetectAndroidTVChannels()

	if feature == nil {
		t.Fatal("expected feature to be detected")
	}

	if scheme, ok := feature.Metadata["uri_scheme"]; !ok || scheme != "catalogizer" {
		t.Errorf("expected uri_scheme='catalogizer', got: %v", scheme)
	}
}

func TestPlatformFeatureDetector_DetectAndroidTVChannels_WithChannelName(t *testing.T) {
	tempDir := t.TempDir()
	androidTVDir := filepath.Join(tempDir, "catalogizer-androidtv", "src")
	os.MkdirAll(androidTVDir, 0755)

	// Create file with channel name - regex looks for COLUMN_DISPLAY_NAME.*"value"
	// or DEFAULT_CHANNEL_DISPLAY_NAME.*"value"
	testFile := filepath.Join(androidTVDir, "ChannelRepository.kt")
	content := `package test
import androidx.tvprovider.media.tv.TvContractCompat
class ChannelRepository {
    fun createChannel() {
        val values = ContentValues().apply {
            put(TvContractCompat.Channels.COLUMN_DISPLAY_NAME, "My App Picks")
        }
    }
    companion object {
        const val DEFAULT_CHANNEL_DISPLAY_NAME = "My App Picks"
    }
}`
	os.WriteFile(testFile, []byte(content), 0644)

	detector := NewPlatformFeatureDetector(tempDir)
	feature := detector.DetectAndroidTVChannels()

	if feature == nil {
		t.Fatal("expected feature to be detected")
	}

	if name, ok := feature.Metadata["default_channel"]; !ok {
		t.Error("expected default_channel in metadata")
	} else if name != "My App Picks" {
		t.Errorf("expected 'My App Picks', got: %v", name)
	}
}

func TestPlatformFeatureDetector_DetectAllPlatformFeatures(t *testing.T) {
	tempDir := t.TempDir()
	androidTVDir := filepath.Join(tempDir, "catalogizer-androidtv", "src")
	os.MkdirAll(androidTVDir, 0755)

	// Create file with TvContractCompat
	testFile := filepath.Join(androidTVDir, "Test.kt")
	content := `import androidx.tvprovider.media.tv.TvContractCompat`
	os.WriteFile(testFile, []byte(content), 0644)

	detector := NewPlatformFeatureDetector(tempDir)
	features := detector.DetectAllPlatformFeatures()

	if len(features) == 0 {
		t.Error("expected at least one feature to be detected")
	}

	foundChannels := false
	for _, f := range features {
		if f.Name == "androidtv_channels" {
			foundChannels = true
			break
		}
	}

	if !foundChannels {
		t.Error("expected androidtv_channels feature to be detected")
	}
}

func TestBoolToString(t *testing.T) {
	if boolToString(true) != "true" {
		t.Error("expected 'true' for true")
	}
	if boolToString(false) != "false" {
		t.Error("expected 'false' for false")
	}
}

func BenchmarkPlatformFeatureDetector_DetectAndroidTVChannels(b *testing.B) {
	tempDir := b.TempDir()
	androidTVDir := filepath.Join(tempDir, "catalogizer-androidtv", "src")
	os.MkdirAll(androidTVDir, 0755)

	testFile := filepath.Join(androidTVDir, "Test.kt")
	content := `import androidx.tvprovider.media.tv.TvContractCompat`
	os.WriteFile(testFile, []byte(content), 0644)

	detector := NewPlatformFeatureDetector(tempDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.DetectAndroidTVChannels()
	}
}
