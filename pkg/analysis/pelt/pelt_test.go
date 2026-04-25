// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package pelt

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"testing"
)

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

// stepSeries returns a deterministic step signal: the first `split`
// samples are centred at μ₁, the rest at μ₂, with Gaussian noise of the
// given scale. The caller seeds the PRNG for reproducibility.
func stepSeries(split, total int, mu1, mu2, noise float64, seed uint64) []float64 {
	r := rand.New(rand.NewPCG(seed, seed*0x9E3779B97F4A7C15))
	out := make([]float64, total)
	for i := 0; i < split; i++ {
		out[i] = mu1 + r.NormFloat64()*noise
	}
	for i := split; i < total; i++ {
		out[i] = mu2 + r.NormFloat64()*noise
	}
	return out
}

// ---------------------------------------------------------------------------
// Happy path
// ---------------------------------------------------------------------------

func TestPELT_DetectsStepChange(t *testing.T) {
	series := stepSeries(100, 200, 0, 5, 0.5, 42)
	p := PELT{}
	cps, err := p.Segment(context.Background(), series, 10)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(cps) == 0 {
		t.Fatal("step change not detected")
	}
	// Within a reasonable window around the true split point (100).
	near := false
	for _, cp := range cps {
		if cp >= 95 && cp <= 105 {
			near = true
			break
		}
	}
	if !near {
		t.Fatalf("detected change points %v — none near the true split at 100", cps)
	}
}

func TestPELT_DetectsMultipleChanges(t *testing.T) {
	// Build a signal with three distinct level-shift segments.
	r := rand.New(rand.NewPCG(1, 2))
	n := 300
	series := make([]float64, n)
	for i := 0; i < 100; i++ {
		series[i] = 0 + r.NormFloat64()*0.3
	}
	for i := 100; i < 200; i++ {
		series[i] = 5 + r.NormFloat64()*0.3
	}
	for i := 200; i < 300; i++ {
		series[i] = -2 + r.NormFloat64()*0.3
	}

	p := PELT{}
	cps, err := p.Segment(context.Background(), series, 15)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(cps) < 2 {
		t.Fatalf("expected ≥ 2 change points for three-segment signal, got %v", cps)
	}
}

func TestPELT_ConstantSignalYieldsNoChangePoints(t *testing.T) {
	series := make([]float64, 100)
	for i := range series {
		series[i] = 3.14
	}
	p := PELT{}
	cps, err := p.Segment(context.Background(), series, 1)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(cps) != 0 {
		t.Fatalf("constant signal produced %d change points: %v", len(cps), cps)
	}
}

func TestPELT_HigherPenaltyYieldsFewerOrEqualChangePoints(t *testing.T) {
	series := stepSeries(50, 100, 0, 3, 0.5, 5)
	series = append(series, stepSeries(50, 100, 3, 0, 0.5, 6)...)

	p := PELT{}
	low, _ := p.Segment(context.Background(), series, 5)
	high, _ := p.Segment(context.Background(), series, 100)

	if len(high) > len(low) {
		t.Fatalf("higher penalty produced MORE change points (%d > %d)", len(high), len(low))
	}
}

// ---------------------------------------------------------------------------
// Change-point return-shape invariants
// ---------------------------------------------------------------------------

func TestPELT_ChangePointsAreStrictlyIncreasing(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 8))
	n := 500
	series := make([]float64, n)
	for i := 0; i < n; i++ {
		switch {
		case i < 120:
			series[i] = 0 + r.NormFloat64()*0.2
		case i < 230:
			series[i] = 4 + r.NormFloat64()*0.2
		case i < 370:
			series[i] = -1 + r.NormFloat64()*0.2
		default:
			series[i] = 2 + r.NormFloat64()*0.2
		}
	}
	cps, err := PELT{}.Segment(context.Background(), series, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(cps) == 0 {
		t.Fatal("expected some change points on a 4-segment signal")
	}
	for i := 1; i < len(cps); i++ {
		if cps[i] <= cps[i-1] {
			t.Fatalf("change points not strictly increasing at index %d: %v", i, cps)
		}
	}
	// Every change point is within (0, n).
	for _, cp := range cps {
		if cp <= 0 || cp >= n {
			t.Fatalf("change point %d out of range (0, %d)", cp, n)
		}
	}
}

// ---------------------------------------------------------------------------
// Config + defaults
// ---------------------------------------------------------------------------

func TestPELT_CustomMinSizePreservedInOutput(t *testing.T) {
	series := stepSeries(30, 60, 0, 5, 0.5, 99)
	p := PELT{MinSize: 10}
	cps, err := p.Segment(context.Background(), series, 5)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	// With MinSize=10 and series length 60, no change point can be
	// closer than 10 to either end.
	for _, cp := range cps {
		if cp < 10 || cp > 60-10 {
			t.Fatalf("change point %d violates MinSize=10 on n=60", cp)
		}
	}
}

func TestPELT_CustomCostFunc(t *testing.T) {
	// A bespoke cost function that treats every segment as free (cost
	// 0). Under that cost, the optimal segmentation is the one with
	// the fewest segments (change points) because penalty > 0 is the
	// only force. Expect 0 change points.
	zeroCost := func(series []float64, a, b int) float64 { return 0 }
	p := PELT{Cost: zeroCost}
	series := stepSeries(50, 100, 0, 10, 0.2, 1)
	cps, err := p.Segment(context.Background(), series, 1)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(cps) != 0 {
		t.Fatalf("zero-cost segmenter produced %d change points, want 0", len(cps))
	}
}

