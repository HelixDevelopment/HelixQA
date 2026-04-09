# HelixQA OpenCV Integration Architecture
## Comprehensive Computer Vision Enhancement for UI/UX Testing

**Version**: 1.0  
**Date**: 2026-04-09  
**Status**: Implementation Ready

---

## Executive Summary

This document outlines a comprehensive OpenCV-based computer vision pipeline for HelixQA that significantly reduces reliance on external Vision LLMs while improving test execution speed and accuracy. The architecture provides real-time UI element detection, text extraction, layout analysis, and visual regression testing.

---

## 1. Core OpenCV Modules for HelixQA

### 1.1 Feature Detection & Element Matching (`pkg/vision/element_detector.go`)

**Purpose**: Detect and locate UI elements without relying on accessibility trees or DOM queries.

**Algorithms**:
- **ORB (Oriented FAST and Rotated BRIEF)** - Primary choice (fast, rotation invariant, patent-free)
- **SIFT** - Backup for complex scenes (scale/rotation invariant, patented)
- **AKAZE** - Alternative for illumination changes

**Implementation**:
```go
type ElementDetector struct {
    orb         *gocv.ORB
    sift        *gocv.SIFT
    templates   map[string]gocv.Mat
    matcher     *gocv.BFMatcher
}

func (ed *ElementDetector) FindElement(
    screenshot gocv.Mat, 
    templateName string,
    confidence float64,
) (x, y, width, height int, found bool) {
    // Use ORB for fast template matching
    // Return coordinates for ADB interaction
}
```

**Capabilities**:
- Find buttons, text fields, icons by image template
- Match elements across different screen resolutions
- Detect rotated or scaled UI elements
- Real-time tracking during animations

---

### 1.2 Text Detection & OCR Pipeline (`pkg/vision/text_extractor.go`)

**Purpose**: Extract all visible text from UI screenshots for verification and navigation.

**Two-Stage Pipeline**:

**Stage 1: Text Detection (OpenCV EAST)**
```go
type TextDetector struct {
    eastNet gocv.Net // Pre-trained EAST model
}

func (td *TextDetector) DetectTextRegions(
    img gocv.Mat,
) []TextRegion {
    // Returns bounding boxes of text regions
    // Much faster than full OCR
}
```

**Stage 2: OCR (Tesseract)**
```go
type OCREngine struct {
    tesseract *tesseract.Client
}

func (ocr *OCREngine) RecognizeText(
    img gocv.Mat,
    regions []TextRegion,
) []ExtractedText {
    // Extract actual text content from detected regions
}
```

**Capabilities**:
- Detect text regions in <50ms per frame
- Extract button labels, headings, error messages
- Multi-language support via Tesseract
- Confidence scoring for each text block

---

### 1.3 UI Layout Analysis (`pkg/vision/layout_analyzer.go`)

**Purpose**: Understand UI structure and hierarchy through computer vision.

**Techniques**:
- **Edge Detection** (Canny) - Detect UI boundaries
- **Contour Detection** - Identify UI components
- **Hough Transform** - Detect lines for layout structure
- **MSER (Maximally Stable Extremal Regions)** - Detect text/button regions

```go
type LayoutAnalyzer struct {
    cannyThreshold1 float64
    cannyThreshold2 float64
}

func (la *LayoutAnalyzer) AnalyzeLayout(
    screenshot gocv.Mat,
) UILayout {
    // Detect:
    // - Navigation bars
    // - Content areas
    // - Input fields
    // - Button groups
    // - Lists/grids
}
```

**Output Structure**:
```go
type UILayout struct {
    NavigationBar   *UIRegion
    ContentArea     *UIRegion
    InputFields     []UIRegion
    Buttons         []UIRegion
    TextBlocks      []TextBlock
    Hierarchy       *LayoutTree
}
```

---

### 1.4 Visual Regression Engine (`pkg/vision/regression_detector.go`)

**Purpose**: Detect visual changes between app versions.

