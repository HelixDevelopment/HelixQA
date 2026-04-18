package ai

import (
	"math"
	"sync"
)

// FlakeSample is one historical observation of a test run.
type FlakeSample struct {
	TestID    string
	Platform  string
	Pass      bool
	DurationS float64
	Retries   int
	// Extracted features used by the logistic predictor.
	HourOfDay int
	RunnerRSS int
}

// Predictor is a minimalist logistic-regression flake detector. It is
// good enough for a first pass and keeps us CGo-free; swap to an ONNX
// runtime model later without breaking callers.
type Predictor struct {
	mu      sync.RWMutex
	weight  map[string]float64 // feature name -> weight
	bias    float64
	history []FlakeSample
}

// NewPredictor returns a Predictor with reasonable default weights.
// Operators can call Train to refit from a batch of samples.
func NewPredictor() *Predictor {
	return &Predictor{
		weight: map[string]float64{
			"retries":   0.45,
			"duration":  0.02,
			"hour_late": 0.18,
			"rss_gb":    0.12,
		},
		bias: -0.9,
	}
}

// Probability reports the flake probability in [0,1] for a prospective
// run with the supplied features.
func (p *Predictor) Probability(s FlakeSample) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	z := p.bias
	z += p.weight["retries"] * float64(s.Retries)
	z += p.weight["duration"] * s.DurationS
	if s.HourOfDay >= 22 || s.HourOfDay < 5 {
		z += p.weight["hour_late"]
	}
	z += p.weight["rss_gb"] * float64(s.RunnerRSS/1_073_741_824)
	return sigmoid(z)
}

// Decide returns true when the caller should pre-emptively retry or
// skip the test based on the probability threshold.
func (p *Predictor) Decide(s FlakeSample, threshold float64) bool {
	if threshold <= 0 {
		threshold = 0.5
	}
	return p.Probability(s) >= threshold
}

// Observe records a sample. Training logic is deliberately tiny: we
// adjust the bias with a gentle step so the predictor learns from pass/
// fail without a full SGD loop.
func (p *Predictor) Observe(s FlakeSample) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.history = append(p.history, s)
	// Hebbian-ish nudge: each failure pulls bias up, each pass pulls it down.
	if !s.Pass {
		p.bias += 0.01
	} else {
		p.bias -= 0.005
	}
	// Clamp bias to sensible range.
	if p.bias < -3 {
		p.bias = -3
	} else if p.bias > 3 {
		p.bias = 3
	}
}

// History returns a copy of observed samples.
func (p *Predictor) History() []FlakeSample {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]FlakeSample, len(p.history))
	copy(out, p.history)
	return out
}

func sigmoid(z float64) float64 {
	return 1.0 / (1.0 + math.Exp(-z))
}
