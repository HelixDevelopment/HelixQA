package gst

import (
	"image"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultExtractorConfig(t *testing.T) {
	config := DefaultExtractorConfig("rtsp://localhost:8554/test")

	assert.Equal(t, "rtsp://localhost:8554/test", config.SourceURL)
	assert.Equal(t, SourceRTSP, config.SourceType)
	assert.Equal(t, 1920, config.Width)
	assert.Equal(t, 1080, config.Height)
	assert.Equal(t, 30, config.FPS)
	assert.Equal(t, FormatRGB, config.Format)
	assert.Equal(t, 30, config.BufferSize)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 3, config.RetryAttempts)
}

func TestNewFrameExtractor(t *testing.T) {
	config := DefaultExtractorConfig("rtsp://localhost:8554/test")
	extractor := NewFrameExtractor(config)

	require.NotNil(t, extractor)
	assert.Equal(t, config, extractor.config)
	assert.NotNil(t, extractor.frameChan)
	assert.NotNil(t, extractor.errChan)
	assert.NotNil(t, extractor.ctx)
	assert.NotNil(t, extractor.cancel)
	assert.False(t, extractor.running)
}

func TestFrameExtractor_IsRunning(t *testing.T) {
	config := DefaultExtractorConfig("test")
	extractor := NewFrameExtractor(config)

	assert.False(t, extractor.IsRunning())

	// Note: We can't easily test the running state without
	// actually starting GStreamer which requires the binary
}

func TestBuildRTSPPipeline(t *testing.T) {
	config := ExtractorConfig{
		SourceURL:  "rtsp://localhost:8554/test",
		SourceType: SourceRTSP,
		Width:      1920,
		Height:     1080,
		FPS:        30,
		Format:     FormatRGB,
		BufferSize: 30,
	}
	extractor := NewFrameExtractor(config)

	pipeline := extractor.buildRTSPPipeline()

	assert.Contains(t, pipeline, "rtspsrc")
	assert.Contains(t, pipeline, "rtsp://localhost:8554/test")
	assert.Contains(t, pipeline, "decodebin")
	assert.Contains(t, pipeline, "videoconvert")
	assert.Contains(t, pipeline, "1920")
	assert.Contains(t, pipeline, "1080")
	assert.Contains(t, pipeline, "30/1")
	assert.Contains(t, pipeline, "appsink")
}

func TestBuildTestPipeline(t *testing.T) {
	config := ExtractorConfig{
		SourceType: SourceTest,
		Width:      1280,
		Height:     720,
		FPS:        60,
		Format:     FormatRGBA,
		BufferSize: 10,
	}
	extractor := NewFrameExtractor(config)

	pipeline := extractor.buildTestPipeline()

	assert.Contains(t, pipeline, "videotestsrc")
	assert.Contains(t, pipeline, "smpte")
	assert.Contains(t, pipeline, "1280")
	assert.Contains(t, pipeline, "720")
	assert.Contains(t, pipeline, "60/1")
	assert.Contains(t, pipeline, "RGBA")
}

func TestBuildFilePipeline(t *testing.T) {
	config := ExtractorConfig{
		SourceURL:  "/tmp/test.mp4",
		SourceType: SourceFile,
		Width:      1920,
		Height:     1080,
		FPS:        30,
		Format:     FormatRGB,
		BufferSize: 30,
	}
	extractor := NewFrameExtractor(config)

	pipeline := extractor.buildFilePipeline()

	assert.Contains(t, pipeline, "filesrc")
	assert.Contains(t, pipeline, "/tmp/test.mp4")
	assert.Contains(t, pipeline, "decodebin")
}

func TestBuildDevicePipeline(t *testing.T) {
	config := ExtractorConfig{
		SourceURL:  "/dev/video0",
		SourceType: SourceDevice,
		Width:      640,
		Height:     480,
		FPS:        30,
		Format:     FormatRGB,
		BufferSize: 30,
	}
	extractor := NewFrameExtractor(config)

	pipeline := extractor.buildDevicePipeline()

	assert.Contains(t, pipeline, "v4l2src")
	assert.Contains(t, pipeline, "/dev/video0")
}

