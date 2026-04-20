// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package action defines the unified agent-action vocabulary HelixQA
// uses across every VLM backend (UI-TARS, OmniParser-guided, LangGraph
// composed, future BC models). Keeping the vocabulary in one package
// means the executor layer (navigator / uinput / scrcpy control) can
// consume any upstream VLM uniformly.
//
// See OpenClawing4.md §5.4 (VLM-driven action resolution) and the
// Phase-3 kickoff notes.
package action

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Kind enumerates the universe of GUI actions a VLM may emit.
type Kind string

const (
	KindClick   Kind = "click"
	KindType    Kind = "type"
	KindScroll  Kind = "scroll"
	KindWait    Kind = "wait"
	KindDone    Kind = "done"
	KindKey     Kind = "key"     // keypress by name ("ENTER", "BACK", "DPAD_CENTER")
	KindSwipe   Kind = "swipe"   // (x1,y1) → (x2,y2) over Duration
	KindOpenApp Kind = "open_app" // launch a package / URL
)

// Action is the unified VLM-produced action HelixQA executes. Exactly
// one field per Kind is populated — the marshaller emits a compact
// JSON tagged-union envelope consumers can round-trip.
//
// Wire format:
//
//	{"kind": "click", "x": 120, "y": 340, "reason": "log-in button"}
//	{"kind": "type", "text": "admin", "reason": "username field"}
//	{"kind": "scroll", "dx": 0, "dy": -300, "reason": "reveal list"}
//	{"kind": "wait", "duration_ms": 500, "reason": "spinner"}
//	{"kind": "done", "reason": "login confirmed"}
//	{"kind": "key", "key": "ENTER", "reason": "submit form"}
//	{"kind": "swipe", "x": 500, "y": 800, "x2": 500, "y2": 200, "duration_ms": 250}
//	{"kind": "open_app", "target": "com.example.app", "reason": "test start"}
type Action struct {
	Kind       Kind   `json:"kind"`
	X          int    `json:"x,omitempty"`
	Y          int    `json:"y,omitempty"`
	X2         int    `json:"x2,omitempty"`
	Y2         int    `json:"y2,omitempty"`
	DX         int    `json:"dx,omitempty"`
	DY         int    `json:"dy,omitempty"`
	Text       string `json:"text,omitempty"`
	Key        string `json:"key,omitempty"`
	Target     string `json:"target,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`

	// Reason is a freeform VLM-emitted explanation that survives into
	// audit logs and evidence reports. Always optional but strongly
	// encouraged — it's the difference between a readable QA trace and
	// a pile of opaque coordinates.
	Reason string `json:"reason,omitempty"`
}

// Sentinel errors.
var (
	ErrUnknownKind    = errors.New("helixqa/agent/action: unknown Kind")
	ErrMissingField   = errors.New("helixqa/agent/action: required field missing for this Kind")
	ErrInvalidNumeric = errors.New("helixqa/agent/action: numeric field out of range")
)

// Validate checks that the Action has the fields its Kind requires.
// Returns a wrapped ErrMissingField / ErrInvalidNumeric on violation.
// Actions that pass Validate are always safe to execute — the executor
// does NOT need to re-check field presence.
func (a Action) Validate() error {
	switch a.Kind {
	case KindClick:
		if a.X < 0 || a.Y < 0 {
			return fmt.Errorf("%w: click requires X, Y ≥ 0", ErrInvalidNumeric)
		}
	case KindType:
		if a.Text == "" {
			return fmt.Errorf("%w: type requires non-empty Text", ErrMissingField)
		}
	case KindScroll:
		if a.DX == 0 && a.DY == 0 {
			return fmt.Errorf("%w: scroll requires DX or DY non-zero", ErrMissingField)
		}
	case KindWait:
		if a.DurationMs <= 0 {
			return fmt.Errorf("%w: wait requires DurationMs > 0", ErrInvalidNumeric)
		}
	case KindDone:
		// No required fields.
	case KindKey:
		if a.Key == "" {
			return fmt.Errorf("%w: key requires non-empty Key", ErrMissingField)
		}
	case KindSwipe:
		if a.X < 0 || a.Y < 0 || a.X2 < 0 || a.Y2 < 0 {
			return fmt.Errorf("%w: swipe coords must be ≥ 0", ErrInvalidNumeric)
		}
		if a.DurationMs < 0 {
			return fmt.Errorf("%w: swipe DurationMs ≥ 0", ErrInvalidNumeric)
		}
	case KindOpenApp:
		if a.Target == "" {
			return fmt.Errorf("%w: open_app requires non-empty Target", ErrMissingField)
		}
	default:
		return fmt.Errorf("%w: %q", ErrUnknownKind, a.Kind)
	}
	return nil
}

// Summary returns a single-line human-readable description suitable
// for session logs and evidence reports.
func (a Action) Summary() string {
	switch a.Kind {
	case KindClick:
		return fmt.Sprintf("click (%d, %d) — %s", a.X, a.Y, a.Reason)
	case KindType:
		return fmt.Sprintf("type %q — %s", a.Text, a.Reason)
	case KindScroll:
		return fmt.Sprintf("scroll Δ(%d, %d) — %s", a.DX, a.DY, a.Reason)
	case KindWait:
		return fmt.Sprintf("wait %dms — %s", a.DurationMs, a.Reason)
	case KindDone:
		return fmt.Sprintf("done — %s", a.Reason)
	case KindKey:
		return fmt.Sprintf("key %s — %s", a.Key, a.Reason)
	case KindSwipe:
		return fmt.Sprintf("swipe (%d,%d)→(%d,%d) over %dms — %s", a.X, a.Y, a.X2, a.Y2, a.DurationMs, a.Reason)
	case KindOpenApp:
		return fmt.Sprintf("open_app %q — %s", a.Target, a.Reason)
	}
	return fmt.Sprintf("%s (unknown)", a.Kind)
}

// ParseJSON decodes a JSON envelope into an Action and validates it.
// Handy for VLM backends that stream JSON action lines (UI-TARS,
// OmniParser+LLM pipelines).
func ParseJSON(b []byte) (Action, error) {
	var a Action
	if err := json.Unmarshal(b, &a); err != nil {
		return Action{}, fmt.Errorf("helixqa/agent/action: unmarshal: %w", err)
	}
	if err := a.Validate(); err != nil {
		return Action{}, err
	}
	return a, nil
}
