// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package maestro

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlowRunner_BuildArgs_WithDevice(t *testing.T) {
	r := NewFlowRunner()
	args := r.buildArgs("flows/login.yaml", "emulator-5554")
	assert.Contains(t, args, "--device")
	assert.Contains(t, args, "emulator-5554")
	assert.Contains(t, args, "test")
	assert.Contains(t, args, "flows/login.yaml")
}

func TestFlowRunner_BuildArgs_NoDevice(t *testing.T) {
	r := NewFlowRunner()
	args := r.buildArgs("flows/login.yaml", "")
	assert.NotContains(t, args, "--device")
	assert.Contains(t, args, "test")
	assert.Contains(t, args, "flows/login.yaml")
}

func TestFlowRunner_ParseResult_Success(t *testing.T) {
	r := NewFlowRunner()
	output := "Running flow: flows/login.yaml\n1 Passed, 0 Failed\n"
	result, err := r.parseFlowResult(output)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	assert.Equal(t, 1, result.Passed)
	assert.Equal(t, 0, result.Failed)
}

func TestFlowRunner_ParseResult_Failure(t *testing.T) {
	r := NewFlowRunner()
	output := "Running flow: flows/login.yaml\n❌ Step failed\n0 Passed, 1 Failed\n"
	result, err := r.parseFlowResult(output)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Success)
	assert.Equal(t, 0, result.Passed)
	assert.Equal(t, 1, result.Failed)
}
