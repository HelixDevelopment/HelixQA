// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package sglang provides client-side structured-generation guardrails
// for HelixQA Phase-3 agent output. Named after SGLang (Zheng et al.,
// 2024) which enforces grammar-constrained generation at the model
// server's token-sampling layer; this package is the complementary
// client-side safety net: it validates any Actor's emitted action
// against a per-call Schema and retries the Actor with explicit
// error feedback when validation fails.
//
// The client-side Guard can't force the model's token distribution,
// but it can:
//
//   - Catch structurally valid but semantically wrong actions (e.g.,
//     a Click at (5000, 5000) on a 1920×1080 screen).
//   - Restrict the Kind vocabulary per loop stage (e.g., during the
//     login phase, reject Type/Scroll/Swipe — only Click + Key + Done
//     are safe).
//   - Bound text length to prevent prompt-injection runaway from a
//     malicious or confused VLM.
//   - Re-ask the Actor with error context on failure — since VLMs
//     often correct themselves when told what went wrong.
//
// The Guard sits between UI-TARS and Grounder in the Phase-3 stack:
//
//	*uitars.Client
//	    → *sglang.Guard(uitars.Client, Schema)   [M40]
//	    → *ground.Grounder                        [M38]
//	    → *graph.Runner                           [M39]
package sglang

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strings"
	"unicode/utf8"

	"digital.vasic.helixqa/pkg/agent/action"
)

// Actor is the minimal VLM-client contract sglang wraps. Structurally
// identical to ground.Actor; defined locally so sglang doesn't import
// ground (the packages compose but should not take a hard dependency).
type Actor interface {
	Act(ctx context.Context, screenshot image.Image, instruction string) (action.Action, error)
}

// ActorFunc adapts a plain function into an Actor. Handy for tests
// and for stacking with other adapters.
type ActorFunc func(ctx context.Context, screenshot image.Image, instruction string) (action.Action, error)

// Act satisfies Actor for ActorFunc.
func (f ActorFunc) Act(ctx context.Context, screenshot image.Image, instruction string) (action.Action, error) {
	return f(ctx, screenshot, instruction)
}

// Schema captures the structured-generation constraints that a single
// agent step must satisfy. All zero-valued fields mean "no extra
// constraint" — the only constraint in that case is the baseline
// action.Action.Validate() on Kind-specific required fields.
type Schema struct {
	// AllowedKinds is the whitelist of Kinds. Empty = any.
	AllowedKinds []action.Kind

	// RequireReason — when true, action.Reason must be non-empty.
	// Most QA contexts want this; it turns opaque actions into
	// auditable ones.
	RequireReason bool

	// MaxTextLen caps action.Text length (runes, not bytes). 0 =
	// uncapped. Prevents the VLM from dumping a full prompt as type
	// input — a common failure mode that looks like a feedback loop.
	MaxTextLen int

	// ScreenBounds — when non-empty (Dx > 0 && Dy > 0), click/swipe
	// coordinates must fall inside this rectangle. Prevents the VLM
	// from proposing (5000, 5000) on a 1920×1080 screen.
	ScreenBounds image.Rectangle
}

