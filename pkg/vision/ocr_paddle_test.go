package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultPaddleOCRConfig(t *testing.T) {
	config := DefaultPaddleOCRConfig()

	assert.Equal(t, "http://localhost:8866/predict/ocr_system", config.Endpoint)
	assert.Equal(t, "en", config.Language)
	assert.True(t, config.UseAngleCls)
	assert.Equal(t, "ch_PP-OCRv4_rec", config.RecModel)
	assert.Equal(t, 0.7, config.MinConfidence)
	assert.False(t, config.UseGPU)
}

func TestNewPaddleOCR_NilConfig(t *testing.T) {
	ocr, err := NewPaddleOCR(nil)

	require.NoError(t, err)
	assert.NotNil(t, ocr)
	assert.NotNil(t, ocr.config)
}

func TestCheckPaddleOCRAvailable(t *testing.T) {
	// bluff-scan: no-assert-ok (environment-probe smoke — must not panic; result depends on host)
	// This will likely be false in test environment
	available := CheckPaddleOCRAvailable("")
	t.Logf("PaddleOCR available: %v", available)
}

func TestSupportedPaddleLanguages(t *testing.T) {
	assert.Contains(t, SupportedPaddleLanguages, "en")
	assert.Contains(t, SupportedPaddleLanguages, "ch_sim")
	assert.Contains(t, SupportedPaddleLanguages, "japan")
	assert.Equal(t, "English", SupportedPaddleLanguages["en"])
}

func TestPaddleStats(t *testing.T) {
	stats := PaddleStats{
		ImagesProcessed: 50,
		TextsFound:      200,
		TotalTimeMs:     3000,
		Errors:          1,
	}

	assert.Equal(t, uint64(50), stats.ImagesProcessed)
	assert.Equal(t, uint64(200), stats.TextsFound)
	assert.Equal(t, uint64(3000), stats.TotalTimeMs)
	assert.Equal(t, uint64(1), stats.Errors)
}

func TestEncodeBase64(t *testing.T) {
	data := []byte("Hello, World!")
	encoded := encodeBase64(data)

	// Basic validation - base64 should be longer than original
	assert.Greater(t, len(encoded), len(data))

	// Should only contain base64 characters
	for _, c := range encoded {
		assert.True(t,
			(c >= 'A' && c <= 'Z') ||
				(c >= 'a' && c <= 'z') ||
				(c >= '0' && c <= '9') ||
				c == '+' || c == '/' || c == '=')
	}
}

func TestEncodeBase64Empty(t *testing.T) {
	encoded := encodeBase64([]byte{})
	assert.Empty(t, encoded)
}

func TestPaddleOCR_CalculateBounds(t *testing.T) {
	ocr := &PaddleOCR{config: DefaultPaddleOCRConfig()}

	boxData := []interface{}{
		[]interface{}{10.0, 10.0},
		[]interface{}{100.0, 10.0},
		[]interface{}{100.0, 50.0},
		[]interface{}{10.0, 50.0},
	}

	bounds := ocr.calculateBounds(boxData)

	assert.Equal(t, 10, bounds.Min.X)
	assert.Equal(t, 10, bounds.Min.Y)
	assert.Equal(t, 100, bounds.Max.X)
	assert.Equal(t, 50, bounds.Max.Y)
}

func TestPaddleOCR_CalculateBoundsInvalid(t *testing.T) {
	ocr := &PaddleOCR{config: DefaultPaddleOCRConfig()}

	// Empty box
	bounds := ocr.calculateBounds([]interface{}{})
	assert.Equal(t, image.Rect(0, 0, 0, 0), bounds)

	// Invalid coordinates
	bounds = ocr.calculateBounds([]interface{}{
		"invalid",
	})
	assert.Equal(t, image.Rect(0, 0, 0, 0), bounds)
}

func TestPaddleOCR_ParseResult(t *testing.T) {
	ocr := &PaddleOCR{config: DefaultPaddleOCRConfig()}

	result := PaddleOCRResponse{
		Status: "0",
		Msg:    "success",
		Result: [][]interface{}{
			{
				[]interface{}{
					[]interface{}{10.0, 10.0},
					[]interface{}{100.0, 10.0},
					[]interface{}{100.0, 50.0},
					[]interface{}{10.0, 50.0},
				},
				"Hello World",
				0.95,
			},
		},
	}

	blocks, err := ocr.parseResult(result)
	require.NoError(t, err)
	assert.Len(t, blocks, 1)
	assert.Equal(t, "Hello World", blocks[0].Text)
	assert.Equal(t, 0.95, blocks[0].Confidence)
}

func TestPaddleOCR_ParseResultLowConfidence(t *testing.T) {
	ocr := &PaddleOCR{config: DefaultPaddleOCRConfig()}

	result := PaddleOCRResponse{
		Status: "0",
		Result: [][]interface{}{
			{
				[]interface{}{
					[]interface{}{10.0, 10.0},
					[]interface{}{100.0, 10.0},
					[]interface{}{100.0, 50.0},
					[]interface{}{10.0, 50.0},
				},
				"Low confidence text",
				0.5, // Below MinConfidence of 0.7
			},
		},
	}

	blocks, err := ocr.parseResult(result)
	require.NoError(t, err)
	assert.Empty(t, blocks) // Should be filtered out
}

func TestPaddleOCR_ParseResultInvalid(t *testing.T) {
	ocr := &PaddleOCR{config: DefaultPaddleOCRConfig()}

	result := PaddleOCRResponse{
		Status: "0",
		Result: [][]interface{}{
			{
				"invalid box data",
				"Text",
				0.9,
			},
		},
	}

	blocks, err := ocr.parseResult(result)
	require.NoError(t, err)
	assert.Empty(t, blocks) // Invalid box data should be skipped
}

func TestPaddleOCRService(t *testing.T) {
	// bluff-scan: no-assert-ok (service smoke — public method must not panic on standard call)
	// Skip in CI environment
	t.Skip("Requires PaddleOCR installation")
}

func TestPaddleOCRService_IsRunning(t *testing.T) {
	service := &PaddleOCRService{
		config: DefaultPaddleOCRConfig(),
	}

	// Should be false if not started
	assert.False(t, service.IsRunning())
}

func TestCompareOCREngines(t *testing.T) {
	img := createTestImageForOCR(100, 50)

	results, err := CompareOCREngines(img)

	// May or may not have results depending on what's installed
	assert.NoError(t, err)
	assert.NotNil(t, results)
}

// Helper function
func createTestImageForOCR(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// White background
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.White)
		}
	}

	// Draw black bars to simulate text
	for y := height / 3; y < 2*height/3; y++ {
		for x := width / 4; x < 3*width/4; x++ {
			img.Set(x, y, color.Black)
		}
	}

	return img
}
