// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package regression

import (
	"errors"
	"image"
	"image/color"
	"math"
)

// DiffOptions controls the pixelmatch behaviour. Zero values → sensible
// defaults (Threshold=0.1, IncludeAA=false, DiffColor=red, AAColor=yellow,
// AlphaBlend=0.1).
type DiffOptions struct {
	// Threshold is the maximum allowed YIQ color delta (0..1) before two
	// pixels are considered "different". Lower = more sensitive.
	Threshold float64

	// IncludeAA — if false (default), anti-aliased pixels are flagged
	// separately (colored AAColor in the diff output) and NOT counted
	// towards the DiffCount. If true, AA pixels count as differences.
	IncludeAA bool

	// DiffColor is the color for non-AA differing pixels in the diff
	// output. Default: opaque red (255, 0, 0).
	DiffColor color.RGBA

	// AAColor is the color for anti-aliased pixels in the diff output
	// (only used when IncludeAA=false). Default: opaque yellow (255, 255, 0).
	AAColor color.RGBA

	// AlphaBlend is the opacity (0..1) of the original image rendered
	// underneath the diff overlay — 0 = black background, 1 = full
	// original. Default: 0.1.
	AlphaBlend float64
}

// DiffReport is the outcome of a Diff call.
type DiffReport struct {
	// Output is an RGBA image of the same dimensions as the inputs, with:
	//   - unchanged pixels: blended grayscale of A at AlphaBlend opacity
	//   - anti-aliased pixels: AAColor (unless IncludeAA=true)
	//   - differing pixels: DiffColor
	Output *image.RGBA

	// DiffCount is the number of pixels that differ beyond Threshold,
	// EXCLUDING anti-aliased pixels (unless IncludeAA=true).
	DiffCount int

	// AACount is the number of anti-aliased pixels detected. Useful for
	// diagnostics even when IncludeAA=true.
	AACount int

	// TotalPixels is Width × Height of the compared region.
	TotalPixels int
}

// Differ is the exported interface advertised in doc.go.
type Differ interface {
	Diff(a, b image.Image, opts DiffOptions) (DiffReport, error)
}

// PixelMatch is the Differ implementation — a port of mapbox/pixelmatch
// (MIT-licensed) with YIQ color-space deltas and Smith-2009 anti-aliasing
// detection.
type PixelMatch struct{}

// Sentinel errors.
var (
	ErrNilImage         = errors.New("helixqa/regression: nil image")
	ErrDimensionMismatch = errors.New("helixqa/regression: image dimensions differ")
)

// Diff compares two images pixel-by-pixel and returns a DiffReport. The
// two images must have identical bounds dimensions; their Min corners
// may differ (the algorithm works in each image's local coordinate
// system).
func (PixelMatch) Diff(a, b image.Image, opts DiffOptions) (DiffReport, error) {
	if a == nil || b == nil {
		return DiffReport{}, ErrNilImage
	}
	ba, bb := a.Bounds(), b.Bounds()
	if ba.Dx() != bb.Dx() || ba.Dy() != bb.Dy() {
		return DiffReport{}, ErrDimensionMismatch
	}
	w, h := ba.Dx(), ba.Dy()
	if w == 0 || h == 0 {
		return DiffReport{Output: image.NewRGBA(image.Rect(0, 0, 0, 0))}, nil
	}

	opts = opts.withDefaults()
	maxDelta := 35215.0 * opts.Threshold * opts.Threshold

	out := image.NewRGBA(image.Rect(0, 0, w, h))
	var diffCount, aaCount int

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			ra, ga, baCh, aa := rgbaAt(a, ba.Min.X+x, ba.Min.Y+y)
			rb, gb, bbCh, ab := rgbaAt(b, bb.Min.X+x, bb.Min.Y+y)

			delta := colorDelta(ra, ga, baCh, aa, rb, gb, bbCh, ab, false)

			if math.Abs(delta) > maxDelta {
				// Pixels differ. Check whether the difference is caused
				// by anti-aliasing (in either image).
				if !opts.IncludeAA && (isAA(a, ba, x, y, w, h) || isAA(b, bb, x, y, w, h)) {
					out.SetRGBA(x, y, opts.AAColor)
					aaCount++
				} else {
					out.SetRGBA(x, y, opts.DiffColor)
					diffCount++
				}
			} else {
				// Pixels match — render as a blended grayscale of a.
				out.SetRGBA(x, y, blendGray(ra, ga, baCh, aa, opts.AlphaBlend))
			}
		}
	}

	return DiffReport{
		Output:      out,
		DiffCount:   diffCount,
		AACount:     aaCount,
		TotalPixels: w * h,
	}, nil
}

func (o DiffOptions) withDefaults() DiffOptions {
	if o.Threshold == 0 {
		o.Threshold = 0.1
	}
	if (o.DiffColor == color.RGBA{}) {
		o.DiffColor = color.RGBA{R: 255, A: 255}
	}
	if (o.AAColor == color.RGBA{}) {
		o.AAColor = color.RGBA{R: 255, G: 255, A: 255}
	}
	if o.AlphaBlend == 0 {
		o.AlphaBlend = 0.1
	}
	return o
}

// rgbaAt extracts 8-bit RGBA channels from an image at point (x,y).
// Handles the 16-bit native channels of color.Color via the >>8 shift.
func rgbaAt(img image.Image, x, y int) (r, g, b, a uint8) {
	rc, gc, bc, ac := img.At(x, y).RGBA()
	return uint8(rc >> 8), uint8(gc >> 8), uint8(bc >> 8), uint8(ac >> 8)
}

