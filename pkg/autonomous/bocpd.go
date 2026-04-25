// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"errors"
	"math"
)

// BOCPD implements Bayesian Online Change-Point Detection (Adams & MacKay,
// 2007, "Bayesian Online Changepoint Detection", arXiv:0710.3742).
//
// Model: the observations x_t are assumed Gaussian with unknown mean and
// variance. The Normal-Gamma conjugate prior closes under Bayesian update,
// giving a Student-t marginal predictive — no sampling required.
//
// Hazard function: constant rate 1/lambda. This encodes the belief that
// change points arrive as a memoryless process with expected run length
// lambda between them.
//
// On each Observe(x) call, BOCPD returns the posterior probability that
// t is a change point — i.e. P(r_t = 0 | x_{1:t}). In HelixQA this stream
// is the per-frame dHash Hamming distance; a change point marks the exact
// frame where the UI actually changed, complementing the cruder
// "is identical for N seconds?" logic in StagnationDetector.
//
// The run-length posterior is truncated to maxRunLen entries to keep the
// per-step cost bounded — with lambda=250 and maxRunLen=500, the tail
// below the truncation threshold carries < 1e-6 probability and the
// truncation is statistically invisible.
type BOCPD struct {
	hazard          float64 // per-step change-point probability (1/lambda)
	maxRunLen       int
	changeThreshold int
	alpha0          float64 // prior shape — degrees-of-freedom / 2
	beta0           float64 // prior scale
	mu0             float64 // prior mean
	kappa0          float64 // prior precision scaling

	// runProbs[r] = P(r_t = r | x_{1:t}) — the run-length posterior.
	runProbs []float64

	// Sufficient statistics of the Normal-Gamma posterior conditional on
	// each run length r. All four arrays are kept aligned with runProbs.
	alpha []float64
	beta  []float64
	mu    []float64
	kappa []float64

	lastChangeProb float64
	observations   int
}

// BOCPDConfig configures a fresh detector. Zero fields fall back to
// reasonable defaults for per-frame dHash Hamming streams.
type BOCPDConfig struct {
	// Hazard is the per-step change-point probability; 1/Lambda where
	// Lambda is the expected run length in steps. Default: 1/250.
	Hazard float64

	// MaxRunLen truncates the run-length posterior. Default: 500.
	MaxRunLen int

	// ChangeThreshold is the run-length cutoff used by
	// LastChangeProbability() — we return P(r_t ≤ ChangeThreshold),
	// i.e. how much posterior mass has collapsed to short runs. Under
	// constant-hazard BOCPD the point mass P(r_t = 0) is always equal
	// to the hazard (algebraic identity), so the actual change signal
	// lives in "how much of the posterior is at small r". Default: 3.
	ChangeThreshold int

	// Priors on the Normal-Gamma. Defaults are weakly informative:
	// Alpha0=1.0, Beta0=1.0, Mu0=0.0, Kappa0=1.0 — meaning a prior
	// expectation of zero Hamming distance (frames are identical by
	// default) with modest concentration so the first few observations
	// dominate.
	Alpha0, Beta0, Mu0, Kappa0 float64
}

