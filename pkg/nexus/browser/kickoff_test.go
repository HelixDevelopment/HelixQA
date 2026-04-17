package browser

// This file hosts the Phase-0 scenario-style "kickoff" tests referenced in
// the Helix Nexus plan as CH-NX-KICKOFF-001 through CH-NX-KICKOFF-005. They
// exercise end-to-end flows through the Engine + Pool + Snapshot parser to
// confirm Phase 0 wiring before Phase 1 introduces real drivers.
//
// Tests that legitimately need a real Chromium binary carry t.Skip under
// the default build tags and land in the `nexus_chromedp` / `nexus_rod`
// test files in later phases.

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"digital.vasic.helixqa/pkg/nexus"
)

// CH-NX-KICKOFF-001 — Engine opens + closes without leaking sessions.
func TestNexusKickoff001_OpenCloseNoLeaks(t *testing.T) {
	e, err := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		s, err := e.Open(context.Background(), nexus.SessionOptions{})
		if err != nil {
			t.Fatal(err)
		}
		_ = s.Close()
	}
	if e.ActiveSessions() != 0 {
		t.Errorf("leak: ActiveSessions = %d after 100 open/close pairs", e.ActiveSessions())
	}
}

// CH-NX-KICKOFF-002 — Snapshot parser assigns stable e1..eN refs.
func TestNexusKickoff002_SnapshotRefsStable(t *testing.T) {
	html := `<button id=a>ok</button><input id=b /><a href=# id=c>link</a>`
	snap, err := SnapshotFromHTML(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Elements) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(snap.Elements))
	}
	refs := []nexus.ElementRef{snap.Elements[0].Ref, snap.Elements[1].Ref, snap.Elements[2].Ref}
	want := []nexus.ElementRef{"e1", "e2", "e3"}
	for i := range refs {
		if refs[i] != want[i] {
			t.Errorf("refs[%d] = %q, want %q", i, refs[i], want[i])
		}
	}
}

// CH-NX-KICKOFF-003 — Pool caps concurrent sessions at its configured size.
func TestNexusKickoff003_PoolCapEnforced(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	pool, _ := NewPool(e, 3)

	var acquired atomic.Int64
	var wg sync.WaitGroup

	release := make(chan struct{})
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := pool.Acquire(context.Background(), nexus.SessionOptions{})
			if err != nil {
				t.Errorf("acquire: %v", err)
				return
			}
			acquired.Add(1)
			<-release
			pool.Release(s)
		}()
	}

	time.Sleep(50 * time.Millisecond)
	if got := acquired.Load(); got != 3 {
		t.Errorf("expected exactly 3 concurrent acquires, got %d", got)
	}
	close(release)
	wg.Wait()
}

// CH-NX-KICKOFF-004 — Security allowlist rejects disallowed hosts.
func TestNexusKickoff004_AllowlistRejects(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{
		Engine:       EngineChromedp,
		AllowedHosts: []string{"catalogizer.local"},
	})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	defer s.Close()

	if err := e.Navigate(context.Background(), s, "https://disallowed.test/"); err == nil {
		t.Fatal("allowlist must reject unknown host")
	}
	if err := e.Navigate(context.Background(), s, "https://catalogizer.local/"); err != nil {
		t.Fatalf("allowed host rejected: %v", err)
	}
}

// CH-NX-KICKOFF-005 — ToAIFriendlyError is wired through the Engine for
// openable + navigable flows.
func TestNexusKickoff005_ErrorsAITranslated(t *testing.T) {
	e, _ := NewEngine(&mockDriver{kind: EngineChromedp}, Config{Engine: EngineChromedp})
	s, _ := e.Open(context.Background(), nexus.SessionOptions{})
	defer s.Close()

	// File-scheme navigation is blocked before it touches the driver, so
	// the returned error must be a clear denial, not an opaque driver
	// message.
	err := e.Navigate(context.Background(), s, "file:///etc/passwd")
	if err == nil {
		t.Fatal("expected denial error")
	}
	if !containsIgnoreCase(err.Error(), "blocked") && !containsIgnoreCase(err.Error(), "scheme") {
		t.Errorf("error should be human/LLM-friendly, got %q", err.Error())
	}
}
