// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package visual provides screenshot comparison capabilities
// for dual-display testing. It extends the existing evidence
// and screenshot system with pixel-level image analysis,
// black-screen detection, and display-state classification.
package visual

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
)

// defaultTolerance is the per-channel color difference
// allowed when comparing two pixels. Values within this
// range are considered matching (accounts for JPEG
// artifacts, color profile rounding, etc.).
const defaultTolerance = 10

// defaultBlackThreshold is the luminance value below which
// a pixel is considered "dark". Used by IsBlack.
const defaultBlackThreshold = 30

// defaultBlackPercent is the minimum percentage of dark
// pixels required for IsBlack to return true.
const defaultBlackPercent = 0.95

// minRegionPixels is the minimum number of diff pixels in
// a connected region for it to be reported. Filters noise.
const minRegionPixels = 4

// Region describes a rectangular area of an image where
// differences were detected.
type Region struct {
	// X is the left coordinate of the region.
	X int `json:"x"`
	// Y is the top coordinate of the region.
	Y int `json:"y"`
	// Width is the region width in pixels.
	Width int `json:"width"`
	// Height is the region height in pixels.
	Height int `json:"height"`
}

// Area returns the area of the region.
func (r Region) Area() int {
	if r.Width <= 0 || r.Height <= 0 {
		return 0
	}
	return r.Width * r.Height
}

// ComparisonResult holds the outcome of comparing two
// images pixel by pixel.
type ComparisonResult struct {
	// Match is true when all pixels are within tolerance.
	Match bool `json:"match"`

	// Similarity is the fraction of matching pixels (0-1).
	Similarity float64 `json:"similarity"`

	// DiffRegions lists rectangular bounding boxes around
	// clusters of differing pixels.
	DiffRegions []Region `json:"diff_regions,omitempty"`

	// DiffPixelCount is the total number of pixels that
	// differ beyond tolerance.
	DiffPixelCount int `json:"diff_pixel_count"`

	// Width is the image width used for comparison.
	Width int `json:"width"`

	// Height is the image height used for comparison.
	Height int `json:"height"`
}

// DisplayState describes what a single display is showing.
type DisplayState string

const (
	// DisplayStateBlack indicates the display is blank or
	// powered off (>95% dark pixels).
	DisplayStateBlack DisplayState = "black"

	// DisplayStateContent indicates the display has visible
	// content (video, UI, etc.).
	DisplayStateContent DisplayState = "content"

	// DisplayStateUnknown indicates the state could not be
	// determined.
	DisplayStateUnknown DisplayState = "unknown"
)

// DisplayComparisonResult captures the visual state of both
// displays in a dual-display setup.
type DisplayComparisonResult struct {
	// PrimaryState describes the primary display.
	PrimaryState DisplayState `json:"primary_state"`

	// SecondaryState describes the secondary display.
	SecondaryState DisplayState `json:"secondary_state"`

	// SecondaryHasVideo is true when the secondary display
	// shows non-black content (video playing on TV).
	SecondaryHasVideo bool `json:"secondary_has_video"`

	// PrimaryHasOverlay is true when the primary display
	// has the VideoControlOverlay visible (non-black region
	// in the expected overlay area).
	PrimaryHasOverlay bool `json:"primary_has_overlay"`

	// PrimaryBrightness is the mean luminance of the
	// primary display (0-255).
	PrimaryBrightness float64 `json:"primary_brightness"`

	// SecondaryBrightness is the mean luminance of the
	// secondary display (0-255).
	SecondaryBrightness float64 `json:"secondary_brightness"`
}

// Option configures a ScreenshotComparator.
type Option func(*ScreenshotComparator)

// WithTolerance sets the per-channel color tolerance for
// pixel comparison. Default is 10.
func WithTolerance(t int) Option {
	return func(sc *ScreenshotComparator) {
		if t >= 0 {
			sc.tolerance = t
		}
	}
}

