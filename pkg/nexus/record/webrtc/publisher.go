// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package webrtc is the P5 WHIP publisher stub. Real WebRTC track
// publishing arrives in P5.5. Off by default: LiveStream returns
// ErrNotWired unless the operator explicitly opts in via --whip-bind
// + bearer token (per program spec §4.7).
package webrtc

import (
	"context"
	"errors"
)

// ErrNotWired is returned by Publish in P5. Real WHIP session setup
// (ICE, DTLS, RTP track) lands in P5.5.
var ErrNotWired = errors.New("record/webrtc: publisher not wired (opt-in required, real impl P5.5)")

// Publisher is the WHIP track publisher. OptIn controls whether
// LiveStream returns an actual URL. P5 implementation always
// returns ErrNotWired even when OptIn=true, until P5.5 lands.
//
// Security note: BindAddr defaults to "127.0.0.1" (loopback-only).
// Binding to "0.0.0.0" requires an explicit operator flag and a
// bearer token — never happens automatically.
type Publisher struct {
	OptIn     bool
	BindAddr  string // defaults to 127.0.0.1 — never 0.0.0.0 without explicit flag
	BearerTok string
}

// NewPublisher returns a Publisher with safe defaults: OptIn=false,
// BindAddr=127.0.0.1. Operators must explicitly set OptIn=true and
// supply BindAddr + BearerTok to enable the WHIP endpoint.
func NewPublisher() *Publisher {
	return &Publisher{
		BindAddr: "127.0.0.1",
	}
}

// Publish returns a WHIP URL the caller can publish to. P5 scope:
// always returns ErrNotWired regardless of OptIn. Real ICE/DTLS/RTP
// track publishing arrives in P5.5.
func (p *Publisher) Publish(_ context.Context) (string, error) {
	return "", ErrNotWired
}
