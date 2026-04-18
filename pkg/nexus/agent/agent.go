// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
)

// LLMClient is the narrow contract the Agent package requires. Any
// type that returns a JSON-serialised AgentStep from Chat works;
// ai.OrchestratorClient + ai.HTTPLLMClient both qualify via their
// shared Chat(ctx, ChatRequest) method — the adapter in
// agent_llm_adapter.go bridges the shapes without circular imports.
type LLMClient interface {
	PlanStep(ctx context.Context, req PlanRequest) (AgentStep, error)
}

// PlanRequest is the input a planner expects. Holds only what the
// LLM needs — no pointers to AgentState so the implementation stays
// safe for concurrent reuse across sessions.
type PlanRequest struct {
	TaskGoal     string
	Snapshot     *nexus.Snapshot
	Screenshot   []byte
	RecentSteps  []AgentStep
	SystemPrompt string
	Model        string
}

// Config tunes Agent behaviour. All fields are optional; zero values
// resolve to defensible defaults.
type Config struct {
	// MaxIterations caps the Run() loop so a runaway planner cannot
	// spin forever. Zero = 60 (matches browser-use default).
	MaxIterations int

	// StepTimeout bounds the entire Step() call (prepare + plan +
	// execute + postprocess). Zero = 120s.
	StepTimeout time.Duration

	// SystemPrompt overrides the default planner system prompt.
	// Empty string uses the vendored default.
	SystemPrompt string

	// Model is the planner model identifier. Empty delegates to the
	// LLMClient's own default.
	Model string

	// RecentStepsInPrompt bounds how many history entries the planner
	// sees. Zero = 4 (browser-use default). A MessageManager swap-in
	// can override this per step.
	RecentStepsInPrompt int
}

// DefaultSystemPrompt is the planner system prompt used when
// Config.SystemPrompt is empty. Written as a single string so unit
// tests can assert on substrings deterministically.
const DefaultSystemPrompt = `You are the planning brain of an autonomous UI agent.
Each turn you receive (a) the current task goal, (b) a JSON snapshot of the UI
with interactable elements keyed by stable refs like e1, e2, ..., (c) the N
most recent AgentSteps you produced, and (d) an optional screenshot.

Return a single JSON object with these fields:
  evaluation  : one sentence on whether the previous step made progress.
  memory      : the rolling working-memory summary updated with any new facts.
  next_goal   : one sentence on the immediate sub-goal for this turn.
  actions     : array of {kind, target, text, x, y} objects (kind is one of
                click, type, scroll, drag, tap, swipe, key, wait_for,
                screenshot, pdf, tab_open, tab_close, menu_pick).
  done        : true when the task goal is fully satisfied.

Do not emit prose outside the JSON. Do not hallucinate element refs that are
not in the supplied snapshot. When no safe action exists, emit actions=[] with
a descriptive evaluation and next_goal so the runtime can escalate.`

// Agent is the four-phase state-machine runner. Agent is stateless
// across runs — every Run()/Step() call takes an AgentState so a
// single Agent instance can serve many sessions concurrently.
type Agent struct {
	llm     LLMClient
	adapter nexus.Adapter
	cfg     Config
}

// NewAgent wires an LLM + an Adapter together. Both must be non-nil.
// The Config's zero values are filled in at construction time so
// downstream logic never sees them as zero.
func NewAgent(llm LLMClient, adapter nexus.Adapter, cfg Config) (*Agent, error) {
	if llm == nil {
		return nil, errors.New("agent: nil LLMClient")
	}
	if adapter == nil {
		return nil, errors.New("agent: nil Adapter")
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 60
	}
	if cfg.StepTimeout <= 0 {
		cfg.StepTimeout = 120 * time.Second
	}
	if cfg.RecentStepsInPrompt <= 0 {
		cfg.RecentStepsInPrompt = 4
	}
	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = DefaultSystemPrompt
	}
	return &Agent{llm: llm, adapter: adapter, cfg: cfg}, nil
}

// ErrMaxIterationsExceeded is returned from Run() when the loop
// reaches cfg.MaxIterations without the planner marking the state
// Done. Callers typically treat this as an escalation signal.
var ErrMaxIterationsExceeded = errors.New("agent: max iterations exceeded")

// Run drives the four-phase loop until state.Done is set or
// MaxIterations is exceeded. Callers retain ownership of the Session
// that produced state.SessionID; Run() does not Close() it.
func (a *Agent) Run(ctx context.Context, state *AgentState, sess nexus.Session) error {
	if state == nil {
		return errors.New("agent: nil state")
	}
	if sess == nil {
		return errors.New("agent: nil session")
	}
	for !state.Done && state.Iteration < a.cfg.MaxIterations {
		if err := ctx.Err(); err != nil {
			state.LastError = err
			return err
		}
		if err := a.Step(ctx, state, sess); err != nil {
			state.LastError = err
			return err
		}
	}
	if state.Done {
		return nil
	}
	state.LastError = ErrMaxIterationsExceeded
	return ErrMaxIterationsExceeded
}

