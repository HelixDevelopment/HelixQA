// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package automation

import "time"

// Result is the structured report Engine.Perform returns after
// dispatching one Action. The Agent folds this into the next
// planning turn's perception signal.
type Result struct {
	// Success reports whether the dispatched sub-engine call returned
	// without error.
	Success bool

	// VerificationPassed reports whether the optional post-action
	// verifier (if wired) confirmed the screen changed as expected.
	// When no verifier is wired this is always false and carries no
	// negative meaning.
	VerificationPassed bool

	// Elapsed is the wall-clock time from Perform entry to return,
	// including dispatch, verification, and evidence collection.
	Elapsed time.Duration

	// Evidence lists artefacts the Engine produced during this action
	// (screenshots, clips, hook-trace snapshots). Consumers resolve
	// Kind+Ref to on-disk artefacts via the evidence store.
	Evidence []EvidenceRef

	// Error holds the error string from the first failure encountered.
	// Empty when Success is true.
	Error string

	// DispatchedTo names the worker or model that handled an
	// ActionAnalyze request (e.g. "local-cpu", "thinker-cuda").
	// Empty for all other action kinds.
	DispatchedTo string
}

// EvidenceRef points at an artefact produced during Engine.Perform.
// Kind and Ref together form a stable key the evidence store uses to
// locate the actual bytes.
type EvidenceRef struct {
	// Kind is a short label: "screenshot_before", "screenshot_after",
	// "clip", or "hook_trace".
	Kind string

	// Ref is an opaque identifier — a frame sequence number, a file
	// path relative to the session output directory, or a clip size
	// annotation. Consumers must not parse or compare Ref across
	// different Kind values.
	Ref string
}
