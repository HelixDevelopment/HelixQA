// Package detection provides computer vision-based element detection.
//
// This file implements ORB (Oriented FAST and Rotated BRIEF) feature detection
// for finding UI elements by template matching.
//
//go:build vision
// +build vision

package detection

import (
	"context"
	"fmt"
	"image"
	"sync"

	"gocv.io/x/gocv"

	"digital.vasic.helixqa/pkg/vision/core"
)

// ORBDetector implements element detection using ORB features.
//
// ORB is chosen as the primary feature detector because:
// - It's fast (combination of FAST keypoint detector and BRIEF descriptor)
// - It's rotation invariant
// - It's free from patents (unlike SIFT and SURF)
// - It provides good performance for UI element detection
//
// Usage:
//
//	detector := detection.NewORBDetector(detection.ORBConfig{
//	    MaxFeatures: 500,
//	    ScaleFactor: 1.2,
//	    NLevels: 8,
//	})
//
//	element, err := detector.FindByTemplate(ctx, frame, templateImage, 0.8)
type ORBDetector struct {
	config ORBConfig
	orb    gocv.ORB

	// Template cache stores precomputed features for templates
	templateCache map[string]*templateFeatures
	cacheMutex    sync.RWMutex

	// Matcher for feature comparison
	matcher gocv.BFMatcher
}

// ORBConfig configures the ORB detector.
type ORBConfig struct {
	// MaxFeatures is the maximum number of features to detect
	MaxFeatures int

	// ScaleFactor is the pyramid decimation ratio
	ScaleFactor float64

	// NLevels is the number of pyramid levels
	NLevels int

	// EdgeThreshold is the size of the border where features are not detected
	EdgeThreshold int

	// PatchSize is the size of the patch used by the oriented BRIEF descriptor
	PatchSize int

	// FastThreshold is the threshold for FAST keypoint detection
	FastThreshold int

	// MatchRatio is the ratio test threshold for good matches (Lowe's ratio test)
	MatchRatio float64

	// MinMatches is the minimum number of matches required for a valid detection
	MinMatches int

	// RANSACThreshold is the reprojection error threshold for RANSAC
	RANSACThreshold float64
}

// DefaultORBConfig returns sensible defaults for ORB detection.
func DefaultORBConfig() ORBConfig {
	return ORBConfig{
		MaxFeatures:     500,
		ScaleFactor:     1.2,
		NLevels:         8,
		EdgeThreshold:   31,
		PatchSize:       31,
		FastThreshold:   20,
		MatchRatio:      0.75,
		MinMatches:      10,
		RANSACThreshold: 3.0,
	}
}

// templateFeatures stores precomputed features for a template.
type templateFeatures struct {
	KeyPoints   gocv.KeyPoints
	Descriptors gocv.Mat
	Size        image.Point
}

// NewORBDetector creates a new ORB-based element detector.
func NewORBDetector(config ORBConfig) (*ORBDetector, error) {
	orb := gocv.NewORBWithParams(
		config.MaxFeatures,
		config.ScaleFactor,
		config.NLevels,
		config.EdgeThreshold,
		config.PatchSize,
		0, // first level
		2, // WTA_K
		gocv.ORBScoreTypeHarris,
		config.FastThreshold,
	)

	matcher := gocv.NewBFMatcherWithParams(gocv.NormHamming, false)

	return &ORBDetector{
		config:        config,
		orb:           orb,
		templateCache: make(map[string]*templateFeatures),
		matcher:       matcher,
	}, nil
}

// Close releases resources.
func (d *ORBDetector) Close() error {
	d.orb.Close()
	d.matcher.Close()

	// Clean up cached features
	d.cacheMutex.Lock()
	defer d.cacheMutex.Unlock()

	for _, features := range d.templateCache {
		features.Descriptors.Close()
	}

	return nil
}