func TestFrame_ToImage(t *testing.T) {
	tests := []struct {
		name    string
		frame   Frame
		wantErr bool
	}{
		{
			name: "RGB format",
			frame: Frame{
				Data:   make([]byte, 1920*1080*3),
				Width:  1920,
				Height: 1080,
				Format: FormatRGB,
			},
			wantErr: false,
		},
		{
			name: "RGBA format",
			frame: Frame{
				Data:   make([]byte, 1920*1080*4),
				Width:  1920,
				Height: 1080,
				Format: FormatRGBA,
			},
			wantErr: false,
		},
		{
			name: "GRAY8 format",
			frame: Frame{
				Data:   make([]byte, 1920*1080),
				Width:  1920,
				Height: 1080,
				Format: FormatGRAY8,
			},
			wantErr: false,
		},
		{
			name: "Unsupported format",
			frame: Frame{
				Data:   make([]byte, 100),
				Width:  10,
				Height: 10,
				Format: FormatNV12,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img, err := tt.frame.ToImage()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, img)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, img)
				assert.Equal(t, tt.frame.Width, img.Bounds().Max.X)
				assert.Equal(t, tt.frame.Height, img.Bounds().Max.Y)
			}
		})
	}
}

func TestRGBImage(t *testing.T) {
	// Create a simple 2x2 RGB image
	data := []byte{
		255, 0, 0, // Red
		0, 255, 0, // Green
		0, 0, 255, // Blue
		255, 255, 255, // White
	}

	img := &rgbImage{
		Pix:    data,
		Stride: 6,
		Rect:   image.Rect(0, 0, 2, 2),
	}

	// Test ColorModel
	assert.NotNil(t, img.ColorModel())

	// Test Bounds
	bounds := img.Bounds()
	assert.Equal(t, 0, bounds.Min.X)
	assert.Equal(t, 0, bounds.Min.Y)
	assert.Equal(t, 2, bounds.Max.X)
	assert.Equal(t, 2, bounds.Max.Y)

	// Test At - check specific pixels
	red := img.At(0, 0)
	r, g, b, a := red.RGBA()
	assert.Equal(t, uint32(0xFFFF), r)
	assert.Equal(t, uint32(0), g)
	assert.Equal(t, uint32(0), b)
	assert.Equal(t, uint32(0xFFFF), a)

	green := img.At(1, 0)
	r, g, b, a = green.RGBA()
	assert.Equal(t, uint32(0), r)
	assert.Equal(t, uint32(0xFFFF), g)
	assert.Equal(t, uint32(0), b)
	assert.Equal(t, uint32(0xFFFF), a)

	// Test out of bounds
	black := img.At(10, 10)
	r, g, b, a = black.RGBA()
	assert.Equal(t, uint32(0), r)
	assert.Equal(t, uint32(0), g)
	assert.Equal(t, uint32(0), b)
	assert.Equal(t, uint32(0xFFFF), a)
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		format   PixelFormat
		width    int
		height   int
		expected int
	}{
		{FormatRGB, 1920, 1080, 1920 * 1080 * 3},
		{FormatBGR, 1920, 1080, 1920 * 1080 * 3},
		{FormatRGBA, 1920, 1080, 1920 * 1080 * 4},
		{FormatBGRA, 1920, 1080, 1920 * 1080 * 4},
		{FormatGRAY8, 1920, 1080, 1920 * 1080},
		{FormatNV12, 1920, 1080, 1920 * 1080 * 3 / 2},
		{FormatI420, 1920, 1080, 1920 * 1080 * 3 / 2},
		{PixelFormat("unknown"), 1920, 1080, 1920 * 1080 * 3}, // Default
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			size := FormatSize(tt.width, tt.height, tt.format)
			assert.Equal(t, tt.expected, size)
		})
	}
}

