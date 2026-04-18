// Package vision provides computer vision and UI element detection
package vision

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"sync"
	"time"
)

// ElementType represents the type of UI element detected
type ElementType string

const (
	ElementButton   ElementType = "button"
	ElementInput    ElementType = "input"
	ElementText     ElementType = "text"
	ElementImage    ElementType = "image"
	ElementLink     ElementType = "link"
	ElementCheckbox ElementType = "checkbox"
	ElementRadio    ElementType = "radio"
	ElementDropdown ElementType = "dropdown"
	ElementSlider   ElementType = "slider"
	ElementToggle   ElementType = "toggle"
	ElementMenu     ElementType = "menu"
	ElementTab      ElementType = "tab"
	ElementScroll   ElementType = "scroll"
	ElementUnknown  ElementType = "unknown"
)

// Element represents a detected UI element
type Element struct {
	ID         string            `json:"id"`
	Type       ElementType       `json:"type"`
	Bounds     image.Rectangle   `json:"bounds"`
	Confidence float64           `json:"confidence"`
	Text       string            `json:"text,omitempty"`
	Label      string            `json:"label,omitempty"`
	Enabled    bool              `json:"enabled"`
	Visible    bool              `json:"visible"`
	Selected   bool              `json:"selected,omitempty"`
	Focused    bool              `json:"focused,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// FrameResult contains the analysis results for a frame
type FrameResult struct {
	FrameID    string      `json:"frame_id"`
	Timestamp  time.Time   `json:"timestamp"`
	Elements   []Element   `json:"elements"`
	TextBlocks []TextBlock `json:"text_blocks,omitempty"`
	LatencyMs  float64     `json:"latency_ms"`
}

// TextBlock represents a block of text detected in the frame
type TextBlock struct {
	Text       string          `json:"text"`
	Bounds     image.Rectangle `json:"bounds"`
	Confidence float64         `json:"confidence"`
	Language   string          `json:"language,omitempty"`
}

// DetectorConfig configures the element detector
type DetectorConfig struct {
	// Confidence threshold (0-1)
	MinConfidence float64

	// Maximum number of elements to detect
	MaxElements int

	// Enable text detection (OCR)
	EnableOCR bool

	// Enable element classification
	EnableClassification bool

	// Processing resolution (lower = faster)
	ProcessingWidth  int
	ProcessingHeight int

	// Detection model path (if using ML)
	ModelPath string

	// Parallel processing workers
	Workers int
}

// DefaultDetectorConfig returns default configuration
func DefaultDetectorConfig() DetectorConfig {
	return DetectorConfig{
		MinConfidence:        0.7,
		MaxElements:          100,
		EnableOCR:            true,
		EnableClassification: true,
		ProcessingWidth:      640,
		ProcessingHeight:     480,
		Workers:              4,
	}
}

// ElementDetector detects UI elements in video frames
type ElementDetector struct {
	config    DetectorConfig
	mu        sync.RWMutex
	statsMu   sync.RWMutex // guards stats fields
	workers   chan struct{}
	stats     DetectorStats
	ocrEngine OCREngine
}

// DetectorStats holds detection statistics.
// It is a pure data struct with no embedded mutex; callers must hold
// the parent ElementDetector.statsMu lock while reading/writing fields.
type DetectorStats struct {
	FramesProcessed uint64
	ElementsFound   uint64
	Errors          uint64
	AvgLatencyMs    float64
}

// OCREngine interface for text recognition
type OCREngine interface {
	DetectText(img image.Image) ([]TextBlock, error)
}

// NewElementDetector creates a new element detector
func NewElementDetector(config DetectorConfig) *ElementDetector {
	return &ElementDetector{
		config:  config,
		workers: make(chan struct{}, config.Workers),
	}
}

// SetOCREngine sets the OCR engine
func (ed *ElementDetector) SetOCREngine(engine OCREngine) {
	ed.mu.Lock()
	defer ed.mu.Unlock()
	ed.ocrEngine = engine
}

// Detect analyzes a frame and returns detected elements
func (ed *ElementDetector) Detect(frame image.Image) (*FrameResult, error) {
	start := time.Now()

	// Acquire worker
	ed.workers <- struct{}{}
	defer func() { <-ed.workers }()

	// Resize frame for processing if needed
	procFrame := ed.preprocess(frame)

	result := &FrameResult{
		FrameID:   generateFrameID(),
		Timestamp: time.Now(),
		Elements:  make([]Element, 0),
	}

	// Detect UI elements using contour analysis
	elements, err := ed.detectElements(procFrame)
	if err != nil {
		ed.statsMu.Lock()
		ed.stats.Errors++
		ed.statsMu.Unlock()
		return nil, fmt.Errorf("element detection failed: %w", err)
	}

	// Filter by confidence
	for _, elem := range elements {
		if elem.Confidence >= ed.config.MinConfidence {
			// Scale bounds back to original resolution
			elem.Bounds = ed.scaleBounds(elem.Bounds, frame.Bounds(), procFrame.Bounds())
			result.Elements = append(result.Elements, elem)
		}
	}

	// Limit number of elements
	if len(result.Elements) > ed.config.MaxElements {
		result.Elements = result.Elements[:ed.config.MaxElements]
	}

	// OCR if enabled
	if ed.config.EnableOCR && ed.ocrEngine != nil {
		textBlocks, err := ed.ocrEngine.DetectText(frame)
		if err == nil {
			result.TextBlocks = textBlocks

			// Associate text with elements
			ed.associateText(result)
		}
	}

	// Update statistics
	latency := time.Since(start).Milliseconds()
	result.LatencyMs = float64(latency)

	ed.statsMu.Lock()
	ed.stats.FramesProcessed++
	ed.stats.ElementsFound += uint64(len(result.Elements))
	// Update rolling average
	n := float64(ed.stats.FramesProcessed)
	ed.stats.AvgLatencyMs = (ed.stats.AvgLatencyMs*(n-1) + float64(latency)) / n
	ed.statsMu.Unlock()

	return result, nil
}

// DetectBatch processes multiple frames in parallel
func (ed *ElementDetector) DetectBatch(frames []image.Image) ([]*FrameResult, error) {
	results := make([]*FrameResult, len(frames))
	errors := make([]error, len(frames))

	var wg sync.WaitGroup
	for i, frame := range frames {
		wg.Add(1)
		go func(idx int, img image.Image) {
			defer wg.Done()
			result, err := ed.Detect(img)
			results[idx] = result
			errors[idx] = err
		}(i, frame)
	}
	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return results, fmt.Errorf("batch processing had errors")
		}
	}

	return results, nil
}

// GetStats returns a snapshot of detector statistics.
// The returned value is a plain copy with no mutex — safe to read
// without a lock after the function returns.
func (ed *ElementDetector) GetStats() DetectorStats {
	ed.statsMu.RLock()
	snap := DetectorStats{
		FramesProcessed: ed.stats.FramesProcessed,
		ElementsFound:   ed.stats.ElementsFound,
		Errors:          ed.stats.Errors,
		AvgLatencyMs:    ed.stats.AvgLatencyMs,
	}
	ed.statsMu.RUnlock()
	return snap
}

// preprocess resizes and prepares the frame for processing
func (ed *ElementDetector) preprocess(frame image.Image) image.Image {
	bounds := frame.Bounds()

	// Check if resize needed
	if bounds.Dx() <= ed.config.ProcessingWidth && bounds.Dy() <= ed.config.ProcessingHeight {
		return frame
	}

	// Calculate aspect ratio
	ratio := float64(bounds.Dx()) / float64(bounds.Dy())
	newWidth := ed.config.ProcessingWidth
	newHeight := int(float64(newWidth) / ratio)

	if newHeight > ed.config.ProcessingHeight {
		newHeight = ed.config.ProcessingHeight
		newWidth = int(float64(newHeight) * ratio)
	}

	// Resize using bilinear interpolation
	return resizeImage(frame, newWidth, newHeight)
}

// detectElements performs contour-based element detection
func (ed *ElementDetector) detectElements(frame image.Image) ([]Element, error) {
	var elements []Element

	// Convert to grayscale
	gray := toGrayscale(frame)

	// Apply Gaussian blur to reduce noise
	blurred := gaussianBlur(gray, 5)

	// Edge detection using Sobel operator
	edges := sobelEdges(blurred)

	// Find contours
	contours := findContours(edges)

	// Analyze each contour
	for i, contour := range contours {
		// Filter small contours
		area := contourArea(contour)
		if area < 100 { // Minimum area threshold
			continue
		}

		// Get bounding rectangle
		bounds := boundingRect(contour)

		// Classify element type based on shape and features
		elemType := ed.classifyElement(contour, bounds, frame)

		// Calculate confidence based on shape quality
		confidence := ed.calculateConfidence(contour, area)

		element := Element{
			ID:         fmt.Sprintf("elem_%d", i),
			Type:       elemType,
			Bounds:     bounds,
			Confidence: confidence,
			Visible:    true,
			Enabled:    true,
		}

		elements = append(elements, element)
	}

	return elements, nil
}

// classifyElement determines the type of UI element
func (ed *ElementDetector) classifyElement(contour []image.Point, bounds image.Rectangle, frame image.Image) ElementType {
	if !ed.config.EnableClassification {
		return ElementUnknown
	}

	// Calculate aspect ratio
	width := bounds.Dx()
	height := bounds.Dy()
	aspectRatio := float64(width) / float64(height)

	// Calculate solidity (area / convex hull area)
	solidity := calculateSolidity(contour)

	// Calculate extent (area / bounding rect area)
	extent := calculateExtent(contour, bounds)

	// Classify based on geometric features
	switch {
	// Buttons: moderate aspect ratio, high solidity
	case aspectRatio >= 0.5 && aspectRatio <= 3.0 && solidity > 0.8:
		return ElementButton

	// Input fields: wide aspect ratio, high solidity
	case aspectRatio > 3.0 && aspectRatio < 15.0 && solidity > 0.7:
		return ElementInput

	// Checkboxes: near-square, small size
	case aspectRatio >= 0.8 && aspectRatio <= 1.2 && width < 50 && height < 50:
		return ElementCheckbox

	// Radio buttons: near-circular, small size
	case aspectRatio >= 0.9 && aspectRatio <= 1.1 && width < 40 && height < 40 && extent > 0.7:
		return ElementRadio

	// Images: varying aspect ratio, lower solidity (may have transparency)
	case solidity < 0.7 && extent < 0.9:
		return ElementImage

	// Sliders: very wide or very tall
	case aspectRatio > 10.0 || aspectRatio < 0.1:
		return ElementSlider

	// Text blocks: wide, low solidity (text has gaps)
	case aspectRatio > 2.0 && solidity < 0.6:
		return ElementText

	default:
		return ElementUnknown
	}
}

// calculateConfidence calculates detection confidence
func (ed *ElementDetector) calculateConfidence(contour []image.Point, area float64) float64 {
	// Base confidence
	confidence := 0.7

	// Adjust based on contour quality
	perimeter := contourPerimeter(contour)
	if perimeter > 0 {
		// Circularity: 4π*area/perimeter² (1 = perfect circle)
		circularity := 4 * math.Pi * area / (perimeter * perimeter)
		confidence += circularity * 0.2
	}

	// Penalize very small or very large areas
	if area < 500 || area > 500000 {
		confidence -= 0.2
	}

	// Clamp to [0, 1]
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.0 {
		confidence = 0.0
	}

	return confidence
}

// scaleBounds scales bounding box from processed resolution to original
func (ed *ElementDetector) scaleBounds(bounds, origBounds, procBounds image.Rectangle) image.Rectangle {
	scaleX := float64(origBounds.Dx()) / float64(procBounds.Dx())
	scaleY := float64(origBounds.Dy()) / float64(procBounds.Dy())

	return image.Rect(
		int(float64(bounds.Min.X)*scaleX),
		int(float64(bounds.Min.Y)*scaleY),
		int(float64(bounds.Max.X)*scaleX),
		int(float64(bounds.Max.Y)*scaleY),
	)
}

// associateText associates detected text with elements
func (ed *ElementDetector) associateText(result *FrameResult) {
	for i := range result.Elements {
		elem := &result.Elements[i]

		// Find text blocks that overlap with element
		for _, textBlock := range result.TextBlocks {
			if boundsOverlap(elem.Bounds, textBlock.Bounds) {
				if elem.Text == "" {
					elem.Text = textBlock.Text
				} else {
					elem.Text += " " + textBlock.Text
				}

				// If element has no label, use text as label
				if elem.Label == "" {
					elem.Label = textBlock.Text
				}
			}
		}
	}
}

// Helper functions

func generateFrameID() string {
	return fmt.Sprintf("frame_%d", time.Now().UnixNano())
}

func boundsOverlap(a, b image.Rectangle) bool {
	return a.Overlaps(b)
}

// Image processing helpers (simplified implementations)

func toGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := color.GrayModel.Convert(img.At(x, y)).(color.Gray)
			gray.Set(x, y, c)
		}
	}

	return gray
}

func gaussianBlur(img *image.Gray, kernelSize int) *image.Gray {
	// Simplified blur - in production use proper Gaussian kernel
	return img
}

func sobelEdges(img *image.Gray) *image.Gray {
	bounds := img.Bounds()
	edges := image.NewGray(bounds)

	// Sobel operators
	sobelX := [][]int{{-1, 0, 1}, {-2, 0, 2}, {-1, 0, 1}}
	sobelY := [][]int{{-1, -2, -1}, {0, 0, 0}, {1, 2, 1}}

	for y := bounds.Min.Y + 1; y < bounds.Max.Y-1; y++ {
		for x := bounds.Min.X + 1; x < bounds.Max.X-1; x++ {
			var gx, gy int

			for ky := -1; ky <= 1; ky++ {
				for kx := -1; kx <= 1; kx++ {
					c := img.GrayAt(x+kx, y+ky).Y
					gx += int(c) * sobelX[ky+1][kx+1]
					gy += int(c) * sobelY[ky+1][kx+1]
				}
			}

			magnitude := math.Sqrt(float64(gx*gx + gy*gy))
			if magnitude > 255 {
				magnitude = 255
			}

			edges.SetGray(x, y, color.Gray{Y: uint8(magnitude)})
		}
	}

	return edges
}

type Point = image.Point

func findContours(edges *image.Gray) [][]Point {
	// Simplified contour finding - in production use Suzuki's algorithm
	// For now, return dummy contours based on edge density
	bounds := edges.Bounds()
	var contours [][]Point

	// Find connected components (simplified)
	visited := make(map[Point]bool)

	for y := bounds.Min.Y; y < bounds.Max.Y; y += 10 {
		for x := bounds.Min.X; x < bounds.Max.X; x += 10 {
			pt := Point{X: x, Y: y}
			if visited[pt] {
				continue
			}

			if edges.GrayAt(x, y).Y > 128 {
				contour := traceContour(edges, x, y, visited)
				if len(contour) > 10 {
					contours = append(contours, contour)
				}
			}
		}
	}

	return contours
}

func traceContour(edges *image.Gray, startX, startY int, visited map[Point]bool) []Point {
	// Simplified contour tracing
	var contour []Point
	bounds := edges.Bounds()

	x, y := startX, startY
	for i := 0; i < 1000; i++ { // Limit iterations
		pt := Point{X: x, Y: y}
		if visited[pt] {
			break
		}
		visited[pt] = true
		contour = append(contour, pt)

		// Find next point
		found := false
		for dy := -1; dy <= 1 && !found; dy++ {
			for dx := -1; dx <= 1 && !found; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				nx, ny := x+dx, y+dy
				if nx < bounds.Min.X || nx >= bounds.Max.X || ny < bounds.Min.Y || ny >= bounds.Max.Y {
					continue
				}
				npt := Point{X: nx, Y: ny}
				if !visited[npt] && edges.GrayAt(nx, ny).Y > 128 {
					x, y = nx, ny
					found = true
					break
				}
			}
		}

		if !found {
			break
		}
	}

	return contour
}

func contourArea(contour []Point) float64 {
	if len(contour) < 3 {
		return 0
	}

	// Shoelace formula
	area := 0.0
	j := len(contour) - 1

	for i := 0; i < len(contour); i++ {
		area += float64(contour[j].X+contour[i].X) * float64(contour[j].Y-contour[i].Y)
		j = i
	}

	return math.Abs(area) / 2.0
}

func boundingRect(contour []Point) image.Rectangle {
	if len(contour) == 0 {
		return image.Rect(0, 0, 0, 0)
	}

	minX, minY := contour[0].X, contour[0].Y
	maxX, maxY := contour[0].X, contour[0].Y

	for _, pt := range contour[1:] {
		if pt.X < minX {
			minX = pt.X
		}
		if pt.X > maxX {
			maxX = pt.X
		}
		if pt.Y < minY {
			minY = pt.Y
		}
		if pt.Y > maxY {
			maxY = pt.Y
		}
	}

	return image.Rect(minX, minY, maxX, maxY)
}

func contourPerimeter(contour []Point) float64 {
	if len(contour) < 2 {
		return 0
	}

	perimeter := 0.0
	for i := 0; i < len(contour); i++ {
		next := (i + 1) % len(contour)
		dx := float64(contour[next].X - contour[i].X)
		dy := float64(contour[next].Y - contour[i].Y)
		perimeter += math.Sqrt(dx*dx + dy*dy)
	}

	return perimeter
}

func calculateSolidity(contour []Point) float64 {
	// Simplified - in production compute convex hull
	area := contourArea(contour)
	if area == 0 {
		return 0
	}

	// Approximate convex hull area as bounding rect area
	bounds := boundingRect(contour)
	hullArea := float64(bounds.Dx() * bounds.Dy())

	if hullArea == 0 {
		return 0
	}

	return area / hullArea
}

func calculateExtent(contour []Point, bounds image.Rectangle) float64 {
	area := contourArea(contour)
	rectArea := float64(bounds.Dx() * bounds.Dy())

	if rectArea == 0 {
		return 0
	}

	return area / rectArea
}

func resizeImage(img image.Image, width, height int) image.Image {
	// Simplified nearest-neighbor resize
	bounds := img.Bounds()
	resized := image.NewRGBA(image.Rect(0, 0, width, height))

	scaleX := float64(bounds.Dx()) / float64(width)
	scaleY := float64(bounds.Dy()) / float64(height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := int(float64(x) * scaleX)
			srcY := int(float64(y) * scaleY)

			if srcX >= bounds.Min.X && srcX < bounds.Max.X && srcY >= bounds.Min.Y && srcY < bounds.Max.Y {
				resized.Set(x, y, img.At(srcX, srcY))
			}
		}
	}

	return resized
}
