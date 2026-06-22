// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package conduit provides a real-time, conductor-facing event/status
// channel for HelixQA autonomous QA sessions.
//
// The "conductor" is an external orchestrator (a human operator, a
// CLI agent, or another automation process) that needs to stay in
// sync with everything a HelixQA session is doing — session start,
// per-phase / per-Challenge step, evidence captured, verdict, LLM and
// Vision bridge calls, and errors — WITHOUT parsing free-form stdout.
//
// Transport is intentionally simple and dependency-free so any
// conductor on any platform can consume it:
//
//   - An append-only JSONL event stream (one JSON object per line).
//     The conductor tails this file as it grows (see Monitor).
//   - A single status file overwritten atomically on every event,
//     holding the latest session snapshot (current phase, counts,
//     last event). The conductor reads it for an O(1) "where are we
//     now" view without replaying the whole stream.
//
// This package is project-agnostic per CONST-051 (§11.4.28): it
// carries no consuming-project / device / package knowledge. The consuming
// project supplies the session/feature identifiers at runtime.
package conduit

import (
	"time"
)

// EventType enumerates the structured event kinds the conductor can
// react to. The set is closed and stable so a conductor can switch on
// it without guessing (§11.4.6 no-guessing).
type EventType string

const (
	// EventSessionStart marks the beginning of a QA session.
	EventSessionStart EventType = "session_start"
	// EventSessionEnd marks the end of a QA session (terminal).
	EventSessionEnd EventType = "session_end"

	// EventPhaseStart marks a session phase entering the running
	// state (setup / doc-driven / curiosity / report, or any
	// consumer-defined phase).
	EventPhaseStart EventType = "phase_start"
	// EventPhaseComplete marks a phase finishing successfully.
	EventPhaseComplete EventType = "phase_complete"
	// EventPhaseError marks a phase failing.
	EventPhaseError EventType = "phase_error"
	// EventPhaseProgress reports incremental progress within a
	// phase (0.0–1.0).
	EventPhaseProgress EventType = "phase_progress"

	// EventChallengeStart marks a Challenge / test case beginning.
	EventChallengeStart EventType = "challenge_start"
	// EventChallengeStep marks a single step inside a Challenge.
	EventChallengeStep EventType = "challenge_step"
	// EventChallengeVerdict marks a Challenge reaching a verdict
	// (PASS / FAIL / SKIP — see Verdict).
	EventChallengeVerdict EventType = "challenge_verdict"

	// EventEvidenceCaptured marks a piece of evidence (screenshot,
	// recording, logcat, sink-probe report, WAV, JSON) being
	// captured to disk. This is the §11.4.2 / §11.4.69 captured-
	// evidence signal the conductor can audit.
	EventEvidenceCaptured EventType = "evidence_captured"

	// EventLLMCall marks an LLM bridge invocation (request/response).
	EventLLMCall EventType = "llm_call"
	// EventVisionCall marks a Vision bridge invocation
	// (screen/content understanding).
	EventVisionCall EventType = "vision_call"

	// EventError marks a non-phase error worth surfacing live.
	EventError EventType = "error"
	// EventLog is a free-form informational log line, preserved so
	// the conductor sees everything the session prints without
	// having to scrape stdout.
	EventLog EventType = "log"
)

// Verdict is the closed PASS/FAIL/SKIP vocabulary for Challenge and
// session outcomes. It deliberately mirrors the §11.4.69 status
// vocabulary so a conductor never has to interpret free text.
type Verdict string

const (
	// VerdictUnknown is the zero value — no verdict yet.
	VerdictUnknown Verdict = ""
	// VerdictPass means the feature works for the end user with
	// captured positive evidence.
	VerdictPass Verdict = "PASS"
	// VerdictFail means a genuine product defect was observed.
	VerdictFail Verdict = "FAIL"
	// VerdictSkip means the case was not applicable (topology /
	// geo / hardware absent) — carries a Reason.
	VerdictSkip Verdict = "SKIP"
	// VerdictOperatorBlocked means a required sink/hardware was
	// unreachable and the case cannot be decided autonomously.
	VerdictOperatorBlocked Verdict = "OPERATOR-BLOCKED"
)