**Algorithms**:
- **Structural Similarity Index (SSIM)** - Perceptual difference
- **Histogram Comparison** - Color/layout changes
- **Template Matching** - Element position changes
- **ORB Feature Diff** - Content changes

```go
type RegressionDetector struct {
    baselineStore *BaselineStore
}

func (rd *RegressionDetector) CompareScreenshots(
    baseline, current gocv.Mat,
) DiffReport {
    // Multi-metric comparison:
    // 1. SSIM for perceptual similarity
    // 2. Histogram diff for color changes
    // 3. ORB feature matching for structural changes
}
```

---

## 2. Real-Time Processing Pipeline

### 2.1 Video Stream Processor (`pkg/vision/stream_processor.go`)

**Purpose**: Process live video feed from Android TV/Desktop for continuous monitoring.

```go
type StreamProcessor struct {
    frameBuffer   chan gocv.Mat
    elementCache  *ElementCache
    textCache     *TextCache
    eventStream   chan VisionEvent
}

func (sp *StreamProcessor) Start(ctx context.Context) {
    // 1. Capture frames at 5-10 FPS (configurable)
    // 2. Run parallel processing:
    //    - Element detection (ORB)
    //    - Text extraction (EAST + Tesseract)
    //    - Layout analysis (Contours)
    //    - Change detection (Frame differencing)
    // 3. Emit VisionEvents for significant changes
}
```

**Performance Targets**:
- Frame capture: ~10 FPS
- Element detection: <30ms per frame
- Text extraction: <100ms per frame
- Layout analysis: <50ms per frame

---

### 2.2 Smart Screenshot Manager (`pkg/vision/screenshot_manager.go`)

**Purpose**: Intelligent screenshot capture based on UI stability.

```go
type ScreenshotManager struct {
    stabilityDetector *StabilityDetector
    captureStrategy   CaptureStrategy
}

func (sm *ScreenshotManager) CaptureWhenStable(
    timeout time.Duration,
) (gocv.Mat, error) {
    // Wait for UI to stabilize:
    // - No motion for N frames
    // - Text content stable
    // - No loading indicators
}
```

---

## 3. Integration with HelixQA

### 3.1 Vision-Augmented Navigator (`pkg/navigator/cv_navigator.go`)

**Purpose**: Navigation using computer vision instead of element IDs.

```go
type CVNavigator struct {
    elementDetector *ElementDetector
    textExtractor   *TextExtractor
    layoutAnalyzer  *LayoutAnalyzer
}

func (cvn *CVNavigator) FindAndClick(
    elementDescription string, // "Sign In button", "Username field"
) error {
    // 1. Extract text from current screen
    // 2. Match description to text regions
    // 3. If text match found, click center of region
    // 4. If no text match, try template matching
    // 5. Verify click was successful (screen changed)
}

func (cvn *CVNavigator) NavigateTo(
    destination string, // "Settings page", "Media browser"
) error {
    // Use layout analysis + text extraction to navigate
    // without requiring predefined selectors
}
```

---

### 3.2 Smart Action Verifier (`pkg/vision/action_verifier.go`)

**Purpose**: Verify actions completed successfully using visual confirmation.

```go
type ActionVerifier struct {
    preState      gocv.Mat
    expectedState string // "modal opened", "page changed", "toast appeared"
}

func (av *ActionVerifier) VerifyAction(
    action func(),
    expectedChange ChangeType,
) bool {
    // 1. Capture pre-action state
    // 2. Execute action
    // 3. Wait for stability
    // 4. Capture post-action state
    // 5. Verify expected change occurred:
    //    - New elements appeared
    //    - Text content changed
    //    - Layout changed
}
```

---

## 4. OpenCV Dependencies

### 4.1 GoCV (Go OpenCV Bindings)

```bash
# Installation
go get -u gocv.io/x/gocv

# System dependencies
# Ubuntu/Debian:
sudo apt-get install libopencv-dev

# macOS:
brew install opencv

# Windows:
# Download OpenCV 4.x and extract to C:\opencv
```

### 4.2 Additional Libraries

