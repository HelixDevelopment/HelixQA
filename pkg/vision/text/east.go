// Package text provides OCR and text extraction capabilities.
//
// This file implements EAST (Efficient and Accurate Scene Text detection)
// for fast text region detection before OCR.
//
// EAST is a deep learning-based text detector that:
// - Runs at real-time speeds (~13 FPS on 720p)
// - Detects text at any orientation
// - Provides word-level bounding boxes
//
//go:build vision
// +build vision

package text

import (
	"context"
	"fmt"
	"image"
	"math"
	"sort"
	"sync"

	"gocv.io/x/gocv"

	"digital.vasic.helixqa/pkg/vision/core"
)

// EASTDetector implements text detection using the EAST model.
//
// Usage:
//
//	detector, err := text.NewEASTDetector("models/east/frozen_east_text_detection.pb", text.EASTConfig{
//	    ConfidenceThreshold: 0.5,
//	    NMSThreshold: 0.4,
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer detector.Close()
//	
//	regions, err := detector.Detect(ctx, frame)
type EASTDetector struct {
	config EASTConfig
	net    gocv.Net
	
	// Pre-allocated mats for reuse
	blob       gocv.Mat
	scores     gocv.Mat
	geometry   gocv.Mat
	
	// Thread safety
	mutex sync.Mutex
}

// EASTConfig configures the EAST detector.
type EASTConfig struct {
	// ModelPath to the frozen EAST model
	ModelPath string

	// ConfidenceThreshold for text detection (0-1)
	ConfidenceThreshold float64

	// NMSThreshold for non-maximum suppression (0-1)
	NMSThreshold float64

	// InputWidth for the model (must be multiple of 32)
	InputWidth int

	// InputHeight for the model (must be multiple of 32)
	InputHeight int

	// Padding around detected regions
	Padding int

	// MergeOverlapping merges overlapping boxes
	MergeOverlapping bool

	// MinTextWidth filters out small detections
	MinTextWidth int

	// MinTextHeight filters out small detections
	MinTextHeight int
}

// DefaultEASTConfig returns sensible defaults.
func DefaultEASTConfig() EASTConfig {
	return EASTConfig{
		ModelPath:           "models/east/frozen_east_text_detection.pb",
		ConfidenceThreshold: 0.5,
		NMSThreshold:        0.4,
		InputWidth:          320,
		InputHeight:         320,
		Padding:             2,
		MergeOverlapping:    true,
		MinTextWidth:        10,
		MinTextHeight:       10,
	}
}

// EASTBox represents a detected text box with rotation.
type EASTBox struct {
	// Rectangle bounding box
	Rect image.Rectangle

	// Angle of rotation (radians)
	Angle float64

	// Confidence score
	Confidence float64
}

// NewEASTDetector creates a new EAST text detector.
func NewEASTDetector(config EASTConfig) (*EASTDetector, error) {
	// Load the EAST model
	net := gocv.ReadNet(config.ModelPath)
	if net.Empty() {
		return nil, fmt.Errorf("failed to load EAST model from %s", config.ModelPath)
	}

	// Prefer OpenCL/CUDA if available
	net.SetPreferableBackend(gocv.NetBackendDefault)
	net.SetPreferableTarget(gocv.NetTargetCPU)

	return &EASTDetector{
		config:   config,
		net:      net,
		blob:     gocv.NewMat(),
		scores:   gocv.NewMat(),
		geometry: gocv.NewMat(),
	}, nil
}

// Close releases resources.
func (d *EASTDetector) Close() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.net.Close()
	d.blob.Close()
	d.scores.Close()
	d.geometry.Close()

	return nil
}

