// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package verifier provides post-action verification primitives for the
// OCU P6 automation engine. Verifiers compare a before-frame and after-frame
// to decide whether an action produced the expected screen change.
//
// PixelVerifier delegates to VisionPipeline.Diff. MultiVerifier composes an
// ordered list of inner Verifiers with AND-semantics (first failure wins).
// Neither implementation synthesises decisions — they only observe and report.
package verifier

import (
	"context"
	"fmt"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// Verifier decides whether the screen changed as expected after an action.
// Implementations must be safe for concurrent use.
type Verifier interface {
	// Verify compares before and after frames and reports whether the
	// observed change satisfies the given expected description.
	// expected is the free-form string from Action.Expected; Verifiers
	// may use it for logging or ignore it — they must not act on it as
	// executable instructions.
	Verify(ctx context.Context, before, after contracts.Frame, expected string) (bool, error)
}

// PixelVerifier uses VisionPipeline.Diff to compare two frames. It returns
// true when DiffResult.TotalDelta is greater than or equal to Threshold.
// A Threshold of 0 always returns true (any pixel change counts).
//
// PixelVerifier does not swallow errors from Diff — any failure is returned
// so callers can treat it as a definitive "could not verify" signal rather
// than a silent pass.
type PixelVerifier struct {
	// Vision is the pipeline used to compute the per-pixel diff.
	Vision contracts.VisionPipeline

	// Threshold is the minimum TotalDelta required for Verify to return
	// true. Negative values behave the same as zero.
	Threshold float64
}

// Verify returns true when the pixel delta between before and after meets or
// exceeds Threshold. An error from Vision.Diff is returned unchanged.
func (p *PixelVerifier) Verify(ctx context.Context, before, after contracts.Frame, _ string) (bool, error) {
	if p.Vision == nil {
		return false, fmt.Errorf("verifier: PixelVerifier has nil VisionPipeline")
	}
	d, err := p.Vision.Diff(ctx, before, after)
	if err != nil {
		return false, fmt.Errorf("verifier: Diff failed: %w", err)
	}
	if d == nil {
		return false, fmt.Errorf("verifier: Diff returned nil result")
	}
	return d.TotalDelta >= p.Threshold, nil
}

// MultiVerifier runs a list of inner Verifiers in order and returns true only
// when every inner verifier passes. It short-circuits on the first false or
// the first error, returning it immediately without running subsequent
// verifiers.
//
// An empty Inner slice always returns (true, nil) — vacuous truth, consistent
// with the AND-semantics: no constraint is violated when no constraint exists.
type MultiVerifier struct {
	Inner []Verifier
}

// Verify passes when all inner verifiers pass. The first failure (false result
// or non-nil error) stops evaluation and is returned to the caller.
func (m *MultiVerifier) Verify(ctx context.Context, before, after contracts.Frame, expected string) (bool, error) {
	for i, v := range m.Inner {
		ok, err := v.Verify(ctx, before, after, expected)
		if err != nil {
			return false, fmt.Errorf("verifier: inner[%d] error: %w", i, err)
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}
