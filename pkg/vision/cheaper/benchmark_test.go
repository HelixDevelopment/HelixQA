// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"image"
	"image/color"
	"testing"

	"digital.vasic.helixqa/pkg/vision/cheaper/adapters"
	"digital.vasic.helixqa/pkg/vision/cheaper/cache"
	"digital.vasic.helixqa/pkg/vision/cheaper/memory"
)

// newBenchImage returns a w×h RGBA image filled with a solid colour,
// suitable for use in benchmarks.
func newBenchImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 42, G: 84, B: 168, A: 255})
		}
	}
	return img
}

// BenchmarkExactCache_Hit measures Get throughput when the entry is already
// present in the cache (warm-cache path).
func BenchmarkExactCache_Hit(b *testing.B) {
	c := cache.NewExactCache(1000)
	img := newBenchImage(100, 100)
	prompt := "describe the UI elements on screen"
	resp := &cache.CachedResponse{Text: "hit", Model: "bench-model"}
	c.Put(img, prompt, resp)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(img, prompt)
	}
}

// BenchmarkExactCache_Miss measures Get throughput when the cache is empty
// (cold-cache path).
func BenchmarkExactCache_Miss(b *testing.B) {
	c := cache.NewExactCache(1000)
	img := newBenchImage(100, 100)
	prompt := "describe the UI elements on screen"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get(img, prompt)
	}
}

// BenchmarkRegistry_Create measures the Create hot path after all providers
// have already been registered.
func BenchmarkRegistry_Create(b *testing.B) {
	reg := NewRegistry()
	reg.Register("bench-provider", stubFactory("bench-provider"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = reg.Create("bench-provider", nil)
	}
}

// BenchmarkImageToBase64 measures the cost of PNG-encoding + base64-encoding a
// 100×100 test image — the hot path in every provider adapter.
func BenchmarkImageToBase64(b *testing.B) {
	img := newBenchImage(100, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = adapters.ImageToBase64(img)
	}
}

// BenchmarkComputeImageHash measures the pixel-hashing throughput for a
// 100×100 image — called on every cache lookup and store.
func BenchmarkComputeImageHash(b *testing.B) {
	img := newBenchImage(100, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = memory.ComputeImageHash(img)
	}
}
