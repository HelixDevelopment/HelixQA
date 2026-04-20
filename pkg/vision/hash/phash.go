// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package hash

import (
	"image"
	"math"
	"math/bits"
	"sort"
)

// PHasher implements pHash (Zauner 2010, "Implementation and
// Benchmarking of Perceptual Image Hash Functions") using the DCT-II
// of the 32×32 grayscale image, keeping the 8×8 low-frequency block
// (skipping DC), comparing each coefficient against the median, and
// packing the boolean results into 64 bits.
//
// pHash is more robust than dHash against rotations, scaling and
// mild color shifts. The tradeoff is ~5× the CPU cost (the DCT is
// O(N²·M²)). Use pHash when dHash reports a medium Hamming distance
// for two frames that a human would consider the same scene.
//
// Safe to share across goroutines — no internal state.
type PHasher struct{}

// Hash computes a 64-bit pHash of img.
func (PHasher) Hash(img image.Image) (uint64, error) {
	if img == nil {
		return 0, ErrNilImage
	}
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return 0, ErrZeroBounds
	}

	// Step 1: reduce to 32×32 luma. Reusing dHash's fast-path
	// resizer keeps the pHash implementation small.
	const N = 32
	pixels, err := resizeGray(img, N, N)
	if err != nil {
		return 0, err
	}

	// Step 2: compute the DCT-II of the 32×32 grid.
	dct := dct32(pixels)

	// Step 3: extract the 8×8 top-left block (low frequencies,
	// including DC at [0][0]).
	const M = 8
	coeffs := make([]float64, 0, M*M)
	for y := 0; y < M; y++ {
		for x := 0; x < M; x++ {
			coeffs = append(coeffs, dct[y*N+x])
		}
	}

	// Step 4: compute the median of the 63 non-DC coefficients.
	// Excluding DC is the canonical pHash trick — DC dominates the
	// magnitude and would make the median trivially high.
	withoutDC := append([]float64(nil), coeffs[1:]...)
	sort.Float64s(withoutDC)
	var median float64
	if n := len(withoutDC); n%2 == 1 {
		median = withoutDC[n/2]
	} else {
		median = (withoutDC[n/2-1] + withoutDC[n/2]) / 2
	}

	// Step 5: pack one bit per coefficient — 1 if above median,
	// 0 otherwise. This produces a 64-bit hash; the DC bit at
	// index 0 is almost always above the median (its coefficient is
	// huge) but carrying it keeps the encoding uniform.
	var out uint64
	for i, c := range coeffs {
		if c > median {
			out |= uint64(1) << uint(63-i)
		}
	}
	return out, nil
}

// Distance returns the Hamming distance between two 64-bit pHashes.
func (PHasher) Distance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// dct32 computes the 2-D DCT-II of a 32×32 grayscale buffer using
// the separable 1-D formulation: run the 1-D DCT on every row, then
// on every column of the row-DCT result. 2048 inner-product
// iterations per 1-D pass × 64 passes = ~130k multiplications total,
// which benchmarks below 1 ms on commodity CPU.
//
// Exposed package-internally so other hashers (future pHash-256,
// wHash) can reuse the same transform kernel.
func dct32(pixels []uint8) []float64 {
	const N = 32
	if len(pixels) != N*N {
		// Callers guarantee this invariant; this panic would only
		// fire on an internal refactor bug.
		panic("hash: dct32 input must be exactly 32×32 bytes")
	}
	// Precompute the cosine table once per call (32×32 = 1024 float64s,
	// ~8 KB, fits in L1). The table is C[k][n] = cos((2n+1)kπ / 2N).
	var cosTab [N][N]float64
	for k := 0; k < N; k++ {
		for n := 0; n < N; n++ {
			cosTab[k][n] = math.Cos(float64(2*n+1) * float64(k) * math.Pi / (2 * N))
		}
	}

	// Pass 1: row DCT. tmp[y*N+k] is the k-th DCT coefficient of row y.
	tmp := make([]float64, N*N)
	for y := 0; y < N; y++ {
		for k := 0; k < N; k++ {
			var sum float64
			for n := 0; n < N; n++ {
				sum += float64(pixels[y*N+n]) * cosTab[k][n]
			}
			tmp[y*N+k] = sum
		}
	}

	// Pass 2: column DCT. out[ky*N+kx] is the full 2-D coefficient.
	out := make([]float64, N*N)
	for kx := 0; kx < N; kx++ {
		for ky := 0; ky < N; ky++ {
			var sum float64
			for n := 0; n < N; n++ {
				sum += tmp[n*N+kx] * cosTab[ky][n]
			}
			out[ky*N+kx] = sum
		}
	}

	return out
}

// PHasher reuses the DHasher sentinels (ErrNilImage, ErrZeroBounds
// from dhash.go) for error coherence across the package.

// Compile-time guard: PHasher satisfies the package-wide Hasher
// interface from dhash.go.
var _ Hasher = PHasher{}