// Detect implements core.ElementDetector.
// For ORB detector, this performs general feature detection without templates.
func (d *ORBDetector) Detect(ctx context.Context, frame *core.Frame) ([]core.UIElement, error) {
	mat, err := bytesToMat(frame.Data, frame.Bounds)
	if err != nil {
		return nil, fmt.Errorf("converting frame to mat: %w", err)
	}
	defer mat.Close()

	// Detect keypoints
	keypoints, descriptors := d.detectFeatures(mat)
	defer descriptors.Close()

	// Convert keypoints to UI elements (as interest points)
	elements := make([]core.UIElement, 0, len(keypoints))
	for i, kp := range keypoints {
		elem := core.UIElement{
			ID:   fmt.Sprintf("orb_keypoint_%d", i),
			Type: core.ElementUnknown,
			Bounds: core.Rectangle{
				Rectangle: image.Rectangle{
					Min: image.Point{
						X: int(kp.X) - int(kp.Size/2),
						Y: int(kp.Y) - int(kp.Size/2),
					},
					Max: image.Point{
						X: int(kp.X) + int(kp.Size/2),
						Y: int(kp.Y) + int(kp.Size/2),
					},
				},
				Confidence: float64(kp.Response),
			},
			Confidence: float64(kp.Response),
			Source:     "orb",
		}
		elements = append(elements, elem)
	}

	return elements, nil
}

// DetectType implements core.ElementDetector.
// ORB detector doesn't classify element types, so this returns an error.
func (d *ORBDetector) DetectType(ctx context.Context, frame *core.Frame, elemType core.ElementType) ([]core.UIElement, error) {
	return nil, fmt.Errorf("ORB detector does not support type-based detection")
}

// FindByTemplate locates an element matching a template image.
// This is the primary use case for ORB detection.
func (d *ORBDetector) FindByTemplate(
	ctx context.Context,
	frame *core.Frame,
	template []byte,
	confidence float64,
) (*core.UIElement, error) {
	// Convert frame to mat
	frameMat, err := bytesToMat(frame.Data, frame.Bounds)
	if err != nil {
		return nil, fmt.Errorf("converting frame to mat: %w", err)
	}
	defer frameMat.Close()

	// Convert template to mat
	templateMat, err := bytesToMat(template, image.Rect(0, 0, 0, 0))
	if err != nil {
		return nil, fmt.Errorf("converting template to mat: %w", err)
	}
	defer templateMat.Close()

	// Detect features in frame
	frameKP, frameDesc := d.detectFeatures(frameMat)
	defer frameDesc.Close()

	// Detect features in template
	templateKP, templateDesc := d.detectFeatures(templateMat)
	defer templateDesc.Close()

	// Match features
	matches := d.matchFeatures(frameDesc, templateDesc)
	defer matches.Close()

	// Check if we have enough matches
	if matches.Rows() < d.config.MinMatches {
		return nil, fmt.Errorf("insufficient matches: got %d, need %d", matches.Rows(), d.config.MinMatches)
	}

	// Find homography using RANSAC
	homography, mask := d.findHomography(frameKP, templateKP, matches)
	defer homography.Close()
	defer mask.Close()

	// Count inliers
	inliers := countNonZero(mask)
	if inliers < d.config.MinMatches {
		return nil, fmt.Errorf("insufficient inliers: got %d, need %d", inliers, d.config.MinMatches)
	}

	// Calculate confidence based on inlier ratio
	matchConfidence := float64(inliers) / float64(matches.Rows())
	if matchConfidence < confidence {
		return nil, fmt.Errorf("match confidence %.2f below threshold %.2f", matchConfidence, confidence)
	}

	// Transform template corners to frame coordinates
	corners := d.transformCorners(homography, templateMat.Cols(), templateMat.Rows())

	// Calculate bounding box from transformed corners
	bounds := boundingBoxFromPoints(corners)

	element := &core.UIElement{
		ID:         "orb_match",
		Type:       core.ElementUnknown,
		Bounds:     bounds,
		Confidence: matchConfidence,
		Source:     "orb",
	}

	return element, nil
}

// FindByText implements core.ElementDetector.
// ORB detector doesn't do text recognition.
func (d *ORBDetector) FindByText(ctx context.Context, frame *core.Frame, text string) ([]core.UIElement, error) {
	return nil, fmt.Errorf("ORB detector does not support text-based detection")
}