// Check validates a against the Schema. Returns nil on success.
// Errors always wrap one of the exported sentinels.
func (s Schema) Check(a action.Action) error {
	// Baseline Kind-specific validation first — no point checking
	// ScreenBounds on a Kind that has no coords.
	if err := a.Validate(); err != nil {
		return fmt.Errorf("%w: %v", ErrBaseValidate, err)
	}

	if len(s.AllowedKinds) > 0 {
		allowed := false
		for _, k := range s.AllowedKinds {
			if a.Kind == k {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w: got %q, allowed %v", ErrDisallowedKind, a.Kind, s.AllowedKinds)
		}
	}

	if s.RequireReason && strings.TrimSpace(a.Reason) == "" {
		return fmt.Errorf("%w: Reason is empty", ErrMissingReason)
	}

	if s.MaxTextLen > 0 && utf8.RuneCountInString(a.Text) > s.MaxTextLen {
		return fmt.Errorf("%w: Text has %d runes, max %d", ErrTextTooLong, utf8.RuneCountInString(a.Text), s.MaxTextLen)
	}

	if s.ScreenBounds.Dx() > 0 && s.ScreenBounds.Dy() > 0 {
		switch a.Kind {
		case action.KindClick:
			if !pointIn(s.ScreenBounds, image.Point{X: a.X, Y: a.Y}) {
				return fmt.Errorf("%w: click (%d, %d) outside %v", ErrOutOfBounds, a.X, a.Y, s.ScreenBounds)
			}
		case action.KindSwipe:
			if !pointIn(s.ScreenBounds, image.Point{X: a.X, Y: a.Y}) {
				return fmt.Errorf("%w: swipe start (%d, %d) outside %v", ErrOutOfBounds, a.X, a.Y, s.ScreenBounds)
			}
			if !pointIn(s.ScreenBounds, image.Point{X: a.X2, Y: a.Y2}) {
				return fmt.Errorf("%w: swipe end (%d, %d) outside %v", ErrOutOfBounds, a.X2, a.Y2, s.ScreenBounds)
			}
		}
	}

	return nil
}

// pointIn honors Go's Min-inclusive / Max-exclusive rect semantics.
func pointIn(r image.Rectangle, p image.Point) bool {
	return p.X >= r.Min.X && p.X < r.Max.X && p.Y >= r.Min.Y && p.Y < r.Max.Y
}

// Sentinel errors. All schema violations wrap one of these so callers
// can errors.Is-branch on the specific failure mode.
var (
	ErrBaseValidate    = errors.New("helixqa/agent/sglang: action failed action.Validate()")
	ErrDisallowedKind  = errors.New("helixqa/agent/sglang: Kind not in AllowedKinds")
	ErrMissingReason   = errors.New("helixqa/agent/sglang: RequireReason=true but Reason is empty")
	ErrTextTooLong     = errors.New("helixqa/agent/sglang: Text exceeds MaxTextLen")
	ErrOutOfBounds     = errors.New("helixqa/agent/sglang: click/swipe coordinate outside ScreenBounds")
	ErrRetriesExhausted = errors.New("helixqa/agent/sglang: schema validation failed after MaxRetries")
	ErrNoActor         = errors.New("helixqa/agent/sglang: Guard.Actor is nil")
)

// Guard wraps an Actor with schema validation + retry-with-feedback.
// When the wrapped Actor emits an action that fails Schema.Check,
// Guard re-invokes the Actor with the original instruction augmented
// with the error description — giving the VLM a chance to self-correct.
//
// VLMs at Temperature > 0 are non-deterministic, so retries often
// produce different outputs. Temperature = 0 clients (like the
// HelixQA default UI-TARS configuration) will likely emit the same
// output on every retry; callers that need real retry behaviour
// should either raise Temperature or set MaxRetries = 0 to fail fast.
type Guard struct {
	Actor Actor

	// Schema is applied to every action emitted by Actor. Mandatory
	// (though an empty Schema is valid — it reduces to action.Validate
	// enforcement only).
	Schema Schema

	// MaxRetries caps the number of retry attempts. 0 = no retries;
	// the Actor is called exactly once. Default when unset: 2.
	MaxRetries int

	// RetryHeader is the preface the Guard injects into the retry
	// instruction. Default: "Your previous attempt was invalid: %s.
	// Emit a JSON action matching the schema exactly.". The %s is
	// replaced with the validation error.
	RetryHeader string

	// OnRetry is called before each retry attempt. Useful for session
	// logging and metrics. Optional.
	OnRetry func(attempt int, err error)
}

// Act validates and (if needed) retries the Actor call. The returned
// action always passes Guard.Schema.Check, or the call returns an
// ErrRetriesExhausted-wrapped error.
func (g *Guard) Act(ctx context.Context, screenshot image.Image, instruction string) (action.Action, error) {
	if g.Actor == nil {
		return action.Action{}, ErrNoActor
	}
	maxRetries := g.MaxRetries
	if maxRetries == 0 {
		maxRetries = 2
	}
	header := g.RetryHeader
	if header == "" {
		header = "Your previous attempt was invalid: %s. Emit a JSON action matching the schema exactly."
	}

	instr := instruction
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return action.Action{}, err
		}
		a, err := g.Actor.Act(ctx, screenshot, instr)
		if err != nil {
			return action.Action{}, fmt.Errorf("sglang: Actor.Act (attempt %d): %w", attempt+1, err)
		}
		if err := g.Schema.Check(a); err == nil {
			return a, nil
		} else {
			lastErr = err
			if g.OnRetry != nil {
				g.OnRetry(attempt+1, err)
			}
			instr = instruction + "\n\n" + fmt.Sprintf(header, err.Error())
		}
	}
	return action.Action{}, fmt.Errorf("%w (attempts=%d): %v", ErrRetriesExhausted, maxRetries+1, lastErr)
}

// Compile-time guard: *Guard satisfies Actor.
var _ Actor = (*Guard)(nil)
