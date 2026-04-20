// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"math"
	"math/rand/v2"
	"testing"
)

// ---------------------------------------------------------------------------
// NewBOCPD — config validation
// ---------------------------------------------------------------------------

func TestNewBOCPD_DefaultConfig(t *testing.T) {
	b, err := NewBOCPD(BOCPDConfig{})
	if err != nil {
		t.Fatalf("default cfg: %v", err)
	}
	if b.hazard != 1.0/250.0 {
		t.Fatalf("hazard = %v, want 1/250", b.hazard)
	}
	if b.maxRunLen != 500 {
		t.Fatalf("maxRunLen = %v, want 500", b.maxRunLen)
	}
	if b.alpha0 != 1 || b.beta0 != 1 || b.kappa0 != 1 {
		t.Fatalf("priors = (%v, %v, _, %v), want (1,1,_,1)", b.alpha0, b.beta0, b.kappa0)
	}
	if b.LastChangeProbability() != 0 {
		t.Fatalf("LastChangeProbability before any Observe = %v, want 0", b.LastChangeProbability())
	}
	if b.Observations() != 0 {
		t.Fatalf("Observations before any Observe = %d, want 0", b.Observations())
	}
}

func TestNewBOCPD_InvalidHazard(t *testing.T) {
	for _, h := range []float64{-0.1, 1.0, 1.5} {
		if _, err := NewBOCPD(BOCPDConfig{Hazard: h}); err == nil {
			t.Errorf("hazard %v: want error, got nil", h)
		}
	}
}

func TestNewBOCPD_InvalidMaxRunLen(t *testing.T) {
	if _, err := NewBOCPD(BOCPDConfig{MaxRunLen: 1}); err == nil {
		t.Fatal("MaxRunLen=1: want error")
	}
}

func TestNewBOCPD_InvalidPriors(t *testing.T) {
	cases := []BOCPDConfig{
		{Alpha0: -1},
		{Beta0: -1},
		{Kappa0: -1},
	}
	for i, c := range cases {
		if _, err := NewBOCPD(c); err == nil {
			t.Errorf("case %d (%+v): want error, got nil", i, c)
		}
	}
}

// ---------------------------------------------------------------------------
// Observe — first call boundary + algorithm sanity
// ---------------------------------------------------------------------------

func TestObserve_FirstCallProducesSaturatedChangeProb(t *testing.T) {
	b, _ := NewBOCPD(BOCPDConfig{Hazard: 0.01})
	cp := b.Observe(3.14)
	// At t=1 the run-length posterior has only r=0 and r=1, both of
	// which are ≤ ChangeThreshold=3 by default → cp ≈ 1 trivially.
	// This is why callers ignore the first few samples; the signal
	// becomes meaningful only after the posterior concentrates at some
	// stable large r.
	if math.Abs(cp-1.0) > 1e-9 {
		t.Fatalf("first change prob = %v, want ≈ 1 (all mass at small r)", cp)
	}
	if b.Observations() != 1 {
		t.Fatalf("Observations = %d, want 1", b.Observations())
	}
}

func TestObserve_StableStreamConvergesToLowChangeProbability(t *testing.T) {
	b, _ := NewBOCPD(BOCPDConfig{Hazard: 1.0 / 100})
	// Burn in — the first few samples return ≈ 1 trivially (posterior
	// hasn't had time to concentrate at a large r yet).
	for i := 0; i < 10; i++ {
		b.Observe(0)
	}
	// After burn-in the change probability should converge to a small
	// value as posterior mass concentrates at a single large r.
	for i := 0; i < 50; i++ {
		cp := b.Observe(0)
		if cp > 0.3 {
			t.Fatalf("stable stream step %d (post-burn-in): cp = %v, want ≤ 0.3", i+10, cp)
		}
	}
	if rl := b.MostLikelyRunLength(); rl < 40 {
		t.Fatalf("MostLikelyRunLength = %d, want ≥ 40 after 60 stable steps", rl)
	}
}

func TestObserve_StepChangeTriggersChangePoint(t *testing.T) {
	b, _ := NewBOCPD(BOCPDConfig{Hazard: 1.0 / 100})
	// Burn in with moderate-variance mean=0 noise — realistic dHash stream
	// scale, where a "stable UI" produces a few Hamming bits of jitter per
	// frame. Too-tight a prior variance makes the Student-t underflow on
	// larger shifts before the change is detected.
	r := rand.New(rand.NewPCG(1, 2))
	for i := 0; i < 30; i++ {
		b.Observe(r.NormFloat64() * 1.0)
	}
	preRL := b.MostLikelyRunLength()
	if preRL < 15 {
		t.Fatalf("burn-in run length %d too short — test will be noisy", preRL)
	}

	// Shift to a mean that is well above the burn-in scale but not so
	// extreme that the Student-t underflows — realistic for a real UI
	// transition producing ~15 Hamming bits of dHash change.
	var sawCP bool
	for i := 0; i < 10; i++ {
		cp := b.Observe(15 + r.NormFloat64()*1.0)
		if cp > 0.5 {
			sawCP = true
			break
		}
	}
	if !sawCP {
		t.Fatal("step change did not produce a change probability above 0.5 within 10 steps")
	}
}