// Detect finds text regions in a frame.
func (d *EASTDetector) Detect(ctx context.Context, frame *core.Frame) ([]core.TextRegion, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Convert frame to mat
	mat, err := bytesToMat(frame.Data, frame.Bounds)
	if err != nil {
		return nil, fmt.Errorf("converting frame to mat: %w", err)
	}
	defer mat.Close()

	// Get original dimensions
	origH, origW := mat.Rows(), mat.Cols()

	// Create blob from image
	// EAST expects: blob = cv2.dnn.blobFromImage(image, 1.0, (W, H), (123.68, 116.78, 103.94), swapRB=True, crop=False)
	gocv.BlobFromImage(mat, &d.blob, 1.0,
		image.Pt(d.config.InputWidth, d.config.InputHeight),
		gocv.NewScalar(123.68, 116.78, 103.94, 0),
		true, false,
	)

	// Set input and run forward pass
	d.net.SetInput(d.blob, "input_images")
	
	// Get output layers
	// EAST has two outputs: "feature_fusion/Conv_7/Sigmoid" (scores) and "feature_fusion/concat_3" (geometry)
	outputs := []string{"feature_fusion/Conv_7/Sigmoid", "feature_fusion/concat_3"}
	
	// Run inference
	outputMats := d.net.ForwardLayers(outputs)
	if len(outputMats) != 2 {
		return nil, fmt.Errorf("expected 2 output layers, got %d", len(outputMats))
	}

	d.scores = outputMats[0]
	d.geometry = outputMats[1]

	// Decode predictions
	boxes := d.decodePredictions(origW, origH)

	// Apply NMS
	boxes = d.applyNMS(boxes)

	// Filter by size
	boxes = d.filterBySize(boxes)

	// Convert to TextRegions
	regions := make([]core.TextRegion, 0, len(boxes))
	for _, box := range boxes {
		// Add padding
		padded := d.addPadding(box.Rect, origW, origH)

		region := core.TextRegion{
			Bounds: core.Rectangle{
				Rectangle:  padded,
				Confidence: box.Confidence,
			},
			Text:       "", // Text not extracted yet
			Confidence: box.Confidence,
			Language:   "",
			IsVertical: math.Abs(box.Angle) > math.Pi/4,
		}
		regions = append(regions, region)
	}

	// Clean up output mats
	for _, m := range outputMats {
		m.Close()
	}

	return regions, nil
}

// DetectWithOCR finds text regions and extracts text using OCR.
func (d *EASTDetector) DetectWithOCR(
	ctx context.Context,
	frame *core.Frame,
	ocr core.TextExtractor,
) ([]core.TextRegion, error) {
	// First detect text regions
	regions, err := d.Detect(ctx, frame)
	if err != nil {
		return nil, err
	}

	// Extract text from each region
	results := make([]core.TextRegion, 0, len(regions))
	for _, region := range regions {
		ext, err := ocr.ExtractRegion(ctx, frame, region.Bounds.Rectangle)
		if err != nil {
			// Skip regions that fail OCR
			continue
		}

		// Combine EAST confidence with OCR confidence
		combinedConfidence := region.Confidence * ext.Confidence

		results = append(results, core.TextRegion{
			Bounds: core.Rectangle{
				Rectangle:  region.Bounds.Rectangle,
				Confidence: combinedConfidence,
			},
			Text:       ext.Text,
			Confidence: combinedConfidence,
			Language:   ext.Language,
			IsVertical: region.IsVertical,
		})
	}

	return results, nil
}

