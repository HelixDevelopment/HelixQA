// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package webrtc_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/nexus/record/webrtc"
)

// TestPublisher_DefaultBindAddr verifies the safe default of 127.0.0.1.
func TestPublisher_DefaultBindAddr(t *testing.T) {
	p := webrtc.NewPublisher()
	assert.Equal(t, "127.0.0.1", p.BindAddr,
		"BindAddr must default to 127.0.0.1 (never 0.0.0.0 without explicit operator flag)")
	assert.False(t, p.OptIn, "OptIn must default to false")
}

// TestPublisher_DefaultOptIn_ReturnsErrNotWired verifies that Publish returns
// ErrNotWired when OptIn is false (the default).
func TestPublisher_DefaultOptIn_ReturnsErrNotWired(t *testing.T) {
	p := webrtc.NewPublisher()
	_, err := p.Publish(context.Background())
	require.ErrorIs(t, err, webrtc.ErrNotWired)
}

// TestPublisher_OptIn_StillReturnsErrNotWired verifies that even with OptIn=true
// the P5 stub still returns ErrNotWired (real WHIP lands in P5.5).
func TestPublisher_OptIn_StillReturnsErrNotWired(t *testing.T) {
	p := webrtc.NewPublisher()
	p.OptIn = true
	p.BindAddr = "127.0.0.1"
	p.BearerTok = "test-token"
	_, err := p.Publish(context.Background())
	require.ErrorIs(t, err, webrtc.ErrNotWired,
		"real WHIP session setup arrives in P5.5; P5 always returns ErrNotWired")
}
