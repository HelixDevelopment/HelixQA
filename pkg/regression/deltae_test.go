// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package regression

import (
	"context"
	"errors"
	"image"
	"image/color"
	"math"
	"testing"
)

// ---------------------------------------------------------------------------
// Sharma 2005 benchmark values — the canonical CIEDE2000 test suite.
// https://www.ece.rochester.edu/~gsharma/ciede2000/dataNprograms/CIEDE2000.xls
// ---------------------------------------------------------------------------

var sharmaCases = []struct {
	name    string
	l1, a1, b1, l2, a2, b2, want float64
}{
	// Pairs from Sharma 2005 Table 1 (canonical CIEDE2000 test
	// vectors — https://www2.ece.rochester.edu/~gsharma/ciede2000/).
	// Case numbers match the Sharma table row indices.
	{"case 1", 50.0000, 2.6772, -79.7751, 50.0000, 0.0000, -82.7485, 2.0425},
	{"case 2", 50.0000, 3.1571, -77.2803, 50.0000, 0.0000, -82.7485, 2.8615},
	{"case 3", 50.0000, 2.8361, -74.0200, 50.0000, 0.0000, -82.7485, 3.4412},
	{"case 4", 50.0000, -1.3802, -84.2814, 50.0000, 0.0000, -82.7485, 1.0000},
	{"case 5", 50.0000, -1.1848, -84.8006, 50.0000, 0.0000, -82.7485, 1.0000},
	{"case 6", 50.0000, -0.9009, -85.5211, 50.0000, 0.0000, -82.7485, 1.0000},
	{"case 7", 50.0000, 0.0000, 0.0000, 50.0000, -1.0000, 2.0000, 2.3669},
	{"case 14", 22.7233, 20.0904, -46.6940, 23.0331, 14.9730, -42.5619, 2.0373},
	{"case 15", 36.4612, 47.8580, 18.3852, 36.2715, 50.5065, 21.2231, 1.4146},
	// Large hue+chroma shift (exercises the rotation term).
	{"case 26", 50.0000, 2.5000, 0.0000, 58.0000, 24.0000, 15.0000, 19.4535},
}

func TestDeltaE2000_SharmaReferenceValues(t *testing.T) {
	const tolerance = 0.01 // 0.01 ΔE is well within the precision of the Sharma tables.
	for _, c := range sharmaCases {
		got := DeltaE2000(
			LAB{L: c.l1, A: c.a1, B: c.b1},
			LAB{L: c.l2, A: c.a2, B: c.b2},
		)
		if math.Abs(got-c.want) > tolerance {
			t.Errorf("%s: DeltaE2000 = %v, want %v (±%v)", c.name, got, c.want, tolerance)
		}
	}
}

// ---------------------------------------------------------------------------
// Basic invariants
// ---------------------------------------------------------------------------

func TestDeltaE2000_IdenticalInputsZero(t *testing.T) {
	lab := LAB{L: 55, A: 10, B: -20}
	if got := DeltaE2000(lab, lab); got > 0.001 {
		t.Fatalf("identical LAB should have ΔE = 0, got %v", got)
	}
}

func TestDeltaE2000_Symmetric(t *testing.T) {
	a := LAB{L: 50, A: 50, B: 0}
	b := LAB{L: 50, A: 0, B: 0}
	if d1, d2 := DeltaE2000(a, b), DeltaE2000(b, a); math.Abs(d1-d2) > 0.01 {
		t.Fatalf("ΔE should be symmetric: (a,b)=%v, (b,a)=%v", d1, d2)
	}
}

func TestDeltaE2000_NonNegative(t *testing.T) {
	cases := []struct{ a, b LAB }{
		{LAB{0, 0, 0}, LAB{100, 0, 0}},
		{LAB{50, -50, -50}, LAB{50, 50, 50}},
		{LAB{50, 0, 0}, LAB{50, 0, 0}},
	}
	for _, c := range cases {
		if d := DeltaE2000(c.a, c.b); d < 0 {
			t.Errorf("ΔE should be non-negative, got %v for %+v → %+v", d, c.a, c.b)
		}
	}
}

// ---------------------------------------------------------------------------
// RGBToLAB — D65 reference white + primary colors
// ---------------------------------------------------------------------------

func TestRGBToLAB_Black(t *testing.T) {
	lab := RGBToLAB(0, 0, 0)
	if math.Abs(lab.L) > 0.001 || math.Abs(lab.A) > 0.001 || math.Abs(lab.B) > 0.001 {
		t.Fatalf("black → %+v, want {0, 0, 0}", lab)
	}
}

