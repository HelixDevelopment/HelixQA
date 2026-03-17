// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package detector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestExecRunner_Run_ValidCommand(t *testing.T) {
	r := &execRunner{}
	output, err := r.Run(
		context.Background(), "echo", "hello",
	)
	assert.NoError(t, err)
	assert.Contains(t, string(output), "hello")
}

func TestExecRunner_Run_InvalidCommand(t *testing.T) {
	r := &execRunner{}
	_, err := r.Run(
		context.Background(),
		"nonexistent-command-xyz",
	)
	assert.Error(t, err)
}

func TestExecRunner_Run_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &execRunner{}
	_, err := r.Run(ctx, "sleep", "10")
	assert.Error(t, err)
}

func TestExecRunner_Run_WithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(), 100*time.Millisecond,
	)
	defer cancel()
	r := &execRunner{}
	_, err := r.Run(ctx, "sleep", "10")
	assert.Error(t, err)
}

func TestExecRunner_Run_WithArgs(t *testing.T) {
	r := &execRunner{}
	output, err := r.Run(
		context.Background(),
		"echo", "-n", "test123",
	)
	assert.NoError(t, err)
	assert.Equal(t, "test123", string(output))
}
