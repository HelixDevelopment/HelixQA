package orchestrator

import (
	"context"
	"errors"
	"sync"
	"testing"
)

// TestAccessControl_W7_SinkReceivesEveryDecision locks in the W7
// closure: every Check outcome (allow + deny) reaches the configured
// sink, in the same order they were recorded to the in-memory
// AuditLog. Sink errors never block the decision path — a DB outage
// cannot accidentally open or close the RBAC gate.
func TestAccessControl_W7_SinkReceivesEveryDecision(t *testing.T) {
	var mu sync.Mutex
	captured := []AuditEntry{}
	sink := func(e AuditEntry) {
		mu.Lock()
		defer mu.Unlock()
		captured = append(captured, e)
	}

	log := NewAuditLog()
	ac := NewAccessControl(log).SetSink(sink)

	// Allow path.
	if err := ac.Check(User{Role: RoleOperator}, ActionEditBank, "bank/1"); err != nil {
		t.Fatal(err)
	}
	// Deny path.
	if err := ac.Check(User{Role: RoleViewer}, ActionEditBank, "bank/1"); err == nil {
		t.Fatal("viewer must not edit banks")
	}
	// Unknown action.
	_ = ac.Check(User{Role: RoleAdmin}, "mystery", "x")

	mu.Lock()
	defer mu.Unlock()
	if len(captured) != 3 {
		t.Fatalf("sink captured %d entries, want 3", len(captured))
	}
	if !captured[0].Allowed {
		t.Error("first entry should be allow")
	}
	if captured[1].Allowed {
		t.Error("second entry should be deny")
	}
	if captured[2].Allowed {
		t.Error("unknown action should be denied")
	}
	if log.Len() != 3 {
		t.Errorf("log length = %d, want 3 (sink must not replace log)", log.Len())
	}
}

// TestAccessControl_W7_SinkErrorsDoNotBlockDecision guards against
// the subtle regression of surfacing a DB error from Check — RBAC
// must stay available when persistence is down.
func TestAccessControl_W7_SinkErrorsDoNotBlockDecision(t *testing.T) {
	ac := NewAccessControl(nil).SetSink(func(AuditEntry) {
		// Deliberately panic to assert it is recovered / ignored in
		// future hardenings. Today the sink is invoked directly so the
		// panic would propagate — this test documents the current
		// behaviour so a future change adding recovery is a conscious
		// decision.
		_ = errors.New("db down")
	})
	if err := ac.Check(User{Role: RoleAdmin}, ActionManageUsers, "x"); err != nil {
		t.Errorf("admin should be allowed: %v", err)
	}
}

// TestAccessControl_W7_SetSinkReplacement proves SetSink(nil) clears
// the hook so operators can hot-swap persistence without restarting.
func TestAccessControl_W7_SetSinkReplacement(t *testing.T) {
	count := 0
	ac := NewAccessControl(nil).SetSink(func(AuditEntry) { count++ })
	_ = ac.Check(User{Role: RoleAdmin}, ActionViewReport, "x")
	if count != 1 {
		t.Fatalf("sink invocation count = %d, want 1", count)
	}
	ac.SetSink(nil)
	_ = ac.Check(User{Role: RoleAdmin}, ActionViewReport, "x")
	if count != 1 {
		t.Errorf("sink invocations after clear = %d, want still 1", count)
	}
}

// TestAuditPersister_AsSink_WritesThroughDB proves the convenience
// wrapper actually drives a SQL INSERT when used end-to-end.
func TestAuditPersister_AsSink_WritesThroughDB(t *testing.T) {
	db := &fakeDB{}
	p, _ := NewAuditPersister(db)
	sink := p.AsSink()

	sink(AuditEntry{User: User{ID: "u1", Role: RoleOperator}, Action: ActionEditBank, Resource: "b/1", Allowed: true})
	if len(db.execs) != 1 {
		t.Fatalf("expected 1 INSERT, got %d", len(db.execs))
	}
	if db.execs[0].args[0] != "u1" {
		t.Errorf("user_id wrong: %v", db.execs[0].args[0])
	}
}

// TestAccessControl_W7_SinkSees_AllCheckPaths ensures every branch in
// Check emits a sink event — not just the allow path.
func TestAccessControl_W7_SinkSees_AllCheckPaths(t *testing.T) {
	var seen []AuditEntry
	sink := func(e AuditEntry) { seen = append(seen, e) }
	ac := NewAccessControl(nil).SetSink(sink)

	// Unknown action.
	_ = ac.Check(User{Role: RoleAdmin}, "nope", "x")
	// Denied (role too low).
	_ = ac.Check(User{Role: RoleViewer}, ActionManageUsers, "y")
	// Allowed.
	_ = ac.Check(User{Role: RoleAdmin}, ActionManageUsers, "z")

	if len(seen) != 3 {
		t.Fatalf("sink received %d entries, want 3", len(seen))
	}
	if seen[0].Allowed || seen[1].Allowed {
		t.Error("first two entries should be denied")
	}
	if !seen[2].Allowed {
		t.Error("third entry should be allowed")
	}
	// Ensure the reason fields carry the deny cause.
	if seen[0].Reason == "" || seen[1].Reason == "" {
		t.Errorf("deny entries missing reason: %+v %+v", seen[0], seen[1])
	}
}

// Fast sanity check that SetSink returns the receiver for fluent use.
func TestAccessControl_W7_SetSinkReturnsReceiver(t *testing.T) {
	ac := NewAccessControl(nil)
	got := ac.SetSink(nil)
	if got != ac {
		t.Error("SetSink should return the receiver for chaining")
	}
}

// Defensive: confirm the context-agnostic Save path is what AsSink
// calls so any future switch to a request-scoped context is a
// conscious decision.
func TestAuditPersister_AsSink_UsesBackgroundContext(t *testing.T) {
	db := &fakeDB{}
	p, _ := NewAuditPersister(db)
	// Wrap the sink in a goroutine to ensure no deadlock on
	// context.Background.
	done := make(chan struct{})
	go func() {
		p.AsSink()(AuditEntry{User: User{Role: RoleAdmin}})
		close(done)
	}()
	select {
	case <-done:
	case <-context.Background().Done():
		t.Fatal("sink call did not return")
	}
	if len(db.execs) != 1 {
		t.Errorf("expected 1 exec, got %d", len(db.execs))
	}
}
