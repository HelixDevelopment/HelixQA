// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package helixqa

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewOrchestrator(t *testing.T) {
	o, err := NewOrchestrator()
	require.NoError(t, err)
	require.NotNil(t, o)
	assert.NotEmpty(t, o.repoRoot)
	assert.NotEmpty(t, o.evidenceDir)
}

func TestOrchestratorResults(t *testing.T) {
	o, err := NewOrchestrator()
	require.NoError(t, err)

	// Manually inject a result.
	o.results = append(o.results, TestResult{
		Type:   Unit,
		Passed: true,
	})

	results := o.Results()
	require.Len(t, results, 1)
	assert.Equal(t, Unit, results[0].Type)
	assert.True(t, results[0].Passed)
}

func TestOrchestratorSummary(t *testing.T) {
	o, err := NewOrchestrator()
	require.NoError(t, err)

	o.results = []TestResult{
		{Type: Unit, Passed: true, Duration: 1000000000},
		{Type: Smoke, Passed: false, Error: assert.AnError},
	}

	summary := o.Summary()
	assert.Contains(t, summary, "PASS")
	assert.Contains(t, summary, "FAIL")
	assert.Contains(t, summary, "1 passed, 1 failed")
}

// TestCaptureFailureEvidence_ReturnsSentinel is the round-29 §11.4
// anti-bluff regression test for the sentinel-default of
// captureFailureEvidence. Before the round-29 audit the function
// returned a single fabricated path string ("<evidenceDir>/<type>_failure.log")
// with the comment "Placeholder: in production this would capture
// screenshots, logs, etc." and (nil) — letting a downstream §11.4.2
// gate read a non-empty Evidence slice and conclude evidence had been
// captured when no file existed on disk. The orchestrator MUST now
// refuse to fabricate: empty slice + ErrEvidenceCaptureNotWired.
//
// Constitutional anchors: CONST-035 (anti-bluff), CONST-050(A)
// (no-fakes-beyond-unit-tests), Article XI §11.9 (forensic anchor).
func TestCaptureFailureEvidence_ReturnsSentinel(t *testing.T) {
	o, err := NewOrchestrator()
	require.NoError(t, err)

	paths, capErr := o.captureFailureEvidence(Unit, assert.AnError)

	// Honest contract: no fabricated paths.
	assert.Empty(t, paths, "captureFailureEvidence MUST NOT fabricate evidence paths until a real capture pipeline is wired")
	require.Error(t, capErr, "captureFailureEvidence MUST surface the gap, not silently pretend evidence was captured")
	assert.True(t, errors.Is(capErr, ErrEvidenceCaptureNotWired), "the surfaced error MUST be ErrEvidenceCaptureNotWired (got %v)", capErr)
}