func TestPELT_VarianceCostDetectsScaleShift(t *testing.T) {
	// First half: tight noise around 0. Second half: wider noise
	// around 0. Mean is constant throughout — only variance changes,
	// so gaussianMeanCost would miss it but VarianceCost detects it.
	r := rand.New(rand.NewPCG(42, 100))
	series := make([]float64, 200)
	for i := 0; i < 100; i++ {
		series[i] = r.NormFloat64() * 0.2
	}
	for i := 100; i < 200; i++ {
		series[i] = r.NormFloat64() * 3.0
	}
	p := PELT{Cost: VarianceCost}
	cps, err := p.Segment(context.Background(), series, 10)
	if err != nil {
		t.Fatalf("Segment: %v", err)
	}
	if len(cps) == 0 {
		t.Fatal("variance shift not detected by VarianceCost")
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestPELT_EmptySeriesError(t *testing.T) {
	p := PELT{}
	if _, err := p.Segment(context.Background(), nil, 1); !errors.Is(err, ErrEmptySeries) {
		t.Fatalf("nil series: %v, want ErrEmptySeries", err)
	}
	if _, err := p.Segment(context.Background(), []float64{}, 1); !errors.Is(err, ErrEmptySeries) {
		t.Fatalf("empty series: %v, want ErrEmptySeries", err)
	}
}

func TestPELT_NegativePenaltyError(t *testing.T) {
	p := PELT{}
	if _, err := p.Segment(context.Background(), []float64{1, 2, 3}, -0.5); !errors.Is(err, ErrNegativePenalty) {
		t.Fatalf("negative penalty: %v, want ErrNegativePenalty", err)
	}
}

func TestPELT_InvalidMinSizeError(t *testing.T) {
	p := PELT{MinSize: -1}
	if _, err := p.Segment(context.Background(), []float64{1, 2, 3, 4}, 1); !errors.Is(err, ErrInvalidMinSize) {
		t.Fatalf("invalid MinSize: %v, want ErrInvalidMinSize", err)
	}
}

func TestPELT_TooShortReturnsEmpty(t *testing.T) {
	p := PELT{} // default MinSize = 2
	cps, err := p.Segment(context.Background(), []float64{1, 2, 3}, 1)
	if err != nil {
		t.Fatalf("too-short: %v", err)
	}
	if len(cps) != 0 {
		t.Fatalf("too-short series produced %d change points", len(cps))
	}
}

func TestPELT_ContextCanceled(t *testing.T) {
	// 512+ samples are enough to pass the every-256-step ctx.Err()
	// check point; pre-cancel + a long series reliably triggers it.
	series := make([]float64, 1024)
	for i := range series {
		series[i] = float64(i)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := PELT{}
	if _, err := p.Segment(ctx, series, 1); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

// ---------------------------------------------------------------------------
// Cost-function unit tests
// ---------------------------------------------------------------------------

func TestGaussianMeanCost_ZeroForConstantSegment(t *testing.T) {
	series := []float64{5, 5, 5, 5, 5}
	if got := gaussianMeanCost(series, 0, 5); math.Abs(got) > 1e-12 {
		t.Fatalf("constant segment cost = %v, want 0", got)
	}
}

func TestGaussianMeanCost_PositiveForVaryingSegment(t *testing.T) {
	series := []float64{0, 10, 0, 10, 0}
	if got := gaussianMeanCost(series, 0, 5); got <= 0 {
		t.Fatalf("varying segment cost = %v, want > 0", got)
	}
}

func TestGaussianMeanCost_ZeroForEmptySegment(t *testing.T) {
	if got := gaussianMeanCost([]float64{1, 2, 3}, 2, 2); got != 0 {
		t.Fatalf("empty segment cost = %v, want 0", got)
	}
}

func TestVarianceCost_PenalizesHighVariance(t *testing.T) {
	lowVar := []float64{5, 5.1, 4.9, 5.05, 4.95}
	highVar := []float64{5, 50, -40, 100, -100}
	lo := VarianceCost(lowVar, 0, len(lowVar))
	hi := VarianceCost(highVar, 0, len(highVar))
	if hi <= lo {
		t.Fatalf("variance cost: low=%v hi=%v — should have hi > lo", lo, hi)
	}
}

func TestVarianceCost_TooShortReturnsZero(t *testing.T) {
	if got := VarianceCost([]float64{5}, 0, 1); got != 0 {
		t.Fatalf("1-sample variance = %v, want 0", got)
	}
}

func TestVarianceCost_ZeroVarianceFloor(t *testing.T) {
	series := []float64{5, 5, 5, 5}
	// Constant segment: variance=0, cost formula uses floor 1e-12 to
	// avoid -Inf. Result is finite and very negative (log of tiny #).
	got := VarianceCost(series, 0, 4)
	if math.IsInf(got, 0) || math.IsNaN(got) {
		t.Fatalf("zero-variance cost = %v, want finite", got)
	}
}

// ---------------------------------------------------------------------------
// Interface conformance
// ---------------------------------------------------------------------------

func TestPELT_SatisfiesSegmenterInterface(t *testing.T) {
	var s Segmenter = PELT{}
	series := stepSeries(20, 40, 0, 5, 0.5, 123)
	if _, err := s.Segment(context.Background(), series, 10); err != nil {
		t.Fatalf("Segment via interface: %v", err)
	}
}