func TestRGBToLAB_White(t *testing.T) {
	lab := RGBToLAB(255, 255, 255)
	// White under D65 → L≈100, A/B ≈ 0.
	if math.Abs(lab.L-100) > 0.1 {
		t.Errorf("white L = %v, want ≈ 100", lab.L)
	}
	if math.Abs(lab.A) > 0.1 || math.Abs(lab.B) > 0.1 {
		t.Errorf("white A/B = (%v, %v), want ≈ (0, 0)", lab.A, lab.B)
	}
}

func TestRGBToLAB_PrimaryRed(t *testing.T) {
	lab := RGBToLAB(255, 0, 0)
	// Reference: sRGB red → L≈53.24, A≈80.09, B≈67.20.
	if math.Abs(lab.L-53.24) > 0.1 {
		t.Errorf("red L = %v, want ≈ 53.24", lab.L)
	}
	if math.Abs(lab.A-80.09) > 0.1 {
		t.Errorf("red A = %v, want ≈ 80.09", lab.A)
	}
	if math.Abs(lab.B-67.20) > 0.1 {
		t.Errorf("red B = %v, want ≈ 67.20", lab.B)
	}
}

func TestRGBToLAB_PrimaryGreen(t *testing.T) {
	lab := RGBToLAB(0, 255, 0)
	// Reference: sRGB green → L≈87.73, A≈-86.18, B≈83.18.
	if math.Abs(lab.L-87.73) > 0.1 {
		t.Errorf("green L = %v, want ≈ 87.73", lab.L)
	}
	if math.Abs(lab.A-(-86.18)) > 0.2 {
		t.Errorf("green A = %v, want ≈ -86.18", lab.A)
	}
}

func TestRGBToLAB_PrimaryBlue(t *testing.T) {
	lab := RGBToLAB(0, 0, 255)
	// Reference: sRGB blue → L≈32.30, A≈79.20, B≈-107.86.
	if math.Abs(lab.L-32.30) > 0.1 {
		t.Errorf("blue L = %v, want ≈ 32.30", lab.L)
	}
	if math.Abs(lab.B-(-107.86)) > 0.2 {
		t.Errorf("blue B = %v, want ≈ -107.86", lab.B)
	}
}

// ---------------------------------------------------------------------------
// ColorToLAB
// ---------------------------------------------------------------------------

func TestColorToLAB_WrapsRGBToLAB(t *testing.T) {
	// Given a color.RGBA, ColorToLAB should produce the same result
	// as RGBToLAB on the unshifted channels.
	rgba := color.RGBA{R: 128, G: 200, B: 50, A: 255}
	got := ColorToLAB(rgba)
	want := RGBToLAB(128, 200, 50)
	if math.Abs(got.L-want.L) > 0.01 || math.Abs(got.A-want.A) > 0.01 || math.Abs(got.B-want.B) > 0.01 {
		t.Fatalf("ColorToLAB = %+v, want %+v", got, want)
	}
}

// ---------------------------------------------------------------------------
// BrandComplianceReport + CheckBrandCompliance
// ---------------------------------------------------------------------------

func solidImage(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func TestCheckBrandCompliance_AllPixelsInRange(t *testing.T) {
	img := solidImage(16, 16, color.RGBA{R: 200, G: 50, B: 50, A: 255})
	target := RGBToLAB(200, 50, 50)
	r, err := CheckBrandCompliance(context.Background(), img, target, 1.0)
	if err != nil {
		t.Fatalf("CheckBrandCompliance: %v", err)
	}
	if r.TotalPixels != 256 {
		t.Fatalf("TotalPixels = %d, want 256", r.TotalPixels)
	}
	if r.InRange != 256 {
		t.Fatalf("InRange = %d, want 256", r.InRange)
	}
	if r.MaxDeltaE > 0.001 || r.MeanDeltaE > 0.001 {
		t.Fatalf("identical pixels should have ΔE ≈ 0, got max=%v mean=%v", r.MaxDeltaE, r.MeanDeltaE)
	}
	if r.PassRate() != 1.0 {
		t.Fatalf("PassRate = %v, want 1.0", r.PassRate())
	}
}

func TestCheckBrandCompliance_AllPixelsOutOfRange(t *testing.T) {
	// Pure red image, target pure green — no pixel can be within
	// ΔE 5 of green.
	img := solidImage(8, 8, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	target := RGBToLAB(0, 255, 0)
	r, _ := CheckBrandCompliance(context.Background(), img, target, 5.0)
	if r.InRange != 0 {
		t.Fatalf("InRange = %d, want 0", r.InRange)
	}
	if r.PassRate() != 0.0 {
		t.Fatalf("PassRate = %v, want 0.0", r.PassRate())
	}
}

func TestCheckBrandCompliance_PartialPass(t *testing.T) {
	// Half red, half green. Target red, threshold that lets reds
	// pass but rejects greens.
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			if x < 5 {
				img.SetRGBA(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{R: 0, G: 255, B: 0, A: 255})
			}
		}
	}
	target := RGBToLAB(255, 0, 0)
	r, _ := CheckBrandCompliance(context.Background(), img, target, 5.0)
	if r.InRange != 50 {
		t.Fatalf("InRange = %d, want 50 (half the image matches)", r.InRange)
	}
	if pr := r.PassRate(); math.Abs(pr-0.5) > 0.001 {
		t.Fatalf("PassRate = %v, want 0.5", pr)
	}
	// Max should be large (red vs green is extreme); mean somewhere
	// in between.
	if r.MaxDeltaE <= 10 {
		t.Fatalf("MaxDeltaE = %v, want > 10", r.MaxDeltaE)
	}
}