// NewBOCPD builds a detector from cfg. Returns an error only on invalid
// numerics; with zero cfg it yields a sensible default.
func NewBOCPD(cfg BOCPDConfig) (*BOCPD, error) {
	if cfg.Hazard == 0 {
		cfg.Hazard = 1.0 / 250.0
	}
	if cfg.Hazard <= 0 || cfg.Hazard >= 1 {
		return nil, errors.New("BOCPD: Hazard must be in (0, 1)")
	}
	if cfg.MaxRunLen == 0 {
		cfg.MaxRunLen = 500
	}
	if cfg.MaxRunLen < 2 {
		return nil, errors.New("BOCPD: MaxRunLen must be ≥ 2")
	}
	if cfg.ChangeThreshold == 0 {
		cfg.ChangeThreshold = 3
	}
	if cfg.ChangeThreshold < 0 {
		return nil, errors.New("BOCPD: ChangeThreshold must be non-negative")
	}
	if cfg.Alpha0 == 0 {
		cfg.Alpha0 = 1.0
	}
	if cfg.Beta0 == 0 {
		cfg.Beta0 = 1.0
	}
	if cfg.Kappa0 == 0 {
		cfg.Kappa0 = 1.0
	}
	if cfg.Alpha0 <= 0 || cfg.Beta0 <= 0 || cfg.Kappa0 <= 0 {
		return nil, errors.New("BOCPD: Alpha0, Beta0, Kappa0 must be positive")
	}

	b := &BOCPD{
		hazard:          cfg.Hazard,
		maxRunLen:       cfg.MaxRunLen,
		changeThreshold: cfg.ChangeThreshold,
		alpha0:          cfg.Alpha0,
		beta0:           cfg.Beta0,
		mu0:             cfg.Mu0,
		kappa0:          cfg.Kappa0,
	}
	b.Reset()
	return b, nil
}

// Reset returns the detector to its initial (pre-observation) state.
func (b *BOCPD) Reset() {
	b.runProbs = []float64{1.0}
	b.alpha = []float64{b.alpha0}
	b.beta = []float64{b.beta0}
	b.mu = []float64{b.mu0}
	b.kappa = []float64{b.kappa0}
	b.lastChangeProb = 0
	b.observations = 0
}

// Observations returns the total number of Observe() calls since the last
// Reset().
func (b *BOCPD) Observations() int { return b.observations }

// LastChangeProbability returns the change-point probability emitted on
// the most recent Observe() call. Before any observation it returns 0.
func (b *BOCPD) LastChangeProbability() float64 { return b.lastChangeProb }

// Observe integrates one observation x into the posterior and returns the
// change-point probability — defined as P(r_t ≤ ChangeThreshold | x_{1:t}),
// the total posterior mass that has collapsed to short run lengths.
//
// Under constant-hazard BOCPD the point mass P(r_t = 0) is algebraically
// equal to the hazard on every step regardless of the observation, so it
// is useless as a change signal. The actual signal lives in how much of
// the mass moves to small r when the new observation disagrees with the
// high-r posterior — that is what this metric captures.
//
// Interpretation:
//
//   - Steady-state stable run: almost all mass sits at one large r,
//     so P(r ≤ k) ≈ hazard (small).
//   - Immediately after a change point: the new observation is far
//     more likely under fresh priors than under concentrated
//     high-r posteriors, so mass floods into r=1..3 and P(r ≤ k)
//     jumps toward 1.
//
// The first few samples (before the posterior has concentrated at a
// specific r) return values ≈ 1 trivially, since the run-length
// posterior is short and all of its mass is "at small r" by construction.
// Callers that only care about mid-stream change points should ignore
// the first ~5 observations.
func (b *BOCPD) Observe(x float64) float64 {
	n := len(b.runProbs)
	b.observations++

	// Predictive probability under each current run-length hypothesis.
	pi := make([]float64, n)
	for r := 0; r < n; r++ {
		pi[r] = studentTPDF(x, b.mu[r], b.kappa[r], b.alpha[r], b.beta[r])
	}

	// Build the next posterior. Index 0 is the change-point mass; indices
	// 1..n collect the growth mass (r_{t-1}=r-1 grown by one step).
	nextSize := n + 1
	if nextSize > b.maxRunLen {
		nextSize = b.maxRunLen
	}
	next := make([]float64, nextSize)

	// Growth: r_{t-1} = r → r_t = r+1 with prob (1 - hazard).
	for r := 0; r < n && r+1 < nextSize; r++ {
		next[r+1] = b.runProbs[r] * pi[r] * (1 - b.hazard)
	}
	// Change point: sum over all r of runProbs[r] * pi[r] * hazard.
	var cp float64
	for r := 0; r < n; r++ {
		cp += b.runProbs[r] * pi[r] * b.hazard
	}
	next[0] = cp

	// Normalize to a proper probability distribution.
	var sum float64
	for _, p := range next {
		sum += p
	}
	if sum > 0 {
		inv := 1.0 / sum
		for i := range next {
			next[i] *= inv
		}
	} else {
		// Degenerate — all likelihoods underflowed. Fall back to a
		// fresh prior at r=0. This is the only path that restarts
		// from scratch silently; it should essentially never fire on
		// real workloads.
		next = []float64{1.0}
	}

	// Update sufficient statistics. The r=0 run uses the original priors;
	// every other r carries forward the stats from its "parent" r-1,
	// updated by the new observation.
	nextAlpha := make([]float64, len(next))
	nextBeta := make([]float64, len(next))
	nextMu := make([]float64, len(next))
	nextKappa := make([]float64, len(next))

	nextAlpha[0] = b.alpha0
	nextBeta[0] = b.beta0
	nextMu[0] = b.mu0
	nextKappa[0] = b.kappa0

	for r := 1; r < len(next); r++ {
		parent := r - 1
		if parent >= len(b.mu) {
			parent = len(b.mu) - 1
		}
		k := b.kappa[parent]
		m := b.mu[parent]
		a := b.alpha[parent]
		be := b.beta[parent]

		nextKappa[r] = k + 1
		nextMu[r] = (k*m + x) / (k + 1)
		nextAlpha[r] = a + 0.5
		delta := x - m
		nextBeta[r] = be + k*delta*delta/(2*(k+1))
	}

	b.runProbs = next
	b.alpha = nextAlpha
	b.beta = nextBeta
	b.mu = nextMu
	b.kappa = nextKappa

	// Change signal: posterior mass at short run lengths. See the
	// Observe docstring for why this — not next[0] — is the right
	// quantity to thresh on.
	var cpProb float64
	limit := b.changeThreshold + 1
	if limit > len(next) {
		limit = len(next)
	}
	for i := 0; i < limit; i++ {
		cpProb += next[i]
	}
	b.lastChangeProb = cpProb
	return cpProb
}