// Event is a single structured record on the conductor channel.
// Every field is JSON-serialisable; unused fields are omitted so the
// JSONL stays compact and greppable.
type Event struct {
	// Seq is a monotonic per-session sequence number assigned by
	// the Writer. The conductor uses it to detect dropped lines and
	// to order events deterministically.
	Seq uint64 `json:"seq"`

	// Time is the wall-clock instant the event was emitted (RFC3339
	// nanosecond, UTC).
	Time time.Time `json:"time"`

	// Type is the closed-set event kind.
	Type EventType `json:"type"`

	// Session is the QA session identifier (consumer-supplied).
	Session string `json:"session"`

	// Phase is the session phase the event belongs to, when
	// applicable (setup / doc-driven / curiosity / report / ...).
	Phase string `json:"phase,omitempty"`

	// Platform is the platform under test (android / android_tv /
	// web / desktop / ...), when applicable.
	Platform string `json:"platform,omitempty"`

	// Challenge is the Challenge / test-case ID, when applicable.
	Challenge string `json:"challenge,omitempty"`

	// Step is a human-readable step description, when applicable.
	Step string `json:"step,omitempty"`

	// Verdict is the PASS/FAIL/SKIP/OPERATOR-BLOCKED outcome for
	// challenge_verdict and session_end events.
	Verdict Verdict `json:"verdict,omitempty"`

	// Progress is 0.0–1.0 for phase_progress events.
	Progress float64 `json:"progress,omitempty"`

	// EvidencePath is the on-disk path of a captured artefact for
	// evidence_captured events. The conductor can stat/open it to
	// confirm the evidence is real and non-empty (anti-bluff).
	EvidencePath string `json:"evidence_path,omitempty"`

	// EvidenceKind classifies the artefact (screenshot / recording
	// / logcat / sink_probe / wav / json / ...).
	EvidenceKind string `json:"evidence_kind,omitempty"`

	// Reason carries a SKIP / OPERATOR-BLOCKED reason or an error
	// detail. Closed-set reasons (geo_restricted, operator_attended,
	// hardware_not_present, topology_unsupported, ...) are preferred
	// per §11.4.69.
	Reason string `json:"reason,omitempty"`

	// Detail is free-form supplementary context (truncated by the
	// Writer to keep lines bounded).
	Detail string `json:"detail,omitempty"`

	// DurationMS is an elapsed-time measurement in milliseconds for
	// events that wrap an operation (llm_call, vision_call, phase
	// completions).
	DurationMS int64 `json:"duration_ms,omitempty"`

	// Fields is an open extension map for consumer-specific metadata
	// (model name, token counts, cost, codec, channel count, ...).
	// Kept last so the closed core schema stays stable.
	Fields map[string]any `json:"fields,omitempty"`
}

// Status is the latest-snapshot view written to the status file. It
// gives a conductor an O(1) answer to "where is the session now?"
// without replaying the JSONL stream.
type Status struct {
	// Session is the QA session identifier.
	Session string `json:"session"`

	// State is the coarse session lifecycle state: "starting",
	// "running", or "ended".
	State string `json:"state"`

	// CurrentPhase is the phase currently running (empty before the
	// first phase / after the last).
	CurrentPhase string `json:"current_phase,omitempty"`

	// StartedAt / UpdatedAt bound the session's observed lifetime.
	StartedAt time.Time `json:"started_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// LastSeq is the sequence number of the most recent event.
	LastSeq uint64 `json:"last_seq"`

	// LastEvent is a compact copy of the most recent event so the
	// conductor sees the latest action without opening the stream.
	LastEvent *Event `json:"last_event,omitempty"`

	// Counts aggregates verdicts and evidence seen so far so the
	// conductor can render a live scoreboard.
	Counts Counts `json:"counts"`

	// FinalVerdict is set when State == "ended".
	FinalVerdict Verdict `json:"final_verdict,omitempty"`
}

// Counts aggregates running tallies for the live scoreboard.
type Counts struct {
	ChallengesStarted int `json:"challenges_started"`
	Pass              int `json:"pass"`
	Fail              int `json:"fail"`
	Skip              int `json:"skip"`
	OperatorBlocked   int `json:"operator_blocked"`
	EvidenceCaptured  int `json:"evidence_captured"`
	Errors            int `json:"errors"`
	LLMCalls          int `json:"llm_calls"`
	VisionCalls       int `json:"vision_calls"`
}
