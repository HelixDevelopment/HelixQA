// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package regression

import (
	"context"
	"errors"
	"image"
	"image/color"
	"math"
)

// LAB is a color in the CIE 1976 L*a*b* color space, the perceptual
// color space on which the CIEDE2000 formula operates.
//
//   L — lightness (0 = black, 100 = white).
//   A — green ⇄ red (negative = green, positive = red).
//   B — blue ⇄ yellow (negative = blue, positive = yellow).
type LAB struct {
	L, A, B float64
}

// ---------------------------------------------------------------------------
// sRGB → CIELAB conversion
// ---------------------------------------------------------------------------

// srgbToLinear inverts the sRGB gamma. IEC 61966-2-1 piecewise formula.
func srgbToLinear(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

// D65 reference white (CIE 1931 2° observer, used universally with sRGB).
const (
	d65X = 95.047
	d65Y = 100.0
	d65Z = 108.883
)

// f is the CIELAB nonlinear companding function.
func labF(t float64) float64 {
	const delta = 6.0 / 29.0
	if t > delta*delta*delta {
		return math.Cbrt(t)
	}
	return t/(3*delta*delta) + 4.0/29.0
}

// RGBToLAB converts an 8-bit RGB triplet (0–255) to CIELAB via
// sRGB → linear-RGB → XYZ → LAB with the D65 illuminant.
func RGBToLAB(r, g, b uint8) LAB {
	rl := srgbToLinear(float64(r) / 255)
	gl := srgbToLinear(float64(g) / 255)
	bl := srgbToLinear(float64(b) / 255)

	// sRGB → XYZ (D65, IEC 61966-2-1) — scaled so Y=100 for white.
	x := (rl*0.4124564 + gl*0.3575761 + bl*0.1804375) * 100
	y := (rl*0.2126729 + gl*0.7151522 + bl*0.0721750) * 100
	z := (rl*0.0193339 + gl*0.1191920 + bl*0.9503041) * 100

	fx := labF(x / d65X)
	fy := labF(y / d65Y)
	fz := labF(z / d65Z)

	return LAB{
		L: 116*fy - 16,
		A: 500 * (fx - fy),
		B: 200 * (fy - fz),
	}
}

// ColorToLAB is a convenience for image.Image pixel conversions.
// Delegates to RGBToLAB after shifting the 16-bit RGBA() channels.
func ColorToLAB(c color.Color) LAB {
	r, g, b, _ := c.RGBA()
	return RGBToLAB(uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// ---------------------------------------------------------------------------
// CIEDE2000 color difference
// ---------------------------------------------------------------------------

// DeltaE2000 returns the CIEDE2000 perceptual color difference
// between LAB1 and LAB2 (CIE 2002 §7, Sharma 2005 §5).
//
// Interpretation (approximate):
//
//	ΔE < 1        — not perceptible by the human eye.
//	ΔE 1–2        — perceptible on close inspection.
//	ΔE 2–10       — perceptible at a glance.
//	ΔE > 10       — very different colors.
//
// HelixQA uses this for brand-compliance verification (a rendered
// pixel must be within ΔE N of a brand-guideline color) and
// dark-mode verification (the theme-switched pixel must differ from
// the light-mode version by > ΔE threshold).
func DeltaE2000(c1, c2 LAB) float64 {
	return deltaE2000KParams(c1, c2, 1, 1, 1)
}

// deltaE2000KParams accepts the three weighting factors (k_L, k_C,
// k_H) which are all 1 by default. Graphics-arts applications
// sometimes use k_L=2 to downweight lightness differences.
func deltaE2000KParams(c1, c2 LAB, kL, kC, kH float64) float64 {
	// Step 1 — chroma and G factor.
	c1Chroma := math.Hypot(c1.A, c1.B)
	c2Chroma := math.Hypot(c2.A, c2.B)
	avgC := (c1Chroma + c2Chroma) / 2
	avgC7 := math.Pow(avgC, 7)
	const pow25_7 = 6103515625.0 // 25^7
	g := 0.5 * (1 - math.Sqrt(avgC7/(avgC7+pow25_7)))

	// Step 2 — a' primes and new chroma + hue.
	a1Prime := (1 + g) * c1.A
	a2Prime := (1 + g) * c2.A
	c1Prime := math.Hypot(a1Prime, c1.B)
	c2Prime := math.Hypot(a2Prime, c2.B)

	h1Prime := hueAngleDeg(c1.B, a1Prime)
	h2Prime := hueAngleDeg(c2.B, a2Prime)

	// Step 3 — differences.
	deltaL := c2.L - c1.L
	deltaC := c2Prime - c1Prime

	var deltaHPrime float64
	switch {
	case c1Prime*c2Prime == 0:
		deltaHPrime = 0
	case math.Abs(h2Prime-h1Prime) <= 180:
		deltaHPrime = h2Prime - h1Prime
	case h2Prime-h1Prime > 180:
		deltaHPrime = h2Prime - h1Prime - 360
	default:
		deltaHPrime = h2Prime - h1Prime + 360
	}
	deltaH := 2 * math.Sqrt(c1Prime*c2Prime) * math.Sin(deg2rad(deltaHPrime/2))

	// Step 4 — averaged quantities.
	avgL := (c1.L + c2.L) / 2
	avgCPrime := (c1Prime + c2Prime) / 2

	var avgHPrime float64
	hSum := h1Prime + h2Prime
	switch {
	case c1Prime*c2Prime == 0:
		avgHPrime = hSum
	case math.Abs(h1Prime-h2Prime) <= 180:
		avgHPrime = hSum / 2
	case hSum < 360:
		avgHPrime = (hSum + 360) / 2
	default:
		avgHPrime = (hSum - 360) / 2
	}

	// Step 5 — T, rotation term, weighting functions.
	t := 1 -
		0.17*math.Cos(deg2rad(avgHPrime-30)) +
		0.24*math.Cos(deg2rad(2*avgHPrime)) +
		0.32*math.Cos(deg2rad(3*avgHPrime+6)) -
		0.20*math.Cos(deg2rad(4*avgHPrime-63))

	avgCPrime7 := math.Pow(avgCPrime, 7)
	rC := 2 * math.Sqrt(avgCPrime7/(avgCPrime7+pow25_7))
	deltaTheta := 30 * math.Exp(-math.Pow((avgHPrime-275)/25, 2))
	rT := -math.Sin(deg2rad(2*deltaTheta)) * rC

	lShift := avgL - 50
	sL := 1 + (0.015*lShift*lShift)/math.Sqrt(20+lShift*lShift)
	sC := 1 + 0.045*avgCPrime
	sH := 1 + 0.015*avgCPrime*t

	// Step 6 — final ΔE.
	lTerm := deltaL / (kL * sL)
	cTerm := deltaC / (kC * sC)
	hTerm := deltaH / (kH * sH)

	return math.Sqrt(lTerm*lTerm + cTerm*cTerm + hTerm*hTerm + rT*cTerm*hTerm)
}

// hueAngleDeg returns the hue angle in degrees, in [0, 360). The
// CIEDE2000 spec defines atan2(b, a') with a CIE-specific branch
// convention (angle 0 when both inputs are zero).
func hueAngleDeg(b, a float64) float64 {
	if a == 0 && b == 0 {
		return 0
	}
	deg := math.Atan2(b, a) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return deg
}

func deg2rad(deg float64) float64 { return deg * math.Pi / 180 }

// ---------------------------------------------------------------------------
// BrandComplianceReport + Check
// ---------------------------------------------------------------------------

// BrandComplianceReport aggregates per-pixel ΔE2000 statistics over
// an image against a single target color.
type BrandComplianceReport struct {
	// TotalPixels is the total number of pixels considered (W×H for
	// the supplied image).
	TotalPixels int

	// InRange is the number of pixels whose ΔE to Target is < or ≤
	// the configured Threshold (see CheckBrandCompliance).
	InRange int

	// MaxDeltaE is the worst ΔE across every pixel.
	MaxDeltaE float64

	// MeanDeltaE is the arithmetic mean of ΔE across every pixel.
	MeanDeltaE float64
}

// PassRate returns InRange / TotalPixels, or 0 for an empty report.
func (r BrandComplianceReport) PassRate() float64 {
	if r.TotalPixels == 0 {
		return 0
	}
	return float64(r.InRange) / float64(r.TotalPixels)
}

// CheckBrandCompliance computes per-pixel ΔE2000 against target and
// counts pixels whose ΔE is ≤ threshold. Useful for verifying that
// a rendered surface (logo, branded button) stays within
// guideline-defined perceptual tolerance of the canonical brand
// color.
func CheckBrandCompliance(ctx context.Context, img image.Image, target LAB, threshold float64) (BrandComplianceReport, error) {
	if img == nil {
		return BrandComplianceReport{}, ErrNilImage
	}
	if threshold < 0 {
		return BrandComplianceReport{}, ErrNegativeThreshold
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return BrandComplianceReport{}, nil
	}

	var sum float64
	var max float64
	var inRange int
	total := w * h

	for y := b.Min.Y; y < b.Max.Y; y++ {
		if y&0x0F == 0 {
			if err := ctx.Err(); err != nil {
				return BrandComplianceReport{}, err
			}
		}
		for x := b.Min.X; x < b.Max.X; x++ {
			lab := ColorToLAB(img.At(x, y))
			d := DeltaE2000(lab, target)
			sum += d
			if d > max {
				max = d
			}
			if d <= threshold {
				inRange++
			}
		}
	}

	return BrandComplianceReport{
		TotalPixels: total,
		InRange:     inRange,
		MaxDeltaE:   max,
		MeanDeltaE:  sum / float64(total),
	}, nil
}

// ErrNegativeThreshold is returned when threshold < 0.
var ErrNegativeThreshold = errors.New("helixqa/regression: brand threshold must be ≥ 0")
