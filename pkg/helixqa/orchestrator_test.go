// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package helixqa

import (
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