func TestObserve_RunLengthResetsAfterChangePoint(t *testing.T) {
	b, _ := NewBOCPD(BOCPDConfig{Hazard: 1.0 / 50})
	r := rand.New(rand.NewPCG(42, 100))
	for i := 0; i < 30; i++ {
		b.Observe(r.NormFloat64() * 0.1)
	}
	// Shift
	for i := 0; i < 10; i++ {
		b.Observe(50 + r.NormFloat64()*0.1)
	}
	// After the shift, the posterior should re-concentrate at a small
	// run length — not still be peaked around 30+.
	if rl := b.MostLikelyRunLength(); rl > 12 {
		t.Fatalf("MostLikelyRunLength after shift = %d, want ≤ 12", rl)
	}
}

func TestObserve_ResetReturnsToPrior(t *testing.T) {
	b, _ := NewBOCPD(BOCPDConfig{})
	for i := 0; i < 20; i++ {
		b.Observe(float64(i))
	}
	if b.Observations() != 20 {
		t.Fatalf("Observations = %d, want 20", b.Observations())
	}
	b.Reset()
	if b.Observations() != 0 || b.LastChangeProbability() != 0 {
		t.Fatal("Reset did not clear observation state")
	}
	if got := len(b.runProbs); got != 1 {
		t.Fatalf("runProbs len after Reset = %d, want 1", got)
	}
}

func TestObserve_MaxRunLenTruncation(t *testing.T) {
	b, err := NewBOCPD(BOCPDConfig{MaxRunLen: 10})
	if err != nil {
		t.Fatal(err)
	}
	// Feed 50 samples — runProbs must cap at MaxRunLen.
	for i := 0; i < 50; i++ {
		b.Observe(0)
	}
	if got := len(b.runProbs); got > 10 {
		t.Fatalf("runProbs len = %d, want ≤ 10", got)
	}
}

// ---------------------------------------------------------------------------
// Student-t PDF — analytic spot checks
// ---------------------------------------------------------------------------

func TestStudentTPDF_SymmetricAroundMu(t *testing.T) {
	p1 := studentTPDF(5.0, 0, 1, 1, 1)
	p2 := studentTPDF(-5.0, 0, 1, 1, 1)
	if math.Abs(p1-p2) > 1e-12 {
		t.Fatalf("Student-t should be symmetric around mu: p(5)=%v p(-5)=%v", p1, p2)
	}
}

func TestStudentTPDF_PeakAtMu(t *testing.T) {
	pMu := studentTPDF(0, 0, 1, 1, 1)
	pOff := studentTPDF(2, 0, 1, 1, 1)
	if pMu <= pOff {
		t.Fatalf("Student-t should peak at mu: p(0)=%v p(2)=%v", pMu, pOff)
	}
}

func TestStudentTPDF_NonNegative(t *testing.T) {
	for _, x := range []float64{-10, -1, 0, 1, 10} {
		p := studentTPDF(x, 0, 1, 1, 1)
		if p < 0 || math.IsNaN(p) {
			t.Fatalf("studentTPDF(%v) = %v — must be non-negative", x, p)
		}
	}
}

func TestStudentTPDF_InvalidScale(t *testing.T) {
	// kappa=0 makes scale=+Inf → guard returns 0.
	if p := studentTPDF(0, 0, 0, 1, 1); p != 0 {
		t.Fatalf("kappa=0: pdf = %v, want 0 (guarded)", p)
	}
}

func TestObserve_UnderflowRecoversToFreshPrior(t *testing.T) {
	// Configure a very tight prior (tiny Beta0 → tiny variance →
	// Student-t collapses quickly). Feed modest observations to
	// concentrate the posterior, then slam with an extreme value
	// that drives every pi[r] below double-precision underflow.
	b, err := NewBOCPD(BOCPDConfig{
		Hazard:    1.0 / 1000,
		Alpha0:    50,
		Beta0:     1e-6,
		Kappa0:    50,
		MaxRunLen: 20,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 15; i++ {
		b.Observe(0)
	}
	// x = 1e200 is so far out that Student-t underflows for every r.
	b.Observe(1e200)
	if len(b.runProbs) != 1 || b.runProbs[0] != 1.0 {
		t.Fatalf("underflow recovery: runProbs = %v, want [1.0]", b.runProbs)
	}
}

func TestStudentTPDF_NaNInfInputsReturnZero(t *testing.T) {
	// Scale computation overflows → log argument is NaN/Inf → exp is
	// NaN → guarded to 0.
	if p := studentTPDF(1e300, 0, 1e-300, 1, 1e300); p != 0 {
		t.Fatalf("NaN-inducing inputs: pdf = %v, want 0", p)
	}
}

func TestIsFinite(t *testing.T) {
	cases := []struct {
		x    float64
		want bool
	}{
		{0, true},
		{1.5, true},
		{-100, true},
		{math.NaN(), false},
		{math.Inf(1), false},
		{math.Inf(-1), false},
	}
	for _, c := range cases {
		if got := isFinite(c.x); got != c.want {
			t.Errorf("isFinite(%v) = %v, want %v", c.x, got, c.want)
		}
	}
}
