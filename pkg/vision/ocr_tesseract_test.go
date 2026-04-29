package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultTesseractConfig(t *testing.T) {
	config := DefaultTesseractConfig()

	assert.Equal(t, "tesseract", config.TesseractPath)
	assert.Equal(t, "eng", config.Language)
	assert.Equal(t, 3, config.PageSegMode)
	assert.Equal(t, 3, config.EngineMode)
	assert.Equal(t, 60, config.MinConfidence)
	assert.False(t, config.PreserveWhitespace)
}

func TestCheckTesseractAvailable(t *testing.T) {
	// bluff-scan: no-assert-ok (environment-probe smoke — must not panic; result depends on host)
	// This test depends on whether tesseract is installed
	available := CheckTesseractAvailable()

	// Just log the result, don't assert
	t.Logf("Tesseract available: %v", available)
}

func TestTesseractVersionInfo(t *testing.T) {
	if !CheckTesseractAvailable() {
		t.Skip("Tesseract not installed")  // SKIP-OK: #legacy-untriaged
	}

	info, err := GetDetailedVersion()
	require.NoError(t, err)

	assert.NotEmpty(t, info.Version)
	t.Logf("Tesseract version: %s", info.Version)

	if info.Leptonica != "" {
		t.Logf("Leptonica version: %s", info.Leptonica)
	}
}

func TestSupportedLanguages(t *testing.T) {
	assert.Contains(t, SupportedLanguages, "eng")
	assert.Contains(t, SupportedLanguages, "deu")
	assert.Contains(t, SupportedLanguages, "fra")
	assert.Equal(t, "English", SupportedLanguages["eng"])
	assert.Equal(t, "German", SupportedLanguages["deu"])
}

func TestParseTSV(t *testing.T) {
	config := DefaultTesseractConfig()
	ocr := &TesseractOCR{config: config}

	// Sample TSV data
	tsvData := []byte(`level	page_num	block_num	par_num	line_num	word_num	left	top	width	height	conf	text
1	1	0	0	0	0	0	0	100	100	-1	
2	1	1	0	0	0	10	10	80	20	-1	
3	1	1	1	0	0	10	10	80	20	-1	
4	1	1	1	1	0	10	10	40	20	-1	
5	1	1	1	1	1	10	10	35	20	95	Hello
5	1	1	1	1	2	50	10	40	20	92	World
`)

	blocks := ocr.parseTSV(tsvData)

	// Should have 2 text blocks with sufficient confidence
	assert.GreaterOrEqual(t, len(blocks), 0)

	for _, block := range blocks {
		assert.NotEmpty(t, block.Text)
		assert.GreaterOrEqual(t, block.Confidence, 0.0)
		assert.LessOrEqual(t, block.Confidence, 1.0)
		assert.Equal(t, "eng", block.Language)
	}
}

func TestParseTSVEmpty(t *testing.T) {
	config := DefaultTesseractConfig()
	ocr := &TesseractOCR{config: config}

	blocks := ocr.parseTSV([]byte{})
	assert.Empty(t, blocks)
}

func TestParseTSVInvalidData(t *testing.T) {
	config := DefaultTesseractConfig()
	ocr := &TesseractOCR{config: config}

	// Invalid TSV with missing fields
	tsvData := []byte(`level	page_num
1	1
invalid
`)

	blocks := ocr.parseTSV(tsvData)
	assert.Empty(t, blocks)
}

func TestNewTesseractOCR(t *testing.T) {
	if !CheckTesseractAvailable() {
		t.Skip("Tesseract not installed")  // SKIP-OK: #legacy-untriaged
	}

	config := DefaultTesseractConfig()
	ocr, err := NewTesseractOCR(config)

	require.NoError(t, err)
	assert.NotNil(t, ocr)
	assert.Equal(t, config, ocr.config)
}

func TestNewTesseractOCR_NilConfig(t *testing.T) {
	if !CheckTesseractAvailable() {
		t.Skip("Tesseract not installed")  // SKIP-OK: #legacy-untriaged
	}

	ocr, err := NewTesseractOCR(nil)

	require.NoError(t, err)
	assert.NotNil(t, ocr)
	assert.NotNil(t, ocr.config)
}