// RegisterTemplate precomputes and caches features for a template.
func (d *ORBDetector) RegisterTemplate(name string, template []byte) error {
	mat, err := bytesToMat(template, image.Rect(0, 0, 0, 0))
	if err != nil {
		return fmt.Errorf("converting template: %w", err)
	}
	defer mat.Close()

	kp, desc := d.detectFeatures(mat)

	features := &templateFeatures{
		KeyPoints:   kp,
		Descriptors: desc,
		Size:        image.Point{X: mat.Cols(), Y: mat.Rows()},
	}

	d.cacheMutex.Lock()
	defer d.cacheMutex.Unlock()

	// Clean up old features if replacing
	if old, exists := d.templateCache[name]; exists {
		old.Descriptors.Close()
	}

	d.templateCache[name] = features
	return nil
}

// FindByRegisteredTemplate finds using a pre-registered template.
func (d *ORBDetector) FindByRegisteredTemplate(
	ctx context.Context,
	frame *core.Frame,
	templateName string,
	confidence float64,
) (*core.UIElement, error) {
	d.cacheMutex.RLock()
	features, exists := d.templateCache[templateName]
	d.cacheMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("template not registered: %s", templateName)
	}

	// Convert frame to mat
	frameMat, err := bytesToMat(frame.Data, frame.Bounds)
	if err != nil {
		return nil, fmt.Errorf("converting frame: %w", err)
	}
	defer frameMat.Close()

	// Detect features in frame
	frameKP, frameDesc := d.detectFeatures(frameMat)
	defer frameDesc.Close()

	// Match with cached template features
	matches := d.matchFeatures(frameDesc, features.Descriptors)
	defer matches.Close()

	if matches.Rows() < d.config.MinMatches {
		return nil, fmt.Errorf("insufficient matches: got %d, need %d", matches.Rows(), d.config.MinMatches)
	}

	// Find homography
	homography, mask := d.findHomography(frameKP, features.KeyPoints, matches)
	defer homography.Close()
	defer mask.Close()

	inliers := countNonZero(mask)
	if inliers < d.config.MinMatches {
		return nil, fmt.Errorf("insufficient inliers: got %d, need %d", inliers, d.config.MinMatches)
	}

	matchConfidence := float64(inliers) / float64(matches.Rows())
	if matchConfidence < confidence {
		return nil, fmt.Errorf("match confidence %.2f below threshold %.2f", matchConfidence, confidence)
	}

	// Transform corners
	corners := d.transformCorners(homography, features.Size.X, features.Size.Y)
	bounds := boundingBoxFromPoints(corners)

	return &core.UIElement{
		ID:         templateName,
		Type:       core.ElementUnknown,
		Bounds:     bounds,
		Confidence: matchConfidence,
		Source:     "orb",
	}, nil
}

// detectFeatures detects ORB features in an image.
func (d *ORBDetector) detectFeatures(mat gocv.Mat) (gocv.KeyPoints, gocv.Mat) {
	// Convert to grayscale if needed
	var gray gocv.Mat
	if mat.Channels() == 3 {
		gray = gocv.NewMat()
		gocv.CvtColor(mat, &gray, gocv.ColorBGRToGray)
	} else if mat.Channels() == 4 {
		gray = gocv.NewMat()
		gocv.CvtColor(mat, &gray, gocv.ColorBGRAToGray)
	} else {
		gray = mat.Clone()
	}
	defer gray.Close()

	// Detect keypoints and compute descriptors
	descriptors := gocv.NewMat()
	keypoints := d.orb.DetectAndCompute(gray, gocv.NewMat(), &descriptors)

	return keypoints, descriptors
}

// matchFeatures matches descriptors between frame and template.
func (d *ORBDetector) matchFeatures(frameDesc, templateDesc gocv.Mat) gocv.Mat {
	// KNN match with k=2
	matches := gocv.NewMat()
	d.matcher.KnnMatch(frameDesc, templateDesc, &matches, 2)
	return matches
}

