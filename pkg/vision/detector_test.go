package vision

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDetectorConfig(t *testing.T) {
	config := DefaultDetectorConfig()

	assert.Equal(t, 0.7, config.MinConfidence)
	assert.Equal(t, 100, config.MaxElements)
	assert.True(t, config.EnableOCR)
	assert.True(t, config.EnableClassification)
	assert.Equal(t, 640, config.ProcessingWidth)
	assert.Equal(t, 480, config.ProcessingHeight)
	assert.Equal(t, 4, config.Workers)
}

func TestElementDetector_SetOCREngine(t *testing.T) {
	config := DefaultDetectorConfig()
	detector := NewElementDetector(config)

	mockEngine := &mockOCREngine{}
	detector.SetOCREngine(mockEngine)

	assert.Equal(t, mockEngine, detector.ocrEngine)
}

func TestElementDetector_Detect(t *testing.T) {
	config := DefaultDetectorConfig()
	config.EnableOCR = false
	config.EnableClassification = false
	detector := NewElementDetector(config)

	img := createTestImage(100, 100)

	result, err := detector.Detect(img)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.FrameID)
	assert.WithinDuration(t, time.Now(), result.Timestamp, time.Second)
	assert.GreaterOrEqual(t, result.LatencyMs, 0.0)
}

func TestElementDetector_DetectBatch(t *testing.T) {
	config := DefaultDetectorConfig()
	config.EnableOCR = false
	config.EnableClassification = false
	detector := NewElementDetector(config)

	frames := []image.Image{
		createTestImage(100, 100),
		createTestImage(200, 200),
		createTestImage(300, 300),
	}

	results, err := detector.DetectBatch(frames)

	require.NoError(t, err)
	assert.Len(t, results, len(frames))
}

func TestElementDetector_GetStats(t *testing.T) {
	config := DefaultDetectorConfig()
	config.EnableOCR = false
	config.EnableClassification = false
	detector := NewElementDetector(config)

	img := createTestImage(100, 100)
	detector.Detect(img)
	detector.Detect(img)

	stats := detector.GetStats()

	assert.Equal(t, uint64(2), stats.FramesProcessed)
	assert.GreaterOrEqual(t, stats.AvgLatencyMs, 0.0)
}

func TestCalculateConfidence(t *testing.T) {
	config := DefaultDetectorConfig()
	detector := NewElementDetector(config)

	contour := []image.Point{
		{X: 0, Y: 0}, {X: 100, Y: 0},
		{X: 100, Y: 100}, {X: 0, Y: 100},
	}
	area := 10000.0

	confidence := detector.calculateConfidence(contour, area)

	assert.GreaterOrEqual(t, confidence, 0.0)
	assert.LessOrEqual(t, confidence, 1.0)
}

func TestScaleBounds(t *testing.T) {
	config := DefaultDetectorConfig()
	detector := NewElementDetector(config)

	bounds := image.Rect(10, 10, 50, 50)
	origBounds := image.Rect(0, 0, 100, 100)
	procBounds := image.Rect(0, 0, 50, 50)

	scaled := detector.scaleBounds(bounds, origBounds, procBounds)

	assert.Equal(t, 20, scaled.Min.X)
	assert.Equal(t, 20, scaled.Min.Y)
	assert.Equal(t, 100, scaled.Max.X)
	assert.Equal(t, 100, scaled.Max.Y)
}

func TestBoundsOverlap(t *testing.T) {
	a := image.Rect(0, 0, 50, 50)
	b := image.Rect(25, 25, 75, 75)
	assert.True(t, boundsOverlap(a, b))

	c := image.Rect(0, 0, 10, 10)
	d := image.Rect(20, 20, 30, 30)
	assert.False(t, boundsOverlap(c, d))
}

func TestContourArea(t *testing.T) {
	// Square
	square := []image.Point{
		{X: 0, Y: 0}, {X: 100, Y: 0},
		{X: 100, Y: 100}, {X: 0, Y: 100},
	}
	area := contourArea(square)
	assert.Equal(t, 10000.0, area)

	// Triangle
	triangle := []image.Point{
		{X: 0, Y: 0}, {X: 100, Y: 0}, {X: 50, Y: 100},
	}
	area = contourArea(triangle)
	assert.Equal(t, 5000.0, area)

	// Empty
	empty := []image.Point{}
	area = contourArea(empty)
	assert.Equal(t, 0.0, area)
}

func TestBoundingRect(t *testing.T) {
	contour := []image.Point{
		{X: 10, Y: 20},
		{X: 50, Y: 20},
		{X: 50, Y: 80},
		{X: 10, Y: 80},
	}

	bounds := boundingRect(contour)
	assert.Equal(t, 10, bounds.Min.X)
	assert.Equal(t, 20, bounds.Min.Y)
	assert.Equal(t, 50, bounds.Max.X)
	assert.Equal(t, 80, bounds.Max.Y)
}