// WithBlackThreshold sets the luminance threshold below
// which a pixel is considered dark. Default is 30.
func WithBlackThreshold(t int) Option {
	return func(sc *ScreenshotComparator) {
		if t >= 0 && t <= 255 {
			sc.blackThreshold = t
		}
	}
}

// WithBlackPercent sets the minimum dark-pixel fraction
// for IsBlack (0-1). Default is 0.95.
func WithBlackPercent(p float64) Option {
	return func(sc *ScreenshotComparator) {
		if p >= 0.0 && p <= 1.0 {
			sc.blackPercent = p
		}
	}
}

// ScreenshotComparator compares PNG screenshots for
// dual-display testing.
type ScreenshotComparator struct {
	tolerance      int
	blackThreshold int
	blackPercent   float64
}

// NewScreenshotComparator creates a comparator with the
// given options.
func NewScreenshotComparator(
	opts ...Option,
) *ScreenshotComparator {
	sc := &ScreenshotComparator{
		tolerance:      defaultTolerance,
		blackThreshold: defaultBlackThreshold,
		blackPercent:   defaultBlackPercent,
	}
	for _, opt := range opts {
		opt(sc)
	}
	return sc
}

// Compare performs a pixel-by-pixel comparison of two PNG
// images. It returns the similarity ratio, diff pixel
// count, and bounding-box regions of differences.
func (sc *ScreenshotComparator) Compare(
	_ context.Context,
	before, after []byte,
) (*ComparisonResult, error) {
	imgA, err := decodePNG(before)
	if err != nil {
		return nil, fmt.Errorf("decode before image: %w", err)
	}
	imgB, err := decodePNG(after)
	if err != nil {
		return nil, fmt.Errorf("decode after image: %w", err)
	}

	boundsA := imgA.Bounds()
	boundsB := imgB.Bounds()

	if boundsA.Dx() != boundsB.Dx() ||
		boundsA.Dy() != boundsB.Dy() {
		return nil, fmt.Errorf(
			"dimension mismatch: %dx%d vs %dx%d",
			boundsA.Dx(), boundsA.Dy(),
			boundsB.Dx(), boundsB.Dy(),
		)
	}

	w := boundsA.Dx()
	h := boundsA.Dy()
	totalPixels := w * h

	if totalPixels == 0 {
		return &ComparisonResult{
			Match:      true,
			Similarity: 1.0,
			Width:      w,
			Height:     h,
		}, nil
	}

	// Build a boolean diff mask.
	diffMask := make([]bool, totalPixels)
	diffCount := 0

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			cA := imgA.At(
				x+boundsA.Min.X, y+boundsA.Min.Y,
			)
			cB := imgB.At(
				x+boundsB.Min.X, y+boundsB.Min.Y,
			)
			if !sc.colorsMatch(cA, cB) {
				diffMask[y*w+x] = true
				diffCount++
			}
		}
	}

	similarity := 1.0 - float64(diffCount)/
		float64(totalPixels)

	result := &ComparisonResult{
		Match:          diffCount == 0,
		Similarity:     similarity,
		DiffPixelCount: diffCount,
		Width:          w,
		Height:         h,
	}

	if diffCount > 0 {
		result.DiffRegions = findDiffRegions(
			diffMask, w, h,
		)
	}

	return result, nil
}

// IsBlack checks whether an image is mostly black (dark).
// Returns true if more than blackPercent of pixels have
// luminance below blackThreshold. Useful for detecting a
// blank secondary display.
func (sc *ScreenshotComparator) IsBlack(
	_ context.Context,
	data []byte,
) (bool, error) {
	img, err := decodePNG(data)
	if err != nil {
		return false, fmt.Errorf("decode image: %w", err)
	}

	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return true, nil
	}

	darkCount := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if sc.isDark(img.At(x, y)) {
				darkCount++
			}
		}
	}

	ratio := float64(darkCount) / float64(total)
	return ratio >= sc.blackPercent, nil
}

