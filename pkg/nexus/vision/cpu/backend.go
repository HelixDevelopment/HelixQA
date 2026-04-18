// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package cpu is the OCU P2.5 CPU fallback for the vision pipeline.
// It accepts BGRA8 frames and produces pure-Go Diff (per-pixel |Δ|
// with contiguous-region flood-fill) and Analyze (Sobel edge detection
// → contiguous-region UIElements).
//
// Kill-switch: HELIXQA_VISION_CPU_STUB=1 restores the original empty-
// result behaviour for tests that want deterministic zero output.
package cpu

import (
	"context"
	"fmt"
	"os"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// Backend is the CPU-only vision.LocalBackend.
type Backend struct{}

// New returns a new CPU backend.
func New() *Backend { return &Backend{} }

// Analyze implements vision.LocalBackend.
//
// When HELIXQA_VISION_CPU_STUB=1 the result is empty (DispatchedTo is
// still set for identification). Otherwise a simple Sobel-based edge
// detector runs over the luminance channel and returns contiguous high-
// gradient regions as UIElements with Kind "contour" and Source "cv".
func (b *Backend) Analyze(_ context.Context, frame contracts.Frame) (*contracts.Analysis, error) {
	if err := requireBGRA(frame); err != nil {
		return nil, err
	}
	if stubActive("HELIXQA_VISION_CPU_STUB") {
		return &contracts.Analysis{DispatchedTo: "local-cpu"}, nil
	}

	pixels, w, h, ok := framePixels(frame)
	if !ok {
		return &contracts.Analysis{DispatchedTo: "local-cpu"}, nil
	}

	lum := luminance(pixels, w, h)
	edges := sobelEdge(lum, w, h)
	rects := connectedRegions(edges, w, h, true)

	elements := make([]contracts.UIElement, 0, len(rects))
	for _, r := range rects {
		area := float64(r.W * r.H)
		if area == 0 {
			continue
		}
		conf := clampF(area/float64(w*h)*10.0, 0.05, 1.0)
		elements = append(elements, contracts.UIElement{
			Kind:       "contour",
			Rect:       r,
			Source:     "cv",
			Confidence: conf,
		})
	}
	return &contracts.Analysis{
		Elements:     elements,
		DispatchedTo: "local-cpu",
	}, nil
}

// Match implements vision.LocalBackend.
func (b *Backend) Match(_ context.Context, frame contracts.Frame, _ contracts.Template) ([]contracts.Match, error) {
	if err := requireBGRA(frame); err != nil {
		return nil, err
	}
	return nil, nil
}

// Diff implements vision.LocalBackend.
//
// When HELIXQA_VISION_CPU_STUB=1 only SameShape is populated (zero delta,
// no regions). Otherwise per-pixel |Δ| averaged across channels is
// computed; contiguous pixels above threshold 0.05 are flood-filled into
// ChangeRegions.
func (b *Backend) Diff(_ context.Context, before, after contracts.Frame) (*contracts.DiffResult, error) {
	if err := requireBGRA(before); err != nil {
		return nil, err
	}
	if err := requireBGRA(after); err != nil {
		return nil, err
	}
	same := before.Width == after.Width && before.Height == after.Height
	if stubActive("HELIXQA_VISION_CPU_STUB") {
		return &contracts.DiffResult{SameShape: same}, nil
	}
	if !same {
		return &contracts.DiffResult{SameShape: false}, nil
	}

	bPx, bw, bh, bOk := framePixels(before)
	aPx, _, _, aOk := framePixels(after)
	if !bOk || !aOk {
		return &contracts.DiffResult{SameShape: same}, nil
	}

	w, h := bw, bh
	delta := pixelDeltaMap(bPx, aPx, w, h)

	const threshold = 0.05
	rects := connectedRegions(thresholdMap(delta, threshold), w, h, false)

	var sumDelta float64
	total := w * h
	if total > 0 {
		for i := range delta {
			sumDelta += delta[i]
		}
		sumDelta /= float64(total)
	}

	regions := make([]contracts.ChangeRegion, 0, len(rects))
	for _, r := range rects {
		mag := regionMean(delta, r, w)
		regions = append(regions, contracts.ChangeRegion{
			Rect:       r,
			Magnitude:  mag,
			PixelCount: r.W * r.H,
		})
	}
	return &contracts.DiffResult{
		SameShape:  true,
		TotalDelta: sumDelta,
		Regions:    regions,
	}, nil
}

// OCR implements vision.LocalBackend.
func (b *Backend) OCR(_ context.Context, frame contracts.Frame, _ contracts.Rect) (contracts.OCRResult, error) {
	if err := requireBGRA(frame); err != nil {
		return contracts.OCRResult{}, err
	}
	return contracts.OCRResult{}, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func requireBGRA(f contracts.Frame) error {
	if f.Format != contracts.PixelFormatBGRA8 {
		return fmt.Errorf("cpu backend: unsupported pixel format %q (want BGRA8)", f.Format)
	}
	return nil
}

func stubActive(env string) bool {
	return os.Getenv(env) == "1"
}

// framePixels extracts a flat []byte from frame.Data.AsBytes().
// Returns (nil, 0, 0, false) when Data is nil or AsBytes fails.
func framePixels(f contracts.Frame) ([]byte, int, int, bool) {
	if f.Data == nil {
		return nil, 0, 0, false
	}
	b, err := f.Data.AsBytes()
	if err != nil || len(b) == 0 {
		return nil, 0, 0, false
	}
	w, h := f.Width, f.Height
	if w <= 0 || h <= 0 || len(b) < w*h*4 {
		return nil, 0, 0, false
	}
	return b, w, h, true
}

// luminance returns a float64 grayscale map (0..1) from BGRA8 data.
func luminance(px []byte, w, h int) []float64 {
	out := make([]float64, w*h)
	for i := 0; i < w*h; i++ {
		b := float64(px[i*4+0])
		g := float64(px[i*4+1])
		r := float64(px[i*4+2])
		out[i] = (0.299*r + 0.587*g + 0.114*b) / 255.0
	}
	return out
}

// sobelEdge returns a boolean edge map (true = high gradient) from a
// luminance map using a 3x3 Sobel operator.
func sobelEdge(lum []float64, w, h int) []bool {
	out := make([]bool, w*h)
	const thresh = 0.15
	for y := 1; y < h-1; y++ {
		for x := 1; x < w-1; x++ {
			idx := y*w + x
			gx := -lum[(y-1)*w+(x-1)] - 2*lum[y*w+(x-1)] - lum[(y+1)*w+(x-1)] +
				lum[(y-1)*w+(x+1)] + 2*lum[y*w+(x+1)] + lum[(y+1)*w+(x+1)]
			gy := -lum[(y-1)*w+(x-1)] - 2*lum[(y-1)*w+x] - lum[(y-1)*w+(x+1)] +
				lum[(y+1)*w+(x-1)] + 2*lum[(y+1)*w+x] + lum[(y+1)*w+(x+1)]
			mag := gx*gx + gy*gy
			out[idx] = mag > thresh*thresh
		}
	}
	return out
}

// pixelDeltaMap computes per-pixel mean |Δ| across BGRA channels (0..1).
func pixelDeltaMap(a, b []byte, w, h int) []float64 {
	out := make([]float64, w*h)
	for i := 0; i < w*h; i++ {
		var s float64
		for c := 0; c < 4; c++ {
			d := float64(a[i*4+c]) - float64(b[i*4+c])
			if d < 0 {
				d = -d
			}
			s += d
		}
		out[i] = s / (4.0 * 255.0)
	}
	return out
}

// thresholdMap converts a float64 map to a boolean map (true = above t).
func thresholdMap(m []float64, t float64) []bool {
	out := make([]bool, len(m))
	for i, v := range m {
		out[i] = v > t
	}
	return out
}

// connectedRegions flood-fills contiguous true pixels into bounding Rects.
// If minArea is true, single-pixel components are skipped.
func connectedRegions(mask []bool, w, h int, minArea bool) []contracts.Rect {
	visited := make([]bool, w*h)
	var rects []contracts.Rect

	for start := 0; start < w*h; start++ {
		if !mask[start] || visited[start] {
			continue
		}
		x0, y0 := start%w, start/w
		x1, y1 := x0, y0
		queue := []int{start}
		visited[start] = true

		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			cx, cy := cur%w, cur/w
			if cx < x0 {
				x0 = cx
			}
			if cx > x1 {
				x1 = cx
			}
			if cy < y0 {
				y0 = cy
			}
			if cy > y1 {
				y1 = cy
			}
			for _, nb := range neighbors4(cx, cy, w, h) {
				if !visited[nb] && mask[nb] {
					visited[nb] = true
					queue = append(queue, nb)
				}
			}
		}
		rw, rh := x1-x0+1, y1-y0+1
		if minArea && rw*rh < 4 {
			continue
		}
		rects = append(rects, contracts.Rect{X: x0, Y: y0, W: rw, H: rh})
	}
	return rects
}

func neighbors4(x, y, w, h int) []int {
	var nb []int
	if x > 0 {
		nb = append(nb, y*w+(x-1))
	}
	if x < w-1 {
		nb = append(nb, y*w+(x+1))
	}
	if y > 0 {
		nb = append(nb, (y-1)*w+x)
	}
	if y < h-1 {
		nb = append(nb, (y+1)*w+x)
	}
	return nb
}

func regionMean(delta []float64, r contracts.Rect, w int) float64 {
	var s float64
	n := 0
	for dy := 0; dy < r.H; dy++ {
		for dx := 0; dx < r.W; dx++ {
			s += delta[(r.Y+dy)*w+(r.X+dx)]
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return s / float64(n)
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
