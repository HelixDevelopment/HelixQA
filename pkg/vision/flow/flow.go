// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"errors"
	"image"
	"math"
)

// Computer is the HelixQA optical-flow Computer that calculates
// sparse per-grid-point velocity vectors between two consecutive
// frames via the Lucas-Kanade algorithm (Lucas & Kanade 1981,
// "An Iterative Image Registration Technique with an Application
// to Stereo Vision").
//
// The sparse output shape — a velocity vector at every Nth pixel
// — is the HelixQA sweet spot: dense flow is overkill for
// scroll/pan/animation detection (we only need the dominant motion
// direction per region), and the O(N²) cost is small on any grid
// coarser than 20 pixels.
//
// Pure Go, CGO-free. Replaces the doc.go-planned DIS / NVOF gocv
// wrapper.
type Computer struct {
	// WindowSize is the odd-sized neighborhood over which each LK
	// velocity is solved. Zero → default 7. Larger windows are more
	// robust to noise but average over larger regions.
	WindowSize int
}

// Vector is a single (From, U, V) triple — (U, V) is the estimated
// per-frame pixel displacement of the feature located at From.
type Vector struct {
	From image.Point
	U    float64
	V    float64
}

// Sentinel errors.
var (
	ErrNilImage        = errors.New("helixqa/vision/flow: nil image")
	ErrDimensionMismatch = errors.New("helixqa/vision/flow: prev and curr must have identical bounds")
	ErrTooSmall        = errors.New("helixqa/vision/flow: images smaller than WindowSize")
	ErrInvalidGrid     = errors.New("helixqa/vision/flow: grid must be ≥ 1")
)

// Sparse computes Lucas-Kanade optical flow at a regular grid of
// points. Returns one Vector per grid point where the LK system is
// well-conditioned; degenerate points (low texture, singular A^T A)
// are silently dropped.
//
// The grid parameter controls spacing — 1 = every pixel (dense),
// 16 = every 16th pixel (sparse enough for scroll detection at
// ~70 points per row on 1080p).
func (c Computer) Sparse(ctx context.Context, prev, curr image.Image, grid int) ([]Vector, error) {
	if prev == nil || curr == nil {
		return nil, ErrNilImage
	}
	pb, cb := prev.Bounds(), curr.Bounds()
	if pb.Dx() != cb.Dx() || pb.Dy() != cb.Dy() {
		return nil, ErrDimensionMismatch
	}
	if grid < 1 {
		return nil, ErrInvalidGrid
	}
	w, h := pb.Dx(), pb.Dy()
	ws := c.WindowSize
	if ws == 0 {
		ws = 7
	}
	if ws%2 == 0 {
		ws++ // force odd
	}
	half := ws / 2
	if w < ws || h < ws {
		return nil, ErrTooSmall
	}

	pLuma := rec709Luma(prev)
	cLuma := rec709Luma(curr)

	// Precompute per-pixel spatial gradients (central differences)
	// and temporal gradient (frame difference).
	ix := make([]float64, w*h)
	iy := make([]float64, w*h)
	it := make([]float64, w*h)
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			i := y*w + x
			ix[i] = (float64(pLuma[i+1]) - float64(pLuma[i-1])) / 2
			iy[i] = (float64(pLuma[i+w]) - float64(pLuma[i-w])) / 2
			it[i] = float64(cLuma[i]) - float64(pLuma[i])
		}
	}

	// Scan the grid, solve LK at each point.
	var out []Vector
	for y := half; y < h-half; y += grid {
		// Cheap per-row ctx check — the grid loop is O(rows/grid)
		// which is small enough to absorb one ctx.Err() call per
		// row without measurable overhead.
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for x := half; x < w-half; x += grid {
			u, v, ok := solveLK(ix, iy, it, w, x, y, half)
			if ok {
				out = append(out, Vector{
					From: image.Point{X: pb.Min.X + x, Y: pb.Min.Y + y},
					U:    u,
					V:    v,
				})
			}
		}
	}
	return out, nil
}

// solveLK solves the 2×2 Lucas-Kanade system at (x, y) over a
// (2·half+1)² window. Returns (u, v, true) on success, (0, 0, false)
// when the structure tensor A^T A is singular (uniform texture).
func solveLK(ix, iy, it []float64, w, x, y, half int) (float64, float64, bool) {
	var sumIxIx, sumIxIy, sumIyIy, sumIxIt, sumIyIt float64
	for dy := -half; dy <= half; dy++ {
		row := (y + dy) * w
		for dx := -half; dx <= half; dx++ {
			i := row + x + dx
			sumIxIx += ix[i] * ix[i]
			sumIxIy += ix[i] * iy[i]
			sumIyIy += iy[i] * iy[i]
			sumIxIt += ix[i] * it[i]
			sumIyIt += iy[i] * it[i]
		}
	}
	det := sumIxIx*sumIyIy - sumIxIy*sumIxIy
	if math.Abs(det) < 1e-6 {
		return 0, 0, false
	}
	// Solve (A^T A) [u v]^T = -A^T b via the closed-form 2×2 inverse.
	u := (-sumIyIy*sumIxIt + sumIxIy*sumIyIt) / det
	v := (sumIxIy*sumIxIt - sumIxIx*sumIyIt) / det
	return u, v, true
}

// Median returns the median (U, V) pair across a slice of vectors
// — the dominant flow direction, robust to outliers. Returns
// (0, 0) for an empty input.
//
// Used to summarize "which way did the screen scroll" given a sparse
// flow field: Median.U > 0 means content moved right, Median.V < 0
// means content moved up (screen scrolled down), etc.
func Median(vectors []Vector) (u, v float64) {
	n := len(vectors)
	if n == 0 {
		return 0, 0
	}
	us := make([]float64, n)
	vs := make([]float64, n)
	for i, w := range vectors {
		us[i] = w.U
		vs[i] = w.V
	}
	return medianOf(us), medianOf(vs)
}

// rec709Luma is a local copy of the shared luma extractor (kept
// here to keep pkg/vision/flow free of intra-project imports).
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

// medianOf returns the median of a float64 slice without mutating
// it. Uses an in-place sort on a copy.
func medianOf(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	cp := append([]float64(nil), values...)
	insertionSort(cp)
	n := len(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}

// insertionSort sorts ascending. Fine for the sparse-flow vector
// counts (typically ≤ 5k points per frame).
func insertionSort(a []float64) {
	for i := 1; i < len(a); i++ {
		x := a[i]
		j := i - 1
		for j >= 0 && a[j] > x {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = x
	}
}