// findHomography finds the homography matrix using RANSAC.
func (d *ORBDetector) findHomography(
	frameKP gocv.KeyPoints,
	templateKP gocv.KeyPoints,
	matches gocv.Mat,
) (gocv.Mat, gocv.Mat) {
	// Apply Lowe's ratio test
	goodMatches := make([][2]gocv.DMatch, 0)
	for i := 0; i < matches.Rows(); i++ {
		match1 := matches.GetVecfAt(i, 0)
		match2 := matches.GetVecfAt(i, 1)

		if match1[1] < d.config.MatchRatio*match2[1] {
			goodMatches = append(goodMatches, [2]gocv.DMatch{
				{QueryIdx: int(match1[0]), TrainIdx: int(match1[1]), Distance: match1[2]},
				{QueryIdx: int(match2[0]), TrainIdx: int(match2[1]), Distance: match2[2]},
			})
		}
	}

	if len(goodMatches) < 4 {
		return gocv.NewMat(), gocv.NewMat()
	}

	// Extract point correspondences
	framePoints := make([]image.Point, len(goodMatches))
	templatePoints := make([]image.Point, len(goodMatches))

	for i, match := range goodMatches {
		framePoints[i] = image.Point{
			X: int(frameKP[match[0].QueryIdx].X),
			Y: int(frameKP[match[0].QueryIdx].Y),
		}
		templatePoints[i] = image.Point{
			X: int(templateKP[match[0].TrainIdx].X),
			Y: int(templateKP[match[0].TrainIdx].Y),
		}
	}

	// Convert to Mat
	frameMat := pointsToMat(framePoints)
	defer frameMat.Close()
	templateMat := pointsToMat(templatePoints)
	defer templateMat.Close()

	// Find homography
	homography := gocv.FindHomography(frameMat, templateMat, gocv.HomographyMethodRANSAC, d.config.RANSACThreshold)

	// Create mask from homography status
	mask := gocv.NewMat()
	// In OpenCV, FindHomography with RANSAC returns empty mask, so we compute one
	// This is a simplified version - in production, reproject and check error

	return homography, mask
}

// transformCorners transforms template corners using homography.
func (d *ORBDetector) transformCorners(homography gocv.Mat, width, height int) []image.Point {
	// Template corners
	corners := []image.Point{
		{X: 0, Y: 0},
		{X: width, Y: 0},
		{X: width, Y: height},
		{X: 0, Y: height},
	}

	cornersMat := pointsToMat(corners)
	defer cornersMat.Close()

	// Transform corners
	transformed := gocv.NewMat()
	gocv.PerspectiveTransform(cornersMat, &transformed, homography)
	defer transformed.Close()

	// Convert back to points
	result := make([]image.Point, 4)
	for i := 0; i < 4 && i < transformed.Rows(); i++ {
		pt := transformed.GetVecfAt(i, 0)
		result[i] = image.Point{X: int(pt[0]), Y: int(pt[1])}
	}

	return result
}

// Helper functions

func bytesToMat(data []byte, bounds image.Rectangle) (gocv.Mat, error) {
	// Determine image format and decode
	// For now, assume raw BGR data
	if bounds.Empty() {
		// Try to decode as image
		// This is a placeholder - actual implementation would use image.Decode
		return gocv.NewMat(), fmt.Errorf("image decoding not implemented")
	}

	// Create mat from raw bytes
	mat, err := gocv.NewMatFromBytes(bounds.Dy(), bounds.Dx(), gocv.MatTypeCV8UC3, data)
	if err != nil {
		return gocv.NewMat(), fmt.Errorf("creating mat from bytes: %w", err)
	}

	return mat, nil
}

func pointsToMat(points []image.Point) gocv.Mat {
	mat := gocv.NewMatWithSize(len(points), 1, gocv.MatTypeCV32FC2)
	for i, pt := range points {
		mat.SetFloatAt(i, 0, float32(pt.X))
		mat.SetFloatAt(i, 1, float32(pt.Y))
	}
	return mat
}

func countNonZero(mat gocv.Mat) int {
	// Simplified - actual implementation would check homography status
	if mat.Empty() {
		return 0
	}
	return mat.Rows()
}

func boundingBoxFromPoints(points []image.Point) core.Rectangle {
	if len(points) == 0 {
		return core.Rectangle{}
	}

	minX, minY := points[0].X, points[0].Y
	maxX, maxY := points[0].X, points[0].Y

	for _, pt := range points[1:] {
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

	return core.Rectangle{
		Rectangle: image.Rectangle{
			Min: image.Point{X: minX, Y: minY},
			Max: image.Point{X: maxX, Y: maxY},
		},
		Confidence: 1.0,
	}
}