// HasContent checks whether an image has visible content
// (not mostly black). This is the inverse of IsBlack and
// is useful for detecting video playing on the secondary
// display.
func (sc *ScreenshotComparator) HasContent(
	ctx context.Context,
	data []byte,
) (bool, error) {
	black, err := sc.IsBlack(ctx, data)
	if err != nil {
		return false, err
	}
	return !black, nil
}

// CompareDisplays analyzes primary and secondary display
// screenshots for a dual-display setup. It determines
// whether the secondary is showing video content and
// whether the primary has the VideoControlOverlay.
func (sc *ScreenshotComparator) CompareDisplays(
	ctx context.Context,
	primary, secondary []byte,
) (*DisplayComparisonResult, error) {
	result := &DisplayComparisonResult{
		PrimaryState:  DisplayStateUnknown,
		SecondaryState: DisplayStateUnknown,
	}

	// Analyze secondary display.
	secBlack, err := sc.IsBlack(ctx, secondary)
	if err != nil {
		return nil, fmt.Errorf(
			"analyze secondary: %w", err,
		)
	}
	if secBlack {
		result.SecondaryState = DisplayStateBlack
		result.SecondaryHasVideo = false
	} else {
		result.SecondaryState = DisplayStateContent
		result.SecondaryHasVideo = true
	}

	// Analyze primary display.
	priBlack, err := sc.IsBlack(ctx, primary)
	if err != nil {
		return nil, fmt.Errorf(
			"analyze primary: %w", err,
		)
	}
	if priBlack {
		result.PrimaryState = DisplayStateBlack
	} else {
		result.PrimaryState = DisplayStateContent
	}

	// Calculate brightness for both displays.
	priImg, err := decodePNG(primary)
	if err != nil {
		return nil, fmt.Errorf(
			"decode primary for brightness: %w", err,
		)
	}
	result.PrimaryBrightness = meanLuminance(priImg)

	secImg, err := decodePNG(secondary)
	if err != nil {
		return nil, fmt.Errorf(
			"decode secondary for brightness: %w", err,
		)
	}
	result.SecondaryBrightness = meanLuminance(secImg)

	// Check for overlay on primary. The VideoControlOverlay
	// appears as a floating button/panel. When the primary
	// is mostly black (Presenter background) but has a small
	// bright region, that indicates the overlay is visible.
	if priBlack {
		result.PrimaryHasOverlay = false
	} else {
		result.PrimaryHasOverlay = sc.detectOverlay(priImg)
	}

	return result, nil
}

// colorsMatch returns true if two colors are within the
// configured tolerance on all channels.
func (sc *ScreenshotComparator) colorsMatch(
	a, b color.Color,
) bool {
	rA, gA, bA, aA := a.RGBA()
	rB, gB, bB, aB := b.RGBA()

	// RGBA() returns 16-bit values; scale to 8-bit.
	tol := uint32(sc.tolerance) * 257
	return absDiff(rA, rB) <= tol &&
		absDiff(gA, gB) <= tol &&
		absDiff(bA, bB) <= tol &&
		absDiff(aA, aB) <= tol
}

// isDark returns true if the pixel luminance is below the
// black threshold.
func (sc *ScreenshotComparator) isDark(
	c color.Color,
) bool {
	r, g, b, _ := c.RGBA()
	// BT.601 luminance (scaled to 16-bit range).
	lum := 0.299*float64(r) + 0.587*float64(g) +
		0.114*float64(b)
	// Scale back to 8-bit for threshold comparison.
	lum8 := lum / 257.0
	return lum8 < float64(sc.blackThreshold)
}

// detectOverlay checks whether the primary display has a
// VideoControlOverlay. The overlay is a small bright region
// on an otherwise dark background. We check whether the
// image has a mix of dark and non-dark pixels where the
// non-dark fraction is small (5-50% of the image).
func (sc *ScreenshotComparator) detectOverlay(
	img image.Image,
) bool {
	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return false
	}

	brightCount := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !sc.isDark(img.At(x, y)) {
				brightCount++
			}
		}
	}

	brightFrac := float64(brightCount) / float64(total)
	// Overlay is a small UI element (button/panel) on a
	// mostly dark background. Between 1% and 50% bright
	// pixels suggests overlay presence.
	return brightFrac >= 0.01 && brightFrac <= 0.50
}

