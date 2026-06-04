// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"errors"
	"testing"

	"digital.vasic.helixqa/pkg/conduit"
)

// captureSink records emitted events for assertions.
type captureSink struct{ events []conduit.Event }

func (c *captureSink) Emit(ev conduit.Event) { c.events = append(c.events, ev) }
func (c *captureSink) Err() error            { return nil }
func (c *captureSink) Close() error          { return nil }

// TestConduitPhaseListener_MirrorsTransitions proves that driving the
// real PhaseManager emits the corresponding conductor events through
// the adapter — the coordinator's existing observer seam is genuinely
// wired to the live channel (not a metadata-only claim).
func TestConduitPhaseListener_MirrorsTransitions(t *testing.T) {
	sink := &captureSink{}
	pm := NewPhaseManager()
	pm.AddListener(NewConduitPhaseListener(sink))

	if err := pm.Start("setup"); err != nil {
		t.Fatalf("Start setup: %v", err)
	}
	if err := pm.Complete("setup"); err != nil {
		t.Fatalf("Complete setup: %v", err)
	}
	if err := pm.Start("doc-driven"); err != nil {
		t.Fatalf("Start doc-driven: %v", err)
	}
	if err := pm.Fail("doc-driven", errors.New("boom")); err != nil {
		t.Fatalf("Fail doc-driven: %v", err)
	}

	if len(sink.events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(sink.events))
	}
	want := []struct {
		typ   conduit.EventType
		phase string
	}{
		{conduit.EventPhaseStart, "setup"},
		{conduit.EventPhaseComplete, "setup"},
		{conduit.EventPhaseStart, "doc-driven"},
		{conduit.EventPhaseError, "doc-driven"},
	}
	for i, w := range want {
		if sink.events[i].Type != w.typ || sink.events[i].Phase != w.phase {
			t.Errorf("event %d = %s/%s, want %s/%s",
				i, sink.events[i].Type, sink.events[i].Phase, w.typ, w.phase)
		}
	}
	if sink.events[3].Reason != "boom" {
		t.Errorf("error event reason = %q, want boom", sink.events[3].Reason)
	}
}

// TestNewConduitPhaseListener_NilSafe proves a nil sink is tolerated.
func TestNewConduitPhaseListener_NilSafe(t *testing.T) {
	l := NewConduitPhaseListener(nil)
	// Must not panic.
	l.OnPhaseStart(Phase{Name: "x"})
	l.OnPhaseComplete(Phase{Name: "x"})
	l.OnPhaseError(Phase{Name: "x"}, errors.New("e"))
}