```go
// go.mod additions
require (
    gocv.io/x/gocv v0.35.0
    github.com/otiai10/gosseract/v2 v2.4.1  // Tesseract bindings
    github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
)
```

### 4.3 Pre-trained Models

| Model | Purpose | Size | Location |
|-------|---------|------|----------|
| `frozen_east_text_detection.pb` | Text detection | 93MB | `models/east/` |
| `DB_TD500_resnet50.onnx` | Alternative text det | 25MB | `models/dbnet/` |
| Tesseract traineddata | OCR languages | ~40MB/lang | `tessdata/` |

---

## 5. Performance Optimizations

### 5.1 GPU Acceleration

```go
// Enable CUDA for compatible operations
func init() {
    if gocv.GetCudaEnabledDeviceCount() > 0 {
        gocv.SetUseOptimized(true)
    }
}
```

### 5.2 Caching Strategy

```go
type VisionCache struct {
    elementFeatures  map[string][]gocv.KeyPoint
    textRegions      map[string][]TextRegion
    layoutSnapshots  *lru.Cache
}

// Cache ORB keypoints for unchanged elements
// Avoid recomputing expensive operations
```

### 5.3 Region of Interest (ROI) Processing

```go
// Only process relevant screen regions
func (ed *ElementDetector) ProcessROI(
    fullScreen gocv.Mat,
    roi image.Rectangle,
) gocv.Mat {
    // Crop to ROI before processing
    // Significant speedup for focused searches
}
```

---

## 6. Implementation Roadmap

### Phase 1: Core Infrastructure (Week 1-2)
- [ ] Set up GoCV dependency
- [ ] Implement basic element detector with ORB
- [ ] Implement text detector with EAST
- [ ] Integrate Tesseract OCR

### Phase 2: Layout Analysis (Week 3)
- [ ] Implement contour-based UI segmentation
- [ ] Build layout hierarchy parser
- [ ] Create UI region classifier

### Phase 3: Navigation Enhancement (Week 4)
- [ ] Build CVNavigator
- [ ] Implement smart action verification
- [ ] Add stability detection

### Phase 4: Optimization (Week 5)
- [ ] Add GPU acceleration
- [ ] Implement caching
- [ ] Performance tuning

### Phase 5: Integration (Week 6)
- [ ] Integrate with existing HelixQA pipeline
- [ ] Add configuration options
- [ ] Write comprehensive tests

---

## 7. Benefits Summary

| Metric | Before (LLM-only) | After (OpenCV + LLM) | Improvement |
|--------|------------------|---------------------|-------------|
| Screenshot analysis | 2-5 seconds | 50-150ms | **20-100x faster** |
| Element location | Requires IDs | Visual detection | **No dependency** |
| Text extraction | OCR API calls | Local Tesseract | **10x cheaper** |
| Navigation failures | High | Low | **More robust** |
| Offline capability | No | Yes | **Full autonomy** |

---

## 8. Example Usage

```go
// Initialize vision pipeline
vision := helixqa.NewVisionPipeline(
    helixqa.WithElementTemplates("templates/"),
    helixqa.WithTesseractData("tessdata/"),
    helixqa.WithCacheSize(100),
)

// Find and click element
x, y, found := vision.FindElement(screenshot, "login_button")
if found {
    adb.Tap(x, y)
}

// Extract all text
texts := vision.ExtractText(screenshot)
for _, text := range texts {
    if text.Content == "Sign In" {
        adb.Tap(text.Bounds.CenterX, text.Bounds.CenterY)
    }
}

// Analyze layout
layout := vision.AnalyzeLayout(screenshot)
for _, field := range layout.InputFields {
    fmt.Printf("Input field at (%d, %d)\n", field.X, field.Y)
}
```

---

## References

1. OpenCV Documentation: https://docs.opencv.org/
2. GoCV: https://gocv.io/
3. EAST Text Detection: https://arxiv.org/abs/1704.03155
4. ORB Algorithm: https://arxiv.org/abs/1106.1595
5. Tesseract OCR: https://github.com/tesseract-ocr/tesseract