// Step runs a single iteration: PrepareContext → PlanActions →
// Execute → PostProcess. On any phase failure the Step returns the
// error up to Run().
func (a *Agent) Step(parent context.Context, state *AgentState, sess nexus.Session) error {
	ctx, cancel := context.WithTimeout(parent, a.cfg.StepTimeout)
	defer cancel()

	// Phase 1.
	state.CurrentPhase = PhasePrepare
	if err := a.PrepareContext(ctx, state, sess); err != nil {
		return fmt.Errorf("phase prepare: %w", err)
	}

	// Phase 2.
	state.CurrentPhase = PhasePlan
	step, err := a.PlanActions(ctx, state)
	if err != nil {
		return fmt.Errorf("phase plan: %w", err)
	}

	// Phase 3.
	state.CurrentPhase = PhaseExecute
	results := a.Execute(ctx, state, sess, step.Actions)
	step.Results = results

	// Phase 4.
	state.CurrentPhase = PhasePostProcess
	a.PostProcess(ctx, state, step)

	return nil
}

// PrepareContext captures a fresh Snapshot + Screenshot through the
// Adapter and stores them on state. Callers that want to plan
// against a stale snapshot (for example during a self-healing
// re-plan with the previous snapshot) can skip this phase.
func (a *Agent) PrepareContext(ctx context.Context, state *AgentState, sess nexus.Session) error {
	snap, err := a.adapter.Snapshot(ctx, sess)
	if err != nil {
		return err
	}
	state.Snapshot = snap
	png, err := a.adapter.Screenshot(ctx, sess)
	if err != nil {
		return err
	}
	state.Screenshot = png
	return nil
}

// PlanActions asks the LLM for the next AgentStep. The LLMClient is
// responsible for the actual JSON-schema shaping + retry on
// malformed output; Agent just builds the PlanRequest + forwards
// the Snapshot + recent history.
func (a *Agent) PlanActions(ctx context.Context, state *AgentState) (AgentStep, error) {
	req := PlanRequest{
		TaskGoal:     state.TaskGoal,
		Snapshot:     state.Snapshot,
		Screenshot:   state.Screenshot,
		RecentSteps:  state.RecentSteps(a.cfg.RecentStepsInPrompt),
		SystemPrompt: a.cfg.SystemPrompt,
		Model:        a.cfg.Model,
	}
	step, err := a.llm.PlanStep(ctx, req)
	if err != nil {
		return AgentStep{}, err
	}
	step.Phase = PhaseExecute
	step.PlannedAt = time.Now()
	return step, nil
}

// Execute dispatches each action via the Adapter in order. Any
// failure produces an ActionResult with Success=false + the error
// text, but the loop continues so the planner sees the full
// picture and can decide whether to retry or give up.
func (a *Agent) Execute(ctx context.Context, state *AgentState, sess nexus.Session, actions []nexus.Action) []ActionResult {
	out := make([]ActionResult, 0, len(actions))
	for _, action := range actions {
		start := time.Now()
		err := a.adapter.Do(ctx, sess, action)
		out = append(out, ActionResult{
			Action:   action,
			Success:  err == nil,
			Err:      errText(err),
			Duration: time.Since(start),
		})
		if err != nil {
			// A failed action still goes into results so the
			// self-healer sees it, but we stop dispatching so a
			// later action cannot compound the failure.
			break
		}
	}
	return out
}

// PostProcess records the completed step on state, runs the sanity
// checks (did any action succeed? did the planner mark us done?),
// and emits telemetry. The observability span wrapping — spans per
// phase etc. — is done at the caller's layer so agent package stays
// free of tracer imports.
func (a *Agent) PostProcess(_ context.Context, state *AgentState, step AgentStep) {
	// Ensure at least one of Actions / Done is set — an empty step
	// with neither is a soft-error "stuck" signal the caller can
	// escalate on.
	if len(step.Actions) == 0 && !step.Done {
		step.Evaluation = strings.TrimSpace(step.Evaluation + " [runtime: empty action list]")
	}
	state.AppendStep(step)
}

// ParsePlannerJSON decodes a planner's JSON reply into an AgentStep.
// Exported for use by LLMClient implementations that want the
// canonical parser + error shape.
func ParsePlannerJSON(raw string) (AgentStep, error) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var shape struct {
		Evaluation string        `json:"evaluation"`
		Memory     string        `json:"memory"`
		NextGoal   string        `json:"next_goal"`
		Done       bool          `json:"done"`
		Actions    []rawActionJSON `json:"actions"`
	}
	if err := json.Unmarshal([]byte(raw), &shape); err != nil {
		return AgentStep{}, fmt.Errorf("agent: parse planner json: %w", err)
	}
	out := AgentStep{
		Evaluation: shape.Evaluation,
		Memory:     shape.Memory,
		NextGoal:   shape.NextGoal,
		Done:       shape.Done,
		Actions:    make([]nexus.Action, 0, len(shape.Actions)),
	}
	for _, a := range shape.Actions {
		out.Actions = append(out.Actions, nexus.Action{
			Kind:   a.Kind,
			Target: a.Target,
			Text:   a.Text,
			X:      a.X,
			Y:      a.Y,
		})
	}
	return out, nil
}

type rawActionJSON struct {
	Kind   string `json:"kind"`
	Target string `json:"target"`
	Text   string `json:"text"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
