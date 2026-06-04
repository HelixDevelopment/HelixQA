// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package conduit

import "time"

// Typed emit helpers keep call sites in the QA session terse and
// consistent. They all funnel through Sink.Emit so sequencing,
// timestamping, and status folding happen in exactly one place.

// SessionStart emits a session_start event.
func SessionStart(s Sink, session string, fields map[string]any) {
	s.Emit(Event{Type: EventSessionStart, Session: session, Fields: fields})
}

// SessionEnd emits a terminal session_end event with the final
// verdict.
func SessionEnd(s Sink, session string, v Verdict, detail string) {
	s.Emit(Event{Type: EventSessionEnd, Session: session, Verdict: v, Detail: detail})
}

// PhaseStart emits a phase_start event.
func PhaseStart(s Sink, phase, platform string) {
	s.Emit(Event{Type: EventPhaseStart, Phase: phase, Platform: platform})
}

// PhaseComplete emits a phase_complete event with elapsed duration.
func PhaseComplete(s Sink, phase string, dur time.Duration) {
	s.Emit(Event{Type: EventPhaseComplete, Phase: phase, DurationMS: dur.Milliseconds()})
}

// PhaseError emits a phase_error event.
func PhaseError(s Sink, phase, reason string) {
	s.Emit(Event{Type: EventPhaseError, Phase: phase, Reason: reason})
}

// PhaseProgress emits a phase_progress event (progress clamped 0..1).
func PhaseProgress(s Sink, phase string, progress float64) {
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	s.Emit(Event{Type: EventPhaseProgress, Phase: phase, Progress: progress})
}

// ChallengeStart emits a challenge_start event.
func ChallengeStart(s Sink, challenge, platform string) {
	s.Emit(Event{Type: EventChallengeStart, Challenge: challenge, Platform: platform})
}

// ChallengeStep emits a challenge_step event.
func ChallengeStep(s Sink, challenge, step string) {
	s.Emit(Event{Type: EventChallengeStep, Challenge: challenge, Step: step})
}

// ChallengeVerdict emits a challenge_verdict event. For SKIP and
// OPERATOR-BLOCKED, pass a closed-set reason.
func ChallengeVerdict(s Sink, challenge string, v Verdict, reason string) {
	s.Emit(Event{Type: EventChallengeVerdict, Challenge: challenge, Verdict: v, Reason: reason})
}

// EvidenceCaptured emits an evidence_captured event. The conductor
// audits EvidencePath for existence and non-emptiness (anti-bluff).
func EvidenceCaptured(s Sink, challenge, kind, path string) {
	s.Emit(Event{
		Type:         EventEvidenceCaptured,
		Challenge:    challenge,
		EvidenceKind: kind,
		EvidencePath: path,
	})
}

// LLMCall emits an llm_call event recording a bridge invocation.
func LLMCall(s Sink, phase string, dur time.Duration, fields map[string]any) {
	s.Emit(Event{Type: EventLLMCall, Phase: phase, DurationMS: dur.Milliseconds(), Fields: fields})
}

// VisionCall emits a vision_call event recording a bridge invocation.
func VisionCall(s Sink, phase string, dur time.Duration, fields map[string]any) {
	s.Emit(Event{Type: EventVisionCall, Phase: phase, DurationMS: dur.Milliseconds(), Fields: fields})
}

// Errorf emits an error event.
func Errorf(s Sink, phase, reason string) {
	s.Emit(Event{Type: EventError, Phase: phase, Reason: reason})
}

// Logf emits a free-form log event so the conductor sees everything
// the session would otherwise only print to stdout.
func Logf(s Sink, phase, msg string) {
	s.Emit(Event{Type: EventLog, Phase: phase, Detail: msg})
}