func TestCheckBrandCompliance_PassRateEmpty(t *testing.T) {
	if r := (BrandComplianceReport{}); r.PassRate() != 0 {
		t.Fatalf("empty PassRate = %v, want 0", r.PassRate())
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestCheckBrandCompliance_NilImage(t *testing.T) {
	if _, err := CheckBrandCompliance(context.Background(), nil, LAB{}, 1); !errors.Is(err, ErrNilImage) {
		t.Fatalf("nil image = %v, want ErrNilImage", err)
	}
}

func TestCheckBrandCompliance_NegativeThreshold(t *testing.T) {
	img := solidImage(2, 2, color.RGBA{A: 255})
	if _, err := CheckBrandCompliance(context.Background(), img, LAB{}, -1); !errors.Is(err, ErrNegativeThreshold) {
		t.Fatalf("negative threshold = %v, want ErrNegativeThreshold", err)
	}
}

func TestCheckBrandCompliance_ZeroBoundsReturnsEmpty(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 0, 0))
	r, err := CheckBrandCompliance(context.Background(), img, LAB{}, 1)
	if err != nil {
		t.Fatalf("zero-bounds: %v", err)
	}
	if r.TotalPixels != 0 {
		t.Fatalf("TotalPixels = %d, want 0", r.TotalPixels)
	}
}

func TestCheckBrandCompliance_ContextCanceled(t *testing.T) {
	// Need > 16 rows to reach the ctx check on row 16.
	img := solidImage(8, 32, color.RGBA{R: 255, A: 255})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := CheckBrandCompliance(ctx, img, LAB{}, 1)
	if err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

// ---------------------------------------------------------------------------
// sRGB gamma + lab-F helpers
// ---------------------------------------------------------------------------

func TestSRGBToLinear_LowBranch(t *testing.T) {
	// Values ≤ 0.04045 go through the linear branch.
	v := srgbToLinear(0.02)
	want := 0.02 / 12.92
	if math.Abs(v-want) > 1e-9 {
		t.Fatalf("low-branch = %v, want %v", v, want)
	}
}

func TestSRGBToLinear_HighBranch(t *testing.T) {
	v := srgbToLinear(0.5)
	// Pow branch — rough check that result is < input (gamma > 1).
	if v >= 0.5 || v <= 0 {
		t.Fatalf("high-branch = %v, want 0 < v < 0.5", v)
	}
}

func TestLabF_LowBranch(t *testing.T) {
	// Low t (below δ³) uses the linear branch.
	const delta = 6.0 / 29.0
	t0 := delta*delta*delta / 2 // comfortably below the branch point
	want := t0/(3*delta*delta) + 4.0/29.0
	if got := labF(t0); math.Abs(got-want) > 1e-12 {
		t.Fatalf("low-branch = %v, want %v", got, want)
	}
}

func TestLabF_HighBranch(t *testing.T) {
	// t = 1 → cbrt(1) = 1.
	if got := labF(1.0); math.Abs(got-1) > 1e-12 {
		t.Fatalf("high-branch = %v, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// hueAngleDeg edge cases
// ---------------------------------------------------------------------------

func TestHueAngleDeg_BothZeroIsZero(t *testing.T) {
	if got := hueAngleDeg(0, 0); got != 0 {
		t.Fatalf("hueAngle(0, 0) = %v, want 0", got)
	}
}

func TestHueAngleDeg_NegativeBranchWraps(t *testing.T) {
	// atan2(-1, 1) = -45° → wraps to 315°.
	got := hueAngleDeg(-1, 1)
	if math.Abs(got-315) > 0.001 {
		t.Fatalf("hueAngle(-1, 1) = %v, want 315", got)
	}
}

func TestDeg2Rad(t *testing.T) {
	if got := deg2rad(180); math.Abs(got-math.Pi) > 1e-12 {
		t.Fatalf("deg2rad(180) = %v, want π", got)
	}
}
