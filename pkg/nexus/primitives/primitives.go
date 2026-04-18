// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package primitives

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
	"digital.vasic.helixqa/pkg/nexus/agent"
)

// LLMClient is the narrow contract this package requires. Identical
// shape to agent.LLMClient — every existing implementation slots in.
type LLMClient = agent.LLMClient

// ===========================================================================
// Prompt cache
// ===========================================================================

// PromptCache memoises PlanRequest → AgentStep responses for a short
// TTL so repeat primitive calls in a tight loop stop re-hitting the
// LLM. Ports Stagehand's prompt caching idiom with explicit bounds.
type PromptCache struct {
	mu    sync.Mutex
	ttl   time.Duration
	store map[string]cacheEntry
}

type cacheEntry struct {
	step      agent.AgentStep
	expiresAt time.Time
}

// NewPromptCache returns a cache bound to the given TTL. Zero
// defaults to 5 minutes (matches Stagehand's default).
func NewPromptCache(ttl time.Duration) *PromptCache {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	return &PromptCache{ttl: ttl, store: map[string]cacheEntry{}}
}

// Get returns the cached step and true when a live entry matches
// the key.
func (c *PromptCache) Get(key string) (agent.AgentStep, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.store[key]
	if !ok || time.Now().After(e.expiresAt) {
		if ok {
			delete(c.store, key)
		}
		return agent.AgentStep{}, false
	}
	return e.step, true
}

// Put stores step under key for TTL.
func (c *PromptCache) Put(key string, step agent.AgentStep) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = cacheEntry{step: step, expiresAt: time.Now().Add(c.ttl)}
}

// Size returns the current entry count (observability helper).
func (c *PromptCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.store)
}