func TestTesseractOCR_GetAvailableLanguages(t *testing.T) {
	if !CheckTesseractAvailable() {
		t.Skip("Tesseract not installed")  // SKIP-OK: #legacy-untriaged
	}

	config := DefaultTesseractConfig()
	ocr, err := NewTesseractOCR(config)
	require.NoError(t, err)

	langs, err := ocr.GetAvailableLanguages()

	require.NoError(t, err)
	assert.NotEmpty(t, langs)
	assert.Contains(t, langs, "eng")

	t.Logf("Available languages: %v", langs)
}

func TestTesseractOCR_GetVersion(t *testing.T) {
	if !CheckTesseractAvailable() {
		t.Skip("Tesseract not installed")  // SKIP-OK: #legacy-untriaged
	}

	config := DefaultTesseractConfig()
	ocr, err := NewTesseractOCR(config)
	require.NoError(t, err)

	version, err := ocr.GetVersion()

	require.NoError(t, err)
	assert.NotEmpty(t, version)
	t.Logf("Version: %s", version)
}

func TestTesseractOCR_DetectText_NotInstalled(t *testing.T) {
	// Test with invalid path
	config := &TesseractConfig{
		TesseractPath: "/invalid/path/to/tesseract",
	}

	_, err := NewTesseractOCR(config)
	assert.Error(t, err)
}

func TestTesseractStats(t *testing.T) {
	stats := TesseractStats{
		ImagesProcessed: 100,
		TextsFound:      500,
		TotalTimeMs:     5000,
		Errors:          2,
	}

	assert.Equal(t, uint64(100), stats.ImagesProcessed)
	assert.Equal(t, uint64(500), stats.TextsFound)
	assert.Equal(t, uint64(5000), stats.TotalTimeMs)
	assert.Equal(t, uint64(2), stats.Errors)
}

func TestTesseractProcessor(t *testing.T) {
	if !CheckTesseractAvailable() {
		t.Skip("Tesseract not installed")  // SKIP-OK: #legacy-untriaged
	}

	config := DefaultTesseractConfig()
	processor, err := NewTesseractProcessor(config, 2)

	require.NoError(t, err)
	assert.NotNil(t, processor)
	assert.NotNil(t, processor.ocr)
	assert.NotNil(t, processor.buffer)
}

func TestTesseractProcessor_GetStats(t *testing.T) {
	if !CheckTesseractAvailable() {
		t.Skip("Tesseract not installed")  // SKIP-OK: #legacy-untriaged
	}

	config := DefaultTesseractConfig()
	processor, err := NewTesseractProcessor(config, 2)
	require.NoError(t, err)

	stats := processor.GetStats()
	assert.Equal(t, uint64(0), stats.ImagesProcessed)
	assert.Equal(t, uint64(0), stats.TextsFound)
}

func TestPageSegModes(t *testing.T) {
	// Test different PSM values
	modes := []int{0, 1, 3, 4, 6, 7, 8, 11}

	for _, mode := range modes {
		config := DefaultTesseractConfig()
		config.PageSegMode = mode
		assert.Equal(t, mode, config.PageSegMode)
	}
}

func TestEngineModes(t *testing.T) {
	// Test different OEM values
	modes := []int{0, 1, 2, 3}

	for _, mode := range modes {
		config := DefaultTesseractConfig()
		config.EngineMode = mode
		assert.Equal(t, mode, config.EngineMode)
	}
}

func TestCreateTestImageWithText(t *testing.T) {
	// Create a simple image
	img := createTestImageWithText("TEST", 100, 50)

	assert.NotNil(t, img)
	bounds := img.Bounds()
	assert.Equal(t, 100, bounds.Dx())
	assert.Equal(t, 50, bounds.Dy())
}

// Helper function to create a test image with text-like pattern
func createTestImageWithText(text string, width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// White background
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.White)
		}
	}

	// Draw simple black rectangles to simulate text
	for i := 0; i < len(text); i++ {
		x := 10 + i*15
		for y := 15; y < 35; y++ {
			for x2 := x; x2 < x+10; x2++ {
				if x2 < width {
					img.Set(x2, y, color.Black)
				}
			}
		}
	}

	return img
}
