// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package agent_bridge adapts an automation.Engine for the
// pkg/nexus/agent state machine. The Agent (guided by the LLM)
// decides what automation.Action to emit; the Bridge calls
// Engine.Perform and returns the Result. The Bridge contains zero
// decision logic — ExecuteAction is a single delegating call.
package agent_bridge

import (
	"context"
	"errors"

	"digital.vasic.helixqa/pkg/nexus/automation"
)

// Bridge is the mechanical adapter between the pkg/nexus/agent state
// machine and an automation.Engine. It has no state of its own beyond
// the Engine reference.
//
// The Agent emits an automation.Action (decided by the LLM); Bridge
// calls Engine.Perform; the returned Result is fed back as a
// perception signal for the Agent's next planning turn.
type Bridge struct {
	Engine *automation.Engine
}

// NewBridge constructs a Bridge wrapping the given Engine. eng must
// be non-nil; passing nil produces an unusable Bridge that returns an
// error on every ExecuteAction call rather than panicking at
// construction time, so callers that build the Engine lazily can
// still construct a Bridge up front.
func NewBridge(eng *automation.Engine) *Bridge {
	return &Bridge{Engine: eng}
}

// ExecuteAction is the single entry point the Agent calls. It
// delegates to Engine.Perform without inspecting or modifying the
// Action — the LLM's decision passes through unchanged.
//
// Returns an error only when the Engine is nil or when Engine.Perform
// itself returns an error (unsupported ActionKind). Sub-engine
// failures (click missed, vision unavailable, …) are reported inside
// the Result, not as errors, so the Agent can fold them into its next
// planning prompt.
func (b *Bridge) ExecuteAction(ctx context.Context, a automation.Action) (automation.Result, error) {
	if b.Engine == nil {
		return automation.Result{}, errors.New("agent_bridge: Engine is nil")
	}
	return b.Engine.Perform(ctx, a)
}
