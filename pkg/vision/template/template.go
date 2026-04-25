// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package template

import (
	"context"
	"errors"
	"image"
	"math"
)

// Matcher performs normalized cross-correlation template matching
// on grayscale luminance. The canonical "is this logo / icon / button
// still on screen?" primitive — used for presence confirmation, NOT
// for locating clickable targets (NCC is brittle under scaling and
// anti-aliasing; use the grounding VLM for click target resolution).
//
// Pure-Go implementation. Replaces the doc.go-planned gocv wrapper —
// gocv would require OpenCV dev headers + CGO, while the straight
// O(N·M·w·h) NCC loop is sub-100 ms on commodity CPU for typical
// 1920×1080 haystacks + 50×50 needles.
type Matcher struct {
	// MinScore is the threshold below which Match reports
	// "not found". Default 0.7 — empirically a good cutoff for GUI
	// buttons/icons that may have 1-2px anti-aliasing differences.
	// NCC scores range [-1, 1]; 1 = perfect match.
	MinScore float64
}

// Region identifies a rectangular match within the haystack.
type Region struct {
	// BBox is the needle-sized rectangle anchored at the best match
	// position.
	BBox image.Rectangle

	// Score is the NCC correlation coefficient at the match location.
	// Range [-1, 1]; 1 = pixel-perfect match.
	Score float64
}

// Center returns the center point of the region's bbox — the
// natural click target when grounding.
func (r Region) Center() image.Point {
	return image.Point{
		X: (r.BBox.Min.X + r.BBox.Max.X) / 2,
		Y: (r.BBox.Min.Y + r.BBox.Max.Y) / 2,
	}
}

// Sentinel errors.
var (
	ErrNilImage       = errors.New("helixqa/vision/template: nil image")
	ErrNeedleTooLarge = errors.New("helixqa/vision/template: needle larger than haystack")
	ErrZeroBounds     = errors.New("helixqa/vision/template: image has zero bounds")
)

// Match scans haystack for the single best match to needle via
// normalized cross-correlation. Returns (region, found, error).
// found is false iff the best NCC score falls below MinScore;
// region carries the best location regardless (useful for
// inspecting where the closest candidate was even on a failed
// match).
//
// Respects ctx.Err() between haystack rows.
func (m Matcher) Match(ctx context.Context, haystack, needle image.Image) (Region, bool, error) {
	if haystack == nil || needle == nil {
		return Region{}, false, ErrNilImage
	}
	hb := haystack.Bounds()
	nb := needle.Bounds()
	hw, hh := hb.Dx(), hb.Dy()
	nw, nh := nb.Dx(), nb.Dy()
	if hw == 0 || hh == 0 || nw == 0 || nh == 0 {
		return Region{}, false, ErrZeroBounds
	}
	if nw > hw || nh > hh {
		return Region{}, false, ErrNeedleTooLarge
	}

	minScore := m.MinScore
	if minScore == 0 {
		minScore = 0.7
	}

	hayLuma := rec709Luma(haystack)
	nLuma := rec709Luma(needle)

	// Precompute needle mean + sum-of-squared-deviations.
	var nSum float64
	for _, v := range nLuma {
		nSum += float64(v)
	}
	nMean := nSum / float64(len(nLuma))
	var nSqDev float64
	for _, v := range nLuma {
		d := float64(v) - nMean
		nSqDev += d * d
	}
	if nSqDev == 0 {
		// A uniform needle has no features to correlate against;
		// any solid-color region of the haystack matches perfectly.
		// Return the top-left as the "match" with score 0 (neutral)
		// and found=false to signal this to the caller.
		return Region{BBox: image.Rect(hb.Min.X, hb.Min.Y, hb.Min.X+nw, hb.Min.Y+nh)}, false, nil
	}

	var bestScore float64
	var bestX, bestY int
	bestScore = math.Inf(-1)

	maxX := hw - nw
	maxY := hh - nh

	for y := 0; y <= maxY; y++ {
		if y&0x1F == 0 {
			if err := ctx.Err(); err != nil {
				return Region{}, false, err
			}
		}
		for x := 0; x <= maxX; x++ {
			score := ncc(hayLuma, hw, x, y, nLuma, nw, nh, nMean, nSqDev)
			if score > bestScore {
				bestScore = score
				bestX, bestY = x, y
			}
		}
	}

	region := Region{
		BBox: image.Rect(
			hb.Min.X+bestX,
			hb.Min.Y+bestY,
			hb.Min.X+bestX+nw,
			hb.Min.Y+bestY+nh,
		),
		Score: bestScore,
	}
	return region, bestScore >= minScore, nil
}

// ncc computes the normalized cross-correlation of the haystack
// window rooted at (x, y) against the pre-normalized needle.
// hayLuma is the flat luma buffer of the haystack with row stride
// hw; nLuma has row stride nw; nMean is the precomputed needle mean;
// nSqDev is the precomputed needle sum-of-squared-deviations.
func ncc(hayLuma []uint8, hw, x, y int, nLuma []uint8, nw, nh int, nMean, nSqDev float64) float64 {
	// Compute haystack-window mean.
	var hSum float64
	for dy := 0; dy < nh; dy++ {
		row := (y + dy) * hw
		for dx := 0; dx < nw; dx++ {
			hSum += float64(hayLuma[row+x+dx])
		}
	}
	hMean := hSum / float64(nw*nh)

	// Compute cross-correlation numerator + haystack sq-dev.
	var num, hSqDev float64
	for dy := 0; dy < nh; dy++ {
		row := (y + dy) * hw
		nRow := dy * nw
		for dx := 0; dx < nw; dx++ {
			hVal := float64(hayLuma[row+x+dx]) - hMean
			nVal := float64(nLuma[nRow+dx]) - nMean
			num += hVal * nVal
			hSqDev += hVal * hVal
		}
	}

	if hSqDev == 0 {
		// Uniform haystack window — no correlation signal.
		return 0
	}
	return num / math.Sqrt(hSqDev*nSqDev)
}

// rec709Luma extracts a luma-only buffer from any image.Image.
// Shared with pkg/vision/hash — defined locally to avoid a
// cross-package dependency.
func rec709Luma(img image.Image) []uint8 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]uint8, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			out[y*w+x] = uint8((2126*uint32(r>>8) + 7152*uint32(g>>8) + 722*uint32(bl>>8)) / 10000)
		}
	}
	return out
}
