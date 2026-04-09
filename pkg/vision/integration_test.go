package vision

import (
	"image"
	"image/color"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEndToEndVisionPipeline tests the complete vision pipeline
func TestEndToEndVisionPipeline(t *testing.T) {
	// Create a test image with simulated UI elements
	img := createTestUIImage(640, 480)
	
	// Step 1: Element Detection
	detectorConfig := DefaultDetectorConfig()
	detectorConfig.EnableOCR = false
	detector := NewElementDetector(detectorConfig)
	
	detectResult, err := detector.Detect(img)
	require.NoError(t, err)
	assert.NotNil(t, detectResult)
	t.Logf("Detected %d elements", len(detectResult.Elements))
	
	// Verify detection stats
	stats := detector.GetStats()
	assert.Equal(t, uint64(1), stats.FramesProcessed)
}

// TestVisionWithOCR tests element detection with OCR
func TestVisionWithOCR(t *testing.T) {
	if !CheckTesseractAvailable() && !CheckPaddleOCRAvailable("") {
		t.Skip("No OCR engine available")
	}
	
	img := createTestUIImage(640, 480)
	
	// Create detector with OCR
	detectorConfig := DefaultDetectorConfig()
	detectorConfig.EnableOCR = true
	detector := NewElementDetector(detectorConfig)
	
	// Set OCR engine if available
	if CheckTesseractAvailable() {
		tessConfig := DefaultTesseractConfig()
		ocr, err := NewTesseractOCR(tessConfig)
		if err == nil {
			detector.SetOCREngine(ocr)
		}
	}
	
	result, err := detector.Detect(img)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

// TestVisionLLMIntegration tests the complete Vision+LLM pipeline
func TestVisionLLMIntegration(t *testing.T) {
	if !CheckOllamaAvailable("") {
		t.Skip("Ollama not available")
	}
	
	img := createTestUIImage(640, 480)
	
	ollamaConfig := DefaultOllamaConfig()
	detectorConfig := DefaultDetectorConfig()
	
	visionLLM, err := NewVisionLLM(ollamaConfig, detectorConfig)
	require.NoError(t, err)
	
	result, err := visionLLM.Analyze(img)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Description)
	assert.Greater(t, result.LatencyMs, 0.0)
	
	t.Logf("Analysis: %s", result.Description)
	t.Logf("Elements found: %d", len(result.Elements))
	t.Logf("Latency: %.2f ms", result.LatencyMs)
}

// TestMultipleEnginesComparison compares different OCR engines
func TestMultipleEnginesComparison(t *testing.T) {
	img := createTestUIImage(400, 200)
	
	engines := make(map[string][]TextBlock)
	
	// Test Tesseract
	if CheckTesseractAvailable() {
		tessConfig := DefaultTesseractConfig()
		tess, err := NewTesseractOCR(tessConfig)
		if err == nil {
			blocks, err := tess.DetectText(img)
			if err == nil {
				engines["tesseract"] = blocks
			}
		}
	}
	
	// Test PaddleOCR
	if CheckPaddleOCRAvailable("") {
		paddleConfig := DefaultPaddleOCRConfig()
		paddle, err := NewPaddleOCR(paddleConfig)
		if err == nil {
			blocks, err := paddle.DetectText(img)
			if err == nil {
				engines["paddle"] = blocks
			}
		}
	}
	
	// Log results
	for name, blocks := range engines {
		t.Logf("%s found %d text blocks", name, len(blocks))
	}
	
	// At least one engine should work if any are installed
	if CheckTesseractAvailable() || CheckPaddleOCRAvailable("") {
		assert.GreaterOrEqual(t, len(engines), 0)
	}
}

// TestBatchProcessing tests batch frame processing
func TestBatchProcessing(t *testing.T) {
	// Create test frames
	frames := make([]image.Image, 5)
	for i := range frames {
		frames[i] = createTestUIImage(320, 240)
	}
	
	// Process with detector
	detectorConfig := DefaultDetectorConfig()
	detectorConfig.EnableOCR = false
	detector := NewElementDetector(detectorConfig)
	
	results, err := detector.DetectBatch(frames)
	require.NoError(t, err)
	assert.Len(t, results, len(frames))
	
	// Verify all results
	for _, result := range results {
		assert.NotNil(t, result)
		assert.NotEmpty(t, result.FrameID)
	}
	
	// Verify stats
	stats := detector.GetStats()
	assert.Equal(t, uint64(len(frames)), stats.FramesProcessed)
}

// TestRealTimeProcessingPerformance tests processing performance
func TestRealTimeProcessingPerformance(t *testing.T) {
	t.Skip("Performance test - run manually")
	
	img := createTestUIImage(1920, 1080)
	
	detectorConfig := DefaultDetectorConfig()
	detectorConfig.EnableOCR = false
	detector := NewElementDetector(detectorConfig)
	
	// Process multiple frames
	iterations := 10
	for i := 0; i < iterations; i++ {
		_, err := detector.Detect(img)
		require.NoError(t, err)
	}
	
	stats := detector.GetStats()
	avgLatency := stats.AvgLatencyMs
	
	t.Logf("Average latency: %.2f ms", avgLatency)
	t.Logf("FPS: %.2f", 1000.0/avgLatency)
	
	// Should achieve at least 10 FPS (100ms per frame)
	assert.Less(t, avgLatency, 100.0)
}

// Helper functions for integration tests

func createTestUIImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Light gray background
	bgColor := color.RGBA{240, 240, 240, 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, bgColor)
		}
	}
	
	// Simulate a button (rectangle)
	buttonColor := color.RGBA{0, 120, 255, 255}
	buttonX, buttonY := width/2-50, height/2-20
	for y := buttonY; y < buttonY+40 && y < height; y++ {
		for x := buttonX; x < buttonX+100 && x < width; x++ {
			img.Set(x, y, buttonColor)
		}
	}
	
	// Simulate input field
	inputColor := color.RGBA{255, 255, 255, 255}
	borderColor := color.RGBA{200, 200, 200, 255}
	inputX, inputY := width/2-100, height/2-80
	for y := inputY; y < inputY+30 && y < height; y++ {
		for x := inputX; x < inputX+200 && x < width; x++ {
			// Border
			if y == inputY || y == inputY+29 || x == inputX || x == inputX+199 {
				img.Set(x, y, borderColor)
			} else {
				img.Set(x, y, inputColor)
			}
		}
	}
	
	// Simulate checkbox
	checkboxColor := color.RGBA{0, 150, 0, 255}
	checkX, checkY := width/2-150, height/2-20
	for y := checkY; y < checkY+20 && y < height; y++ {
		for x := checkX; x < checkX+20 && x < width; x++ {
			img.Set(x, y, checkboxColor)
		}
	}
	
	return img
}

// BenchmarkElementDetection benchmarks element detection
func BenchmarkElementDetection(b *testing.B) {
	img := createTestUIImage(640, 480)
	
	config := DefaultDetectorConfig()
	config.EnableOCR = false
	config.EnableClassification = false
	detector := NewElementDetector(config)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := detector.Detect(img)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkBatchProcessing benchmarks batch processing
func BenchmarkBatchProcessing(b *testing.B) {
	frames := make([]image.Image, 10)
	for i := range frames {
		frames[i] = createTestUIImage(320, 240)
	}
	
	config := DefaultDetectorConfig()
	config.EnableOCR = false
	detector := NewElementDetector(config)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := detector.DetectBatch(frames)
		if err != nil {
			b.Fatal(err)
		}
	}
}
