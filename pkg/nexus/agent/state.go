// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"time"

	"digital.vasic.helixqa/pkg/nexus"
)

// Phase names each explicit step of the agent loop. Exported as
// constants so callers that hook into telemetry can filter.
type Phase string

const (
	PhasePrepare     Phase = "prepare"
	PhasePlan        Phase = "plan"
	PhaseExecute     Phase = "execute"
	PhasePostProcess Phase = "postprocess"
)

// AgentStep is the typed output of one planning iteration. Mirrors
// browser-use's AgentOutput (evaluate / memory / next_goal / actions)
// so every tool that consumed the reference shape keeps working.
type AgentStep struct {
	// Iteration is the 1-based index of this step inside the Agent's
	// history. Zero means "not yet recorded".
	Iteration int

	// Phase is the phase that produced this step.
	Phase Phase

	// Evaluation is the LLM's judgement on the last step's outcome
	// ("goal reached", "need more input", "previous action failed
	// because ...").
	Evaluation string

	// Memory is the rolling working-memory summary the LLM updates
	// each iteration. Used by the MessageManager in Phase 3 of the
	// OpenClawing2 plan to bound prompt size.
	Memory string

	// NextGoal is the LLM's stated objective for the next Execute
	// phase. Empty when Phase == PhaseExecute (goal already consumed)
	// or PhasePostProcess.
	NextGoal string

	// Actions are the concrete browser / desktop / mobile actions
	// the Execute phase should dispatch in order.
	Actions []nexus.Action

	// Done signals the Agent loop to stop. Planners set this when
	// the task's goal is complete.
	Done bool

	// PlannedAt records when this step's plan was produced. Filled
	// by the planner; may be zero for unit-test fixtures.
	PlannedAt time.Time

	// Results carries the outcomes of Execute phase dispatches. Empty
	// until after Phase 3 runs.
	Results []ActionResult
}

// ActionResult captures the outcome of a single Adapter.Do call.
// Kept separate from AgentStep.Actions so the planner's intent and
// the runtime's observation stay distinguishable during debugging.
type ActionResult struct {
	Action   nexus.Action
	Success  bool
	Err      string
	Duration time.Duration
}

// AgentState is the across-steps state the Agent maintains. History
// is append-only; every completed iteration records an AgentStep
// snapshot so retros + self-healing prompts have ground truth.
type AgentState struct {
	// TaskGoal is the user-supplied objective the Agent drives
	// toward. Never mutated after construction.
	TaskGoal string

	// SessionID uniquely identifies this Agent run — typically the
	// ID of the underlying nexus.Session. Used as a key in
	// telemetry + evidence storage.
	SessionID string

	// Iteration is the count of completed Step() calls.
	Iteration int

	// CurrentPhase is the phase currently in flight. Useful for
	// observability middleware that emits per-phase spans.
	CurrentPhase Phase

	// History is the ordered log of every AgentStep produced,
	// including failed or aborted iterations.
	History []AgentStep

	// Snapshot is the most recently captured nexus.Snapshot. Nil
	// until the first PrepareContext has run.
	Snapshot *nexus.Snapshot

	// Screenshot is a cache of the most recent visual frame keyed
	// to Snapshot. Planners send this in the user message so the
	// LLM sees what the engine sees.
	Screenshot []byte

	// Done mirrors AgentStep.Done so Run() can terminate cleanly
	// without inspecting the whole History.
	Done bool

	// LastError, when non-nil, is the most recent terminal failure
	// from any phase. Non-terminal retry failures live in
	// History[].Results instead.
	LastError error
}

// NewAgentState returns a zero-value AgentState bound to taskGoal +
// sessionID. History + Snapshot + Screenshot are initialised lazily.
func NewAgentState(taskGoal, sessionID string) *AgentState {
	return &AgentState{
		TaskGoal:  taskGoal,
		SessionID: sessionID,
	}
}

// RecentSteps returns the last n steps in chronological order (oldest
// first), or all of History when len(History) <= n. Callers use this
// to build the "previous attempts" section of the self-healing
// prompt without dragging the whole history into the context.
func (s *AgentState) RecentSteps(n int) []AgentStep {
	if n <= 0 || len(s.History) == 0 {
		return nil
	}
	if n >= len(s.History) {
		out := make([]AgentStep, len(s.History))
		copy(out, s.History)
		return out
	}
	out := make([]AgentStep, n)
	copy(out, s.History[len(s.History)-n:])
	return out
}

// AppendStep adds step to History + increments Iteration. Returns
// the iteration index assigned to the step.
func (s *AgentState) AppendStep(step AgentStep) int {
	s.Iteration++
	step.Iteration = s.Iteration
	s.History = append(s.History, step)
	if step.Done {
		s.Done = true
	}
	return s.Iteration
}
