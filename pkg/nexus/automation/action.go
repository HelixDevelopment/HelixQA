// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package automation is the OCU P6 unified automation surface.
// It composes P1–P5 primitives (CaptureSource, VisionPipeline,
// Interactor, Observer, Recorder) behind a single Engine.Perform()
// call. Every Action is produced by the LLM / Agent state machine;
// the Engine dispatches and verifies but never decides.
package automation

import (
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ActionKind enumerates what an automation Action asks the Engine to
// perform. New kinds ADD to this list; existing constants are never
// renamed (callers rely on the string values for serialisation).
type ActionKind string

const (
	// ActionClick sends a mouse-click at Action.At via the Interactor.
	ActionClick ActionKind = "click"

	// ActionType injects Action.Text as keyboard events via the Interactor.
	ActionType ActionKind = "type"

	// ActionScroll sends a scroll event at Action.At with Action.DX/DY via
	// the Interactor.
	ActionScroll ActionKind = "scroll"

	// ActionKey presses and releases Action.Key via the Interactor.
	ActionKey ActionKind = "key"

	// ActionDrag drags the pointer from Action.At to Action.To via the
	// Interactor.
	ActionDrag ActionKind = "drag"

	// ActionCapture pulls the latest frame from the CaptureSource and
	// records a screenshot_before EvidenceRef in the Result.
	ActionCapture ActionKind = "capture"

	// ActionAnalyze pulls the latest frame and runs VisionPipeline.Analyze.
	// The Result.DispatchedTo field is populated from Analysis.DispatchedTo.
	ActionAnalyze ActionKind = "analyze"

	// ActionRecordClip cuts a time-windowed clip from the Recorder's ring
	// buffer and records a clip EvidenceRef in the Result.
	ActionRecordClip ActionKind = "record_clip"
)

// Action is a decision produced by the LLM (via the Agent state machine).
// The Engine consumes Actions but NEVER creates them. Fields not used by
// a given ActionKind carry their zero value, which is always safe.
type Action struct {
	// Kind identifies which sub-engine method the Engine dispatches to.
	Kind ActionKind

	// At is the screen coordinate used by click, scroll, and drag (source).
	At contracts.Point

	// To is the drag destination coordinate (ActionDrag only).
	To contracts.Point

	// Text is the string to inject (ActionType only).
	Text string

	// Key is the portable key code to press (ActionKey only).
	Key contracts.KeyCode

	// Button selects which mouse button to use for click/drag.
	Button contracts.MouseButton

	// DX and DY are horizontal and vertical scroll deltas (ActionScroll
	// only). Positive values scroll down / right.
	DX, DY int

	// ClipAround is the Unix nanosecond timestamp used as the midpoint of
	// the clip window (ActionRecordClip only).
	ClipAround int64

	// ClipWindow is the clip duration in nanoseconds (ActionRecordClip).
	// Zero means the Recorder returns all buffered frames.
	ClipWindow int64

	// Expected is a free-form expectation string the LLM provides so that
	// verification logic can match it against screen state. The Engine
	// forwards this verbatim to Verifier implementations; it never acts on
	// the string itself.
	Expected string
}