// decodePredictions decodes the EAST model outputs.
func (d *EASTDetector) decodePredictions(origW, origH int) []EASTBox {
	scores := d.scores
	geometry := d.geometry

	numRows, numCols := scores.Size()[2], scores.Size()[3]

	boxes := make([]EASTBox, 0)
	confidences := make([]float32, 0)

	for y := 0; y < numRows; y++ {
		scoresData := scores.GetFloatAt(0, 0, y)
		x0Data := geometry.GetFloatAt(0, 0, y)
		x1Data := geometry.GetFloatAt(0, 1, y)
		x2Data := geometry.GetFloatAt(0, 2, y)
		x3Data := geometry.GetFloatAt(0, 3, y)
		anglesData := geometry.GetFloatAt(0, 4, y)

		for x := 0; x < numCols; x++ {
			if scoresData[x] < float32(d.config.ConfidenceThreshold) {
				continue
			}

			// Compute the offset factor
			offsetX := float64(x) * 4.0
			offsetY := float64(y) * 4.0

			// Extract geometry
			angle := anglesData[x]
			cosA := math.Cos(float64(angle))
			sinA := math.Sin(float64(angle))

			h := x0Data[x] + x2Data[x]
			w := x1Data[x] + x3Data[x]

			// Compute box corners
			endX := int(offsetX + cosA*x1Data[x] + sinA*x2Data[x])
			endY := int(offsetY - sinA*x1Data[x] + cosA*x2Data[x])
			startX := int(offsetX - cosA*x3Data[x] - sinA*x0Data[x])
			startY := int(offsetY + sinA*x3Data[x] - cosA*x0Data[x])

			// Scale to original image size
			startX = int(float64(startX) * float64(origW) / float64(d.config.InputWidth))
			startY = int(float64(startY) * float64(origH) / float64(d.config.InputHeight))
			endX = int(float64(endX) * float64(origW) / float64(d.config.InputWidth))
			endY = int(float64(endY) * float64(origH) / float64(d.config.InputHeight))

			// Ensure valid coordinates
			if startX < 0 {
				startX = 0
			}
			if startY < 0 {
				startY = 0
			}
			if endX > origW {
				endX = origW
			}
			if endY > origH {
				endY = origH
			}

			box := EASTBox{
				Rect: image.Rectangle{
					Min: image.Point{X: startX, Y: startY},
					Max: image.Point{X: endX, Y: endY},
				},
				Angle:      float64(angle),
				Confidence: float64(scoresData[x]),
			}

			boxes = append(boxes, box)
			confidences = append(confidences, scoresData[x])
		}
	}

	return boxes
}

// applyNMS applies non-maximum suppression.
func (d *EASTDetector) applyNMS(boxes []EASTBox) []EASTBox {
	if len(boxes) == 0 {
		return boxes
	}

	// Sort by confidence (descending)
	sort.Slice(boxes, func(i, j int) bool {
		return boxes[i].Confidence > boxes[j].Confidence
	})

	// Simple NMS
	kept := make([]EASTBox, 0)
	suppressed := make([]bool, len(boxes))

	for i := 0; i < len(boxes); i++ {
		if suppressed[i] {
			continue
		}

		kept = append(kept, boxes[i])

		for j := i + 1; j < len(boxes); j++ {
			if suppressed[j] {
				continue
			}

			iou := computeIoU(boxes[i].Rect, boxes[j].Rect)
			if iou > d.config.NMSThreshold {
				suppressed[j] = true
			}
		}
	}

	return kept
}

// filterBySize filters boxes by minimum size.
func (d *EASTDetector) filterBySize(boxes []EASTBox) []EASTBox {
	filtered := make([]EASTBox, 0, len(boxes))
	for _, box := range boxes {
		w := box.Rect.Dx()
		h := box.Rect.Dy()

		if w >= d.config.MinTextWidth && h >= d.config.MinTextHeight {
			filtered = append(filtered, box)
		}
	}
	return filtered
}

// addPadding adds padding to a rectangle.
func (d *EASTDetector) addPadding(rect image.Rectangle, maxW, maxH int) image.Rectangle {
	padded := rect.Inset(-d.config.Padding)

	// Clamp to image bounds
	if padded.Min.X < 0 {
		padded.Min.X = 0
	}
	if padded.Min.Y < 0 {
		padded.Min.Y = 0
	}
	if padded.Max.X > maxW {
		padded.Max.X = maxW
	}
	if padded.Max.Y > maxH {
		padded.Max.Y = maxH
	}

	return padded
}

// computeIoU computes intersection over union of two rectangles.
func computeIoU(a, b image.Rectangle) float64 {
	intersection := a.Intersect(b)
	if intersection.Empty() {
		return 0
	}

	interArea := intersection.Dx() * intersection.Dy()
	areaA := a.Dx() * a.Dy()
	areaB := b.Dx() * b.Dy()
	unionArea := areaA + areaB - interArea

	if unionArea == 0 {
		return 0
	}

	return float64(interArea) / float64(unionArea)
}

// bytesToMat converts byte data to gocv.Mat.
func bytesToMat(data []byte, bounds image.Rectangle) (gocv.Mat, error) {
	// For RGB data
	mat, err := gocv.NewMatFromBytes(bounds.Dy(), bounds.Dx(), gocv.MatTypeCV8UC3, data)
	if err != nil {
		return gocv.NewMat(), err
	}
	return mat, nil
}