func TestContourPerimeter(t *testing.T) {
	// Square
	square := []image.Point{
		{X: 0, Y: 0}, {X: 100, Y: 0},
		{X: 100, Y: 100}, {X: 0, Y: 100},
	}
	perimeter := contourPerimeter(square)
	assert.Equal(t, 400.0, perimeter)

	// Empty
	empty := []image.Point{}
	perimeter = contourPerimeter(empty)
	assert.Equal(t, 0.0, perimeter)
}

func TestCalculateSolidity(t *testing.T) {
	// Perfect square
	square := []image.Point{
		{X: 0, Y: 0}, {X: 100, Y: 0},
		{X: 100, Y: 100}, {X: 0, Y: 100},
	}
	solidity := calculateSolidity(square)
	assert.Equal(t, 1.0, solidity)
}

func TestCalculateExtent(t *testing.T) {
	contour := []image.Point{
		{X: 0, Y: 0}, {X: 100, Y: 0},
		{X: 100, Y: 100}, {X: 0, Y: 100},
	}
	bounds := image.Rect(0, 0, 100, 100)
	extent := calculateExtent(contour, bounds)
	assert.Equal(t, 1.0, extent)
}

func TestToGrayscale(t *testing.T) {
	img := createTestImage(10, 10)
	gray := toGrayscale(img)

	assert.NotNil(t, gray)
	assert.Equal(t, img.Bounds(), gray.Bounds())
}

func TestSobelEdges(t *testing.T) {
	img := image.NewGray(image.Rect(0, 0, 10, 10))
	// Fill with gradient
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.SetGray(x, y, color.Gray{Y: uint8(x * 25)})
		}
	}

	edges := sobelEdges(img)
	assert.NotNil(t, edges)
	assert.Equal(t, img.Bounds(), edges.Bounds())
}

func TestResizeImage(t *testing.T) {
	img := createTestImage(100, 100)
	resized := resizeImage(img, 50, 50)

	assert.NotNil(t, resized)
	assert.Equal(t, 50, resized.Bounds().Dx())
	assert.Equal(t, 50, resized.Bounds().Dy())
}

func TestElementTypes(t *testing.T) {
	types := []ElementType{
		ElementButton,
		ElementInput,
		ElementText,
		ElementImage,
		ElementLink,
		ElementCheckbox,
		ElementRadio,
		ElementDropdown,
		ElementSlider,
		ElementToggle,
		ElementMenu,
		ElementTab,
		ElementScroll,
		ElementUnknown,
	}

	for _, et := range types {
		assert.NotEmpty(t, et)
	}
}

func TestFrameResult(t *testing.T) {
	result := &FrameResult{
		FrameID:   "test-frame",
		Timestamp: time.Now(),
		Elements: []Element{
			{ID: "elem1", Type: ElementButton, Confidence: 0.9},
			{ID: "elem2", Type: ElementInput, Confidence: 0.8},
		},
		TextBlocks: []TextBlock{
			{Text: "Hello", Confidence: 0.95},
		},
		LatencyMs: 15.5,
	}

	assert.Equal(t, "test-frame", result.FrameID)
	assert.Len(t, result.Elements, 2)
	assert.Len(t, result.TextBlocks, 1)
	assert.Equal(t, 15.5, result.LatencyMs)
}

func TestElement(t *testing.T) {
	elem := Element{
		ID:         "test-elem",
		Type:       ElementButton,
		Bounds:     image.Rect(10, 10, 50, 30),
		Confidence: 0.85,
		Text:       "Click me",
		Label:      "Submit",
		Enabled:    true,
		Visible:    true,
		Selected:   false,
		Focused:    true,
	}

	assert.Equal(t, "test-elem", elem.ID)
	assert.Equal(t, ElementButton, elem.Type)
	assert.Equal(t, 0.85, elem.Confidence)
	assert.Equal(t, "Click me", elem.Text)
	assert.True(t, elem.Enabled)
	assert.True(t, elem.Visible)
	assert.True(t, elem.Focused)
}

func TestTextBlock(t *testing.T) {
	tb := TextBlock{
		Text:       "Test text",
		Bounds:     image.Rect(0, 0, 100, 20),
		Confidence: 0.92,
		Language:   "en",
	}

	assert.Equal(t, "Test text", tb.Text)
	assert.Equal(t, 0.92, tb.Confidence)
	assert.Equal(t, "en", tb.Language)
}

// Helper functions

func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Draw a simple pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8(x % 256),
				G: uint8(y % 256),
				B: 128,
				A: 255,
			})
		}
	}

	return img
}

// Mock OCR engine
type mockOCREngine struct {
	textBlocks []TextBlock
	err        error
}

func (m *mockOCREngine) DetectText(img image.Image) ([]TextBlock, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.textBlocks, nil
}