// colorDelta returns the squared YIQ difference between two pixels.
// The YIQ weighting is perceptually tuned — it gives much more weight
// to luma (Y) than to chroma (I, Q), matching human visual sensitivity.
//
// If yOnly is true, only the Y component is returned (used inside isAA).
func colorDelta(r1, g1, b1, a1, r2, g2, b2, a2 uint8, yOnly bool) float64 {
	if r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2 {
		return 0
	}

	fr1, fg1, fb1 := blendWhite(r1, g1, b1, a1)
	fr2, fg2, fb2 := blendWhite(r2, g2, b2, a2)

	y := rgb2y(fr1, fg1, fb1) - rgb2y(fr2, fg2, fb2)
	if yOnly {
		return y
	}
	i := rgb2i(fr1, fg1, fb1) - rgb2i(fr2, fg2, fb2)
	q := rgb2q(fr1, fg1, fb1) - rgb2q(fr2, fg2, fb2)
	return 0.5053*y*y + 0.299*i*i + 0.1957*q*q
}

// blendWhite alpha-premultiplies over a white background — pixelmatch's
// standard pre-processing to normalize translucent pixels against a
// consistent background.
func blendWhite(r, g, b, a uint8) (fr, fg, fb float64) {
	af := float64(a) / 255
	fr = 255 + (float64(r)-255)*af
	fg = 255 + (float64(g)-255)*af
	fb = 255 + (float64(b)-255)*af
	return
}

func rgb2y(r, g, b float64) float64 { return r*0.29889531 + g*0.58662247 + b*0.11448223 }
func rgb2i(r, g, b float64) float64 { return r*0.59597799 - g*0.27417610 - b*0.32180189 }
func rgb2q(r, g, b float64) float64 { return r*0.21147017 - g*0.52261711 + b*0.31114694 }

// blendGray renders an RGBA pixel as a grayscale blend over black at
// opacity alphaBlend. Used to render unchanged pixels in the diff
// output (matches pixelmatch behaviour).
func blendGray(r, g, b, a uint8, alphaBlend float64) color.RGBA {
	af := float64(a) / 255 * alphaBlend
	// Render the input pixel as grayscale luma, then blend over black.
	gray := rgb2y(float64(r), float64(g), float64(b))
	v := uint8(255 + (gray-255)*af)
	return color.RGBA{R: v, G: v, B: v, A: 255}
}

// isAA detects whether (x,y) in img is an anti-aliased pixel per
// Smith's 2009 definition: a pixel is AA iff
//   - it has at least one 8-neighbor with zero color delta AND at least
//     three that differ significantly
//   - it is NOT the brightest or darkest Y-luma among its siblings
//
// This matches the mapbox/pixelmatch implementation.
func isAA(img image.Image, bounds image.Rectangle, x, y, w, h int) bool {
	x0 := x - 1
	if x0 < 0 {
		x0 = 0
	}
	y0 := y - 1
	if y0 < 0 {
		y0 = 0
	}
	x1 := x + 1
	if x1 >= w {
		x1 = w - 1
	}
	y1 := y + 1
	if y1 >= h {
		y1 = h - 1
	}

	r0, g0, b0, a0 := rgbaAt(img, bounds.Min.X+x, bounds.Min.Y+y)

	var zeroes int
	if x == x0 || x == x1 || y == y0 || y == y1 {
		zeroes = 1 // the "beyond the edge" neighbor counts as a zero
	}
	var minY, maxY float64
	var minX, minYY, maxX, maxYY int

	for adjY := y0; adjY <= y1; adjY++ {
		for adjX := x0; adjX <= x1; adjX++ {
			if adjX == x && adjY == y {
				continue
			}
			r, g, b, a := rgbaAt(img, bounds.Min.X+adjX, bounds.Min.Y+adjY)
			delta := colorDelta(r0, g0, b0, a0, r, g, b, a, true)
			switch {
			case delta == 0:
				zeroes++
				if zeroes > 2 {
					return false
				}
			case delta < minY:
				minY = delta
				minX, minYY = adjX, adjY
			case delta > maxY:
				maxY = delta
				maxX, maxYY = adjX, adjY
			}
		}
	}

	if minY == 0 || maxY == 0 {
		return false
	}
	return (hasManySiblings(img, bounds, minX, minYY, w, h) &&
		hasManySiblings(img, bounds, maxX, maxYY, w, h))
}

// hasManySiblings returns true if (x,y) has ≥ 3 neighbors with the same
// color. Used by isAA to confirm the suspect neighbors are themselves
// part of the AA gradient rather than stand-alone features.
func hasManySiblings(img image.Image, bounds image.Rectangle, x, y, w, h int) bool {
	x0 := x - 1
	if x0 < 0 {
		x0 = 0
	}
	y0 := y - 1
	if y0 < 0 {
		y0 = 0
	}
	x1 := x + 1
	if x1 >= w {
		x1 = w - 1
	}
	y1 := y + 1
	if y1 >= h {
		y1 = h - 1
	}

	r0, g0, b0, a0 := rgbaAt(img, bounds.Min.X+x, bounds.Min.Y+y)
	var zeroes int
	if x == x0 || x == x1 || y == y0 || y == y1 {
		zeroes = 1
	}

	for adjY := y0; adjY <= y1; adjY++ {
		for adjX := x0; adjX <= x1; adjX++ {
			if adjX == x && adjY == y {
				continue
			}
			r, g, b, a := rgbaAt(img, bounds.Min.X+adjX, bounds.Min.Y+adjY)
			if r == r0 && g == g0 && b == b0 && a == a0 {
				zeroes++
			}
			if zeroes > 2 {
				return true
			}
		}
	}
	return false
}

// Compile-time guard.
var _ Differ = PixelMatch{}