// decodePNG decodes PNG data into an image.Image.
func decodePNG(data []byte) (image.Image, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty image data")
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("png decode: %w", err)
	}
	return img, nil
}

// meanLuminance calculates the average BT.601 luminance
// of all pixels in the image (0-255 scale).
func meanLuminance(img image.Image) float64 {
	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return 0
	}

	var sum float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			lum := 0.299*float64(r) +
				0.587*float64(g) +
				0.114*float64(b)
			sum += lum / 257.0
		}
	}
	return sum / float64(total)
}

// absDiff returns the absolute difference of two uint32
// values.
func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}

// findDiffRegions groups adjacent diff pixels into
// rectangular bounding boxes using a simple grid-based
// clustering approach. The image is divided into cells and
// neighboring cells with diffs are merged into regions.
func findDiffRegions(
	mask []bool, w, h int,
) []Region {
	// Use a grid cell size proportional to image size
	// (minimum 8px, maximum 64px).
	cellSize := w / 20
	if cellSize < 8 {
		cellSize = 8
	}
	if cellSize > 64 {
		cellSize = 64
	}

	gridW := (w + cellSize - 1) / cellSize
	gridH := (h + cellSize - 1) / cellSize

	// Count diff pixels per grid cell.
	cellDiffs := make([]int, gridW*gridH)
	for y := 0; y < h; y++ {
		gy := y / cellSize
		for x := 0; x < w; x++ {
			if mask[y*w+x] {
				gx := x / cellSize
				cellDiffs[gy*gridW+gx]++
			}
		}
	}

	// Mark cells with enough diffs as active.
	active := make([]bool, gridW*gridH)
	for i, count := range cellDiffs {
		if count >= minRegionPixels {
			active[i] = true
		}
	}

	// Flood-fill to find connected components of active
	// cells and compute their bounding boxes.
	visited := make([]bool, gridW*gridH)
	var regions []Region

	for gy := 0; gy < gridH; gy++ {
		for gx := 0; gx < gridW; gx++ {
			idx := gy*gridW + gx
			if !active[idx] || visited[idx] {
				continue
			}

			// BFS to find connected active cells.
			minGX, maxGX := gx, gx
			minGY, maxGY := gy, gy
			queue := []int{idx}
			visited[idx] = true

			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]

				cy := cur / gridW
				cx := cur % gridW

				if cx < minGX {
					minGX = cx
				}
				if cx > maxGX {
					maxGX = cx
				}
				if cy < minGY {
					minGY = cy
				}
				if cy > maxGY {
					maxGY = cy
				}

				// Check 4-connected neighbors.
				neighbors := [][2]int{
					{cx - 1, cy},
					{cx + 1, cy},
					{cx, cy - 1},
					{cx, cy + 1},
				}
				for _, n := range neighbors {
					nx, ny := n[0], n[1]
					if nx < 0 || nx >= gridW ||
						ny < 0 || ny >= gridH {
						continue
					}
					ni := ny*gridW + nx
					if active[ni] && !visited[ni] {
						visited[ni] = true
						queue = append(queue, ni)
					}
				}
			}

			// Convert grid bounds to pixel bounds.
			px := minGX * cellSize
			py := minGY * cellSize
			pw := (maxGX+1)*cellSize - px
			ph := (maxGY+1)*cellSize - py

			// Clamp to image bounds.
			if px+pw > w {
				pw = w - px
			}
			if py+ph > h {
				ph = h - py
			}

			regions = append(regions, Region{
				X:      px,
				Y:      py,
				Width:  pw,
				Height: ph,
			})
		}
	}

	return regions
}

// Luminance returns the BT.601 luminance of a color on a
// 0-255 scale.
func Luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	lum := 0.299*float64(r) + 0.587*float64(g) +
		0.114*float64(b)
	return math.Round(lum / 257.0)
}
