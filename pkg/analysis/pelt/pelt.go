// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package pelt

import (
	"context"
	"errors"
	"math"
)

// Segmenter is the offline change-point-detection contract. Given a
// 1-D time series, a Segmenter returns the indices of change points
// between homogeneous segments.
//
// Change-point convention: the returned slice contains the *starting*
// index of each segment *after the first*. So for a series of length n
// with k change points at positions i₁ < i₂ < ... < iₖ, the segments
// are [0, i₁), [i₁, i₂), ..., [iₖ, n). The first segment (starting at
// 0) is implicit and not included in the return.
type Segmenter interface {
	Segment(ctx context.Context, series []float64, penalty float64) ([]int, error)
}

// PELT implements the Pruned Exact Linear Time change-point-detection
// algorithm (Killick, Fearnhead & Eckley, 2012, "Optimal Detection of
// Changepoints with a Linear Computational Cost", J. Amer. Stat.
// Assoc. 107:500, pp. 1590–1598).
//
// Why pure-Go over the ruptures Python sidecar:
//
//   - The Kickoff brief nominated a Python sidecar because ruptures
//     (https://centre-borelli.github.io/ruptures-docs/) is the
//     canonical reference implementation. That adds a network-bound
//     dependency (gRPC between the Go host and a Python process) for
//     what is ~120 LoC of dynamic programming.
//   - Post-session segmentation (the typical PELT workload in
//     HelixQA) runs on ≤ a few thousand samples — well within the
//     O(n²) naive complexity, and O(n) expected with PELT pruning.
//     No GPU or SIMD advantage to exploit.
//   - Same decision pattern as pkg/vision/perceptual/ssim.go —
//     keeping the Go host CGO-free wins across every deployment.
//
// Cost model: the default is a Gaussian mean-change log-likelihood
//
//	C(segment) = Σ (x - mean)²  over the segment
//
// — i.e. the sum of squared deviations from the segment mean. This
// matches the L2 cost in the ruptures reference and gives change
// points at abrupt level shifts. Other cost models (e.g. variance
// change) can be swapped in by providing a custom CostFunc.
type PELT struct {
	// MinSize is the minimum segment length. Smaller segments are
	// pruned from the dynamic-programming search so the algorithm
	// doesn't fire on single-sample spikes. Default: 2.
	MinSize int

	// Cost computes the cost of the segment series[a:b]. Zero →
	// defaults to gaussianMeanCost (L2 distance from the segment mean).
	// A custom Cost lets callers plug in variance-change, rbf-kernel,
	// or likelihood-based cost functions without touching the PELT
	// machinery itself.
	Cost CostFunc
}

// CostFunc returns the cost of treating series[a:b] as a single
// homogeneous segment. Smaller cost = stronger evidence the segment
// is stationary. Assume a < b and that Cost is symmetric in a, b-1.
type CostFunc func(series []float64, a, b int) float64

// Sentinel errors.
var (
	ErrEmptySeries     = errors.New("helixqa/analysis/pelt: empty series")
	ErrNegativePenalty = errors.New("helixqa/analysis/pelt: penalty must be non-negative")
	ErrInvalidMinSize  = errors.New("helixqa/analysis/pelt: MinSize must be ≥ 1")
)

// Segment returns the change-point indices for series under the given
// penalty. A larger penalty yields fewer change points (BIC-like
// behaviour); a good default is β = 2 * log(n) for length-n series
// under the Gaussian mean-change model.
func (p PELT) Segment(ctx context.Context, series []float64, penalty float64) ([]int, error) {
	if len(series) == 0 {
		return nil, ErrEmptySeries
	}
	if penalty < 0 {
		return nil, ErrNegativePenalty
	}
	minSize := p.MinSize
	if minSize == 0 {
		minSize = 2
	}
	if minSize < 1 {
		return nil, ErrInvalidMinSize
	}
	cost := p.Cost
	if cost == nil {
		cost = gaussianMeanCost
	}

	n := len(series)
	if n < 2*minSize {
		// Too short to have any interior change point.
		return nil, nil
	}

	// F[t] = minimum total cost to segment series[0:t+1].
	// prev[t] = last change-point index producing F[t].
	// R     = active change-point candidates (pruned set).
	F := make([]float64, n+1)
	prev := make([]int, n+1)
	F[0] = -penalty
	prev[0] = 0

	R := []int{0}

	for tau := minSize; tau <= n; tau++ {
		if tau%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}

		best := math.Inf(1)
		bestPrev := 0
		// Try every candidate change point s ∈ R: segment is (s, tau).
		for _, s := range R {
			if tau-s < minSize {
				continue
			}
			c := F[s] + cost(series, s, tau) + penalty
			if c < best {
				best = c
				bestPrev = s
			}
		}
		F[tau] = best
		prev[tau] = bestPrev

		// Prune: drop any s with F[s] + cost(s, tau) > F[tau] — those
		// candidates can never be improved upon by any future tau',
		// since cost is non-negative and only increases as the segment
		// grows.
		next := R[:0]
		for _, s := range R {
			if tau-s < minSize {
				next = append(next, s)
				continue
			}
			c := F[s] + cost(series, s, tau)
			if c <= F[tau] {
				next = append(next, s)
			}
		}
		// tau is always a valid candidate for future steps.
		next = append(next, tau)
		R = next
	}

	// Traceback.
	var cps []int
	t := n
	for t > 0 {
		s := prev[t]
		if s == 0 {
			break
		}
		cps = append([]int{s}, cps...)
		t = s
	}
	return cps, nil
}

// gaussianMeanCost is the default CostFunc — sum of squared deviations
// from the segment mean. Equivalent to negative log-likelihood under a
// Gaussian mean-change model with unit variance.
func gaussianMeanCost(series []float64, a, b int) float64 {
	if b <= a {
		return 0
	}
	var sum float64
	for i := a; i < b; i++ {
		sum += series[i]
	}
	mean := sum / float64(b-a)
	var cost float64
	for i := a; i < b; i++ {
		d := series[i] - mean
		cost += d * d
	}
	return cost
}

// VarianceCost is an exported cost function callers can plug in for
// variance-change detection — -log σ² where σ² is the sample variance
// of the segment. Long stable segments minimize this; spikes inflate it.
func VarianceCost(series []float64, a, b int) float64 {
	if b-a < 2 {
		return 0
	}
	var sum, sumSq float64
	for i := a; i < b; i++ {
		sum += series[i]
		sumSq += series[i] * series[i]
	}
	n := float64(b - a)
	mean := sum / n
	variance := sumSq/n - mean*mean
	if variance <= 0 {
		variance = 1e-12 // floor to avoid -Inf
	}
	return float64(b-a) * math.Log(variance)
}

// Compile-time guard.
var _ Segmenter = PELT{}
