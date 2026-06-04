// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"time"

	"digital.vasic.helixqa/pkg/conduit"
)

// conduitPhaseListener adapts the conduit.Sink to the PhaseListener
// interface so phase transitions on a SessionCoordinator's
// PhaseManager are mirrored onto the real-time conductor channel
// without coupling the PhaseManager to conduit.
//
// It is the bridge that turns the coordinator's existing observer
// seam into live conductor events (session-start side already emitted
// by the caller; this listener handles per-phase transitions).
type conduitPhaseListener struct {
	sink conduit.Sink
}

// NewConduitPhaseListener returns a PhaseListener that forwards phase
// transitions to the given conduit.Sink. Register it on a
// SessionCoordinator via coordinator.PhaseManager().AddListener(...).
func NewConduitPhaseListener(sink conduit.Sink) PhaseListener {
	if sink == nil {
		sink = conduit.NopSink()
	}
	return &conduitPhaseListener{sink: sink}
}

// OnPhaseStart emits a phase_start conductor event.
func (l *conduitPhaseListener) OnPhaseStart(phase Phase) {
	conduit.PhaseStart(l.sink, phase.Name, "")
}

// OnPhaseComplete emits a phase_complete conductor event with the
// observed phase duration.
func (l *conduitPhaseListener) OnPhaseComplete(phase Phase) {
	dur := phase.Duration()
	if dur == 0 && !phase.StartAt.IsZero() {
		dur = time.Since(phase.StartAt)
	}
	conduit.PhaseComplete(l.sink, phase.Name, dur)
}

// OnPhaseError emits a phase_error conductor event.
func (l *conduitPhaseListener) OnPhaseError(phase Phase, err error) {
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	conduit.PhaseError(l.sink, phase.Name, reason)
}

// AttachConduit wires a conduit.Sink onto a SessionCoordinator: it
// registers a phase listener for live phase events. The caller is
// responsible for emitting session_start / session_end around
// coordinator.Run, and for closing the underlying Writer. Returns the
// coordinator for chaining.
func AttachConduit(sc *SessionCoordinator, sink conduit.Sink) *SessionCoordinator {
	if sc == nil || sink == nil {
		return sc
	}
	sc.PhaseManager().AddListener(NewConduitPhaseListener(sink))
	return sc
}
