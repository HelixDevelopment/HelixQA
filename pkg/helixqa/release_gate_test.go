// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package helixqa

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseGateEvaluateAllPass(t *testing.T) {
	g := NewReleaseGate()

	results := []TestResult{
		{Type: Unit, Passed: true},
		{Type: Integration, Passed: true},
		{Type: E2E, Passed: true},
		{Type: Security, Passed: true},
		{Type: Benchmark, Passed: true},
		{Type: Chaos, Passed: true},
		{Type: Stress, Passed: true},
		{Type: Smoke, Passed: true},
		{Type: Challenge, Passed: true, Evidence: []string{"/tmp/evidence.png"}},
	}

	err := g.Evaluate(results)
	assert.NoError(t, err)
	assert.True(t, g.IsOpen(results))
}

func TestReleaseGateEvaluateMissingType(t *testing.T) {
	g := NewReleaseGate()

	results := []TestResult{
		{Type: Unit, Passed: true},
		// Missing Integration and others.
	}

	err := g.Evaluate(results)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing test types")
}

func TestReleaseGateEvaluateFailedType(t *testing.T) {
	g := NewReleaseGate()

	results := []TestResult{
		{Type: Unit, Passed: true},
		{Type: Integration, Passed: true},
		{Type: E2E, Passed: true},
		{Type: Security, Passed: true},
		{Type: Benchmark, Passed: true},
		{Type: Chaos, Passed: true},
		{Type: Stress, Passed: true},
		{Type: Smoke, Passed: true},
		{Type: Challenge, Passed: false, Error: fmt.Errorf("challenge failed")},
	}

	err := g.Evaluate(results)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed test types")
}

func TestReleaseGateEvaluateNoEvidence(t *testing.T) {
	g := NewReleaseGate()

	results := []TestResult{
		{Type: Unit, Passed: true},
		{Type: Integration, Passed: true},
		{Type: E2E, Passed: true},
		{Type: Security, Passed: true},
		{Type: Benchmark, Passed: true},
		{Type: Chaos, Passed: true},
		{Type: Stress, Passed: true},
		{Type: Smoke, Passed: true},
		{Type: Challenge, Passed: true, Evidence: nil},
	}

	err := g.Evaluate(results)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "visual evidence")
}

func TestReleaseGateEvaluateWithCells(t *testing.T) {
	g := NewReleaseGate()

	results := []TestResult{
		{Type: Unit, Passed: true},
		{Type: Integration, Passed: true},
		{Type: E2E, Passed: true},
		{Type: Security, Passed: true},
		{Type: Benchmark, Passed: true},
		{Type: Chaos, Passed: true},
		{Type: Stress, Passed: true},
		{Type: Smoke, Passed: true},
		{Type: Challenge, Passed: true, Evidence: []string{"/tmp/evidence.png"}},
	}

	err := g.EvaluateWithCells(results, 1840, 1840)
	assert.NoError(t, err)

	// Not enough cells total.
	err = g.EvaluateWithCells(results, 100, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "minimum 1840 required")

	// Some cells failed.
	err = g.EvaluateWithCells(results, 1839, 1840)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only 1839/1840 cells passed")
}