// fingerprint returns a stable cache key for prompt + schema + mode.
// Keys never persist to disk; they live in memory only.
func fingerprint(mode, prompt, schema string, extra ...string) string {
	h := sha256.New()
	h.Write([]byte(mode))
	h.Write([]byte{0})
	h.Write([]byte(prompt))
	h.Write([]byte{0})
	h.Write([]byte(schema))
	for _, e := range extra {
		h.Write([]byte{0})
		h.Write([]byte(e))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ===========================================================================
// Engine — shared primitive wiring
// ===========================================================================

// Engine ties an LLMClient, a nexus.Adapter, an optional PromptCache,
// and an optional SelfHealer together so the four primitives share
// one execution path. Callers construct one Engine per session.
type Engine struct {
	llm     LLMClient
	adapter nexus.Adapter
	session nexus.Session
	cache   *PromptCache
	healer  *agent.SelfHealer
}

// EngineOption configures optional dependencies.
type EngineOption func(*Engine)

// WithPromptCache enables cache-through for repeat primitive calls.
func WithPromptCache(c *PromptCache) EngineOption {
	return func(e *Engine) { e.cache = c }
}

// WithSelfHealer enables re-inference on selector failure for Act().
func WithSelfHealer(h *agent.SelfHealer) EngineOption {
	return func(e *Engine) { e.healer = h }
}

// NewEngine wires the mandatory trio.
func NewEngine(llm LLMClient, adapter nexus.Adapter, session nexus.Session, opts ...EngineOption) (*Engine, error) {
	if llm == nil {
		return nil, errors.New("primitives: nil LLMClient")
	}
	if adapter == nil {
		return nil, errors.New("primitives: nil Adapter")
	}
	if session == nil {
		return nil, errors.New("primitives: nil Session")
	}
	e := &Engine{llm: llm, adapter: adapter, session: session}
	for _, opt := range opts {
		opt(e)
	}
	return e, nil
}

// ===========================================================================
// Act — natural language → single nexus.Action
// ===========================================================================

// Act maps a natural-language instruction into a single Action and
// dispatches it through the Adapter. When a SelfHealer is wired and
// the first attempt fails, Act re-plans with an Observe() hint so
// stale selectors recover automatically.
func (e *Engine) Act(ctx context.Context, instruction string) error {
	snap, err := e.adapter.Snapshot(ctx, e.session)
	if err != nil {
		return fmt.Errorf("primitives.act: snapshot: %w", err)
	}
	key := fingerprint("act", instruction, "", snapshotHash(snap))
	if e.cache != nil {
		if step, ok := e.cache.Get(key); ok && len(step.Actions) > 0 {
			return e.adapter.Do(ctx, e.session, step.Actions[0])
		}
	}
	step, err := e.planAct(ctx, instruction, snap, nil)
	if err != nil {
		return err
	}
	if len(step.Actions) == 0 {
		return fmt.Errorf("primitives.act: planner returned no action")
	}
	if err := e.adapter.Do(ctx, e.session, step.Actions[0]); err != nil {
		// Stale-selector self-heal path.
		if e.healer != nil {
			healed, hErr := e.healer.Heal(ctx, agent.NewAgentState(instruction, e.session.ID()),
				"first attempt failed: "+err.Error())
			if hErr != nil {
				return fmt.Errorf("primitives.act: heal: %w", hErr)
			}
			if len(healed.Actions) > 0 {
				return e.adapter.Do(ctx, e.session, healed.Actions[0])
			}
		}
		return fmt.Errorf("primitives.act: dispatch: %w", err)
	}
	if e.cache != nil {
		e.cache.Put(key, step)
	}
	return nil
}

// ===========================================================================
// Extract — schema + prompt → typed value
// ===========================================================================

// Extract asks the LLM to pull structured data matching schema from
// the current page. schema is a JSON-schema fragment as a string
// (e.g. `{"type":"object","properties":{"title":{"type":"string"}}}`)
// and the out pointer receives the decoded value. Caller owns the
// concrete Go type out points at.
func (e *Engine) Extract(ctx context.Context, prompt, schema string, out any) error {
	if out == nil {
		return errors.New("primitives.extract: nil out pointer")
	}
	snap, err := e.adapter.Snapshot(ctx, e.session)
	if err != nil {
		return fmt.Errorf("primitives.extract: snapshot: %w", err)
	}
	key := fingerprint("extract", prompt, schema, snapshotHash(snap))
	if e.cache != nil {
		if step, ok := e.cache.Get(key); ok {
			return json.Unmarshal([]byte(step.Memory), out)
		}
	}
	req := agent.PlanRequest{
		TaskGoal:   prompt,
		Snapshot:   snap,
		SystemPrompt: "You are a structured-data extractor. Reply with a single JSON object matching the schema in Memory. Do not emit actions.",
		RecentSteps: []agent.AgentStep{{
			Evaluation: "Extraction schema:",
			Memory:     schema,
		}},
	}
	step, err := e.llm.PlanStep(ctx, req)
	if err != nil {
		return fmt.Errorf("primitives.extract: plan: %w", err)
	}
	if step.Memory == "" {
		return errors.New("primitives.extract: planner returned empty Memory")
	}
	if err := json.Unmarshal([]byte(step.Memory), out); err != nil {
		return fmt.Errorf("primitives.extract: decode: %w", err)
	}
	if e.cache != nil {
		e.cache.Put(key, step)
	}
	return nil
}

// ===========================================================================
// Observe — descriptor → nexus.ElementRef(s)
// ===========================================================================

// Observe asks the LLM to resolve a human descriptor into one or
// more nexus.ElementRefs against the current snapshot. Callers use
// the returned refs in subsequent Action dispatches — useful when a
// scripted flow needs to target a DOM element without writing a
// selector by hand.
func (e *Engine) Observe(ctx context.Context, descriptor string) ([]nexus.ElementRef, error) {
	snap, err := e.adapter.Snapshot(ctx, e.session)
	if err != nil {
		return nil, fmt.Errorf("primitives.observe: snapshot: %w", err)
	}
	key := fingerprint("observe", descriptor, "", snapshotHash(snap))
	if e.cache != nil {
		if step, ok := e.cache.Get(key); ok {
			return refsFromMemory(step.Memory), nil
		}
	}
	req := agent.PlanRequest{
		TaskGoal:     descriptor,
		Snapshot:     snap,
		SystemPrompt: "You resolve human descriptors into element refs. Reply in Memory with a comma-separated list of refs from the snapshot (e.g. 'e1,e3'). Do not emit actions.",
	}
	step, err := e.llm.PlanStep(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("primitives.observe: plan: %w", err)
	}
	refs := refsFromMemory(step.Memory)
	if len(refs) == 0 {
		return nil, fmt.Errorf("primitives.observe: planner returned no refs")
	}
	if e.cache != nil {
		e.cache.Put(key, step)
	}
	return refs, nil
}

// ===========================================================================
// Agent — scoped autonomous mode
// ===========================================================================

// Agent runs a scoped goal through the full four-phase state
// machine, sharing the same LLM + adapter + session with the other
// primitives. Returns the final AgentState so callers can inspect
// History / Iteration / Done for auditing.
func (e *Engine) Agent(ctx context.Context, goal string, cfg agent.Config) (*agent.AgentState, error) {
	a, err := agent.NewAgent(e.llm, e.adapter, cfg)
	if err != nil {
		return nil, err
	}
	state := agent.NewAgentState(goal, e.session.ID())
	if err := a.Run(ctx, state, e.session); err != nil && !errors.Is(err, agent.ErrMaxIterationsExceeded) {
		return state, err
	}
	return state, nil
}

// ===========================================================================
// Helpers
// ===========================================================================

func (e *Engine) planAct(ctx context.Context, instruction string, snap *nexus.Snapshot, recent []agent.AgentStep) (agent.AgentStep, error) {
	req := agent.PlanRequest{
		TaskGoal:     instruction,
		Snapshot:     snap,
		SystemPrompt: "You are the hybrid AI primitive 'act'. Reply with a single Action in the Actions array.",
		RecentSteps:  recent,
	}
	step, err := e.llm.PlanStep(ctx, req)
	if err != nil {
		return agent.AgentStep{}, fmt.Errorf("primitives.act: plan: %w", err)
	}
	return step, nil
}

// snapshotHash is a tiny content hash of the snapshot tree — used
// only to key the PromptCache so changes in the page invalidate
// stale entries.
func snapshotHash(s *nexus.Snapshot) string {
	if s == nil {
		return "nil"
	}
	h := sha256.New()
	h.Write([]byte(s.Tree))
	for _, el := range s.Elements {
		h.Write([]byte(string(el.Ref)))
		h.Write([]byte(el.Selector))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func refsFromMemory(memory string) []nexus.ElementRef {
	memory = strings.TrimSpace(memory)
	if memory == "" {
		return nil
	}
	// Strip any wrapping brackets a model might emit.
	memory = strings.Trim(memory, "[]")
	parts := strings.Split(memory, ",")
	out := make([]nexus.ElementRef, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.Trim(strings.TrimSpace(p), `"'`)
		if trimmed == "" {
			continue
		}
		out = append(out, nexus.ElementRef(trimmed))
	}
	return out
}