// MostLikelyRunLength returns the argmax of the run-length posterior.
// Useful for diagnostics: during a stable segment this grows linearly;
// immediately after a change point it drops back to 0.
func (b *BOCPD) MostLikelyRunLength() int {
	best := 0
	var bestP float64
	for r, p := range b.runProbs {
		if p > bestP {
			bestP = p
			best = r
		}
	}
	return best
}

// studentTPDF returns the Student-t marginal predictive probability
// density under a Normal-Gamma posterior with the given sufficient stats.
//
// Formula (Bishop 2006, eq. 2.160 / Murphy 2012, §4.6.3.6):
//
//	df    = 2 * alpha
//	scale = sqrt( beta * (kappa+1) / (alpha * kappa) )
//	t     = (x - mu) / scale
//	pdf   = Γ((df+1)/2) / ( Γ(df/2) * sqrt(df π) * scale ) *
//	        (1 + t² / df)^{-(df+1)/2}
//
// Implemented via math.Lgamma for numerical stability in the
// Γ-ratio, then exponentiated back.
func studentTPDF(x, mu, kappa, alpha, beta float64) float64 {
	df := 2 * alpha
	scale := math.Sqrt(beta * (kappa + 1) / (alpha * kappa))
	if scale <= 0 || !isFinite(scale) {
		return 0
	}
	t := (x - mu) / scale

	lgNum, _ := math.Lgamma((df + 1) / 2)
	lgDen, _ := math.Lgamma(df / 2)
	logNorm := lgNum - lgDen - 0.5*math.Log(df*math.Pi) - math.Log(scale)
	logKernel := -((df + 1) / 2) * math.Log(1+t*t/df)

	p := math.Exp(logNorm + logKernel)
	if !isFinite(p) || p < 0 {
		return 0
	}
	return p
}

func isFinite(x float64) bool {
	return !math.IsNaN(x) && !math.IsInf(x, 0)
}