func TestResolution(t *testing.T) {
	r := Resolution{Width: 1920, Height: 1080}
	assert.Equal(t, "1920x1080", r.String())

	r2, err := ParseResolution("1920x1080")
	require.NoError(t, err)
	assert.Equal(t, 1920, r2.Width)
	assert.Equal(t, 1080, r2.Height)

	// Invalid format
	_, err = ParseResolution("invalid")
	assert.Error(t, err)

	_, err = ParseResolution("1920")
	assert.Error(t, err)
}

func TestParseCaps(t *testing.T) {
	caps := "video/x-raw,format=RGB,width=1920,height=1080"
	result, err := ParseCaps(caps)

	require.NoError(t, err)
	// video/x-raw doesn't have a key=value format, so it's skipped
	assert.Equal(t, "RGB", result["format"])
	assert.Equal(t, "1920", result["width"])
	assert.Equal(t, "1080", result["height"])
}

func TestExtractionStats(t *testing.T) {
	stats := ExtractionStats{
		FramesExtracted: 100,
		FramesDropped:   5,
		BytesProcessed:  1000000,
		StartTime:       time.Now(),
		Errors:          0,
	}

	assert.Equal(t, uint64(100), stats.FramesExtracted)
	assert.Equal(t, uint64(5), stats.FramesDropped)
	assert.Equal(t, uint64(1000000), stats.BytesProcessed)
	assert.Equal(t, uint64(0), stats.Errors)
}

func TestSourceType(t *testing.T) {
	types := []SourceType{
		SourceRTSP,
		SourceWebRTC,
		SourceFile,
		SourceDevice,
		SourceTest,
	}

	for _, st := range types {
		assert.NotEmpty(t, st)
	}
}

func TestPixelFormat(t *testing.T) {
	formats := []PixelFormat{
		FormatRGB,
		FormatBGR,
		FormatRGBA,
		FormatBGRA,
		FormatGRAY8,
		FormatNV12,
		FormatI420,
	}

	for _, f := range formats {
		assert.NotEmpty(t, f)
	}
}

// Integration tests - skip if GStreamer not installed

func TestCheckGStreamer(t *testing.T) {
	err := CheckGStreamer()
	if err != nil {
		t.Skip("GStreamer not installed:", err)
	}
}

func TestGetGStreamerVersion(t *testing.T) {
	version, err := GetGStreamerVersion()
	if err != nil {
		t.Skip("GStreamer not installed:", err)
	}

	assert.NotEmpty(t, version)
	t.Logf("GStreamer version: %s", version)
}

func TestCheckElement(t *testing.T) {
	// Common elements that should be available
	commonElements := []string{
		"videotestsrc",
		"videoconvert",
		"videoscale",
		"appsink",
	}

	for _, elem := range commonElements {
		available := CheckElement(elem)
		t.Logf("Element %s: %v", elem, available)
		// Don't assert - just log, since elements may vary by installation
	}
}

func TestFrameExtractor_StartStop(t *testing.T) {
	// Skip if GStreamer not available
	if err := CheckGStreamer(); err != nil {
		t.Skip("GStreamer not installed")
	}

	config := DefaultExtractorConfig("test")
	config.SourceType = SourceTest
	config.FPS = 1 // Low FPS for testing

	extractor := NewFrameExtractor(config)

	// Start
	err := extractor.Start()
	require.NoError(t, err)

	// Should be running
	assert.True(t, extractor.IsRunning())

	// Stop
	err = extractor.Stop()
	require.NoError(t, err)

	// Should not be running
	assert.False(t, extractor.IsRunning())
}

func TestFrameExtractor_Stats(t *testing.T) {
	config := DefaultExtractorConfig("test")
	extractor := NewFrameExtractor(config)

	stats := extractor.GetStats()

	// Initial stats should be zero
	assert.Equal(t, uint64(0), stats.FramesExtracted)
	assert.Equal(t, uint64(0), stats.FramesDropped)
	assert.Equal(t, uint64(0), stats.BytesProcessed)
	assert.Equal(t, uint64(0), stats.Errors)
}
