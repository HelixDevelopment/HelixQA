// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package conduit

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestWriter_EmitsJSONLAndStatus proves the producer side actually
// writes the events to the stream file AND keeps the status snapshot
// in sync — not a metadata-only check (§11.4.27 / §11.4.69).
func TestWriter_EmitsJSONLAndStatus(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(Config{Session: "s1", Dir: dir})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	SessionStart(w, "s1", map[string]any{"platforms": []string{"android"}})
	PhaseStart(w, "execute", "android")
	ChallengeStart(w, "TC-001", "android")
	EvidenceCaptured(w, "TC-001", "screenshot", "/tmp/shot.png")
	ChallengeVerdict(w, "TC-001", VerdictPass, "")
	PhaseComplete(w, "execute", 1500*time.Millisecond)
	SessionEnd(w, "s1", VerdictPass, "done")

	if err := w.Err(); err != nil {
		t.Fatalf("writer error: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// Verify the JSONL stream has exactly 7 valid, sequenced lines.
	f, err := os.Open(w.StreamPath())
	if err != nil {
		t.Fatalf("open stream: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var got []Event
	for sc.Scan() {
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("bad JSONL line %q: %v", sc.Text(), err)
		}
		got = append(got, ev)
	}
	if len(got) != 7 {
		t.Fatalf("expected 7 events, got %d", len(got))
	}
	for i, ev := range got {
		if ev.Seq != uint64(i+1) {
			t.Errorf("event %d has Seq=%d, want %d", i, ev.Seq, i+1)
		}
		if ev.Time.IsZero() {
			t.Errorf("event %d has zero Time", i)
		}
		if ev.Session != "s1" {
			t.Errorf("event %d Session=%q want s1", i, ev.Session)
		}
	}
	if got[0].Type != EventSessionStart {
		t.Errorf("first event is %q, want session_start", got[0].Type)
	}
	if got[6].Type != EventSessionEnd || got[6].Verdict != VerdictPass {
		t.Errorf("last event %q verdict %q, want session_end/PASS", got[6].Type, got[6].Verdict)
	}

	// Verify the status snapshot reflects the final tallies.
	st, err := ReadStatus(w.StatusPath())
	if err != nil {
		t.Fatalf("ReadStatus: %v", err)
	}
	if st.State != "ended" {
		t.Errorf("state=%q want ended", st.State)
	}
	if st.FinalVerdict != VerdictPass {
		t.Errorf("final verdict=%q want PASS", st.FinalVerdict)
	}
	if st.Counts.ChallengesStarted != 1 || st.Counts.Pass != 1 || st.Counts.EvidenceCaptured != 1 {
		t.Errorf("unexpected counts: %+v", st.Counts)
	}
	if st.LastSeq != 7 {
		t.Errorf("LastSeq=%d want 7", st.LastSeq)
	}
}

// TestMonitor_TailsLiveStream proves the conductor side receives
// events in real time as the producer appends them — the core
// "conductor stays in sync" guarantee. The producer and the monitor
// run concurrently against the same file.
func TestMonitor_TailsLiveStream(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(Config{Session: "live", Dir: dir})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	defer w.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	received := make(chan Event, 32)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		mon := NewMonitor(w.StreamPath(), WithPollInterval(20*time.Millisecond), StopOnSessionEnd(), FromStart())
		_ = mon.Tail(ctx, func(ev Event) error {
			received <- ev
			return nil
		})
		close(received)
	}()

	// Emit events with small gaps so the monitor must follow growth.
	SessionStart(w, "live", nil)
	time.Sleep(30 * time.Millisecond)
	ChallengeStart(w, "TC-X", "web")
	time.Sleep(30 * time.Millisecond)
	ChallengeVerdict(w, "TC-X", VerdictFail, "")
	time.Sleep(30 * time.Millisecond)
	SessionEnd(w, "live", VerdictFail, "")

	var order []EventType
	for ev := range received {
		order = append(order, ev.Type)
	}
	wg.Wait()

	want := []EventType{EventSessionStart, EventChallengeStart, EventChallengeVerdict, EventSessionEnd}
	if len(order) != len(want) {
		t.Fatalf("received %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("event %d = %q, want %q (full: %v)", i, order[i], want[i], order)
		}
	}
}

// TestMonitor_WaitsForFile proves a conductor can attach BEFORE the
// session creates the stream file (the realistic ordering) and still
// receive every event.
func TestMonitor_WaitsForFile(t *testing.T) {
	dir := t.TempDir()
	streamPath := filepath.Join(dir, "conduit.events.jsonl")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	done := make(chan []Event, 1)
	go func() {
		evs, _ := Collect(ctx, streamPath, WithPollInterval(20*time.Millisecond))
		done <- evs
	}()

	// Start the producer AFTER the monitor is already waiting.
	time.Sleep(100 * time.Millisecond)
	w, err := NewWriter(Config{Session: "late", Dir: dir})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	SessionStart(w, "late", nil)
	EvidenceCaptured(w, "TC-1", "wav", "/tmp/a.wav")
	SessionEnd(w, "late", VerdictPass, "")
	_ = w.Close()

	select {
	case evs := <-done:
		if len(evs) != 3 {
			t.Fatalf("collected %d events, want 3", len(evs))
		}
		if evs[1].Type != EventEvidenceCaptured || evs[1].EvidencePath != "/tmp/a.wav" {
			t.Fatalf("evidence event wrong: %+v", evs[1])
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for collected events")
	}
}

// TestWriter_ConcurrentEmit proves the channel is safe under
// concurrent emitters (multiple platform workers) — no interleaved /
// corrupt JSONL lines, every line parses, sequence numbers are unique
// and contiguous.
func TestWriter_ConcurrentEmit(t *testing.T) {
	dir := t.TempDir()
	w, err := NewWriter(Config{Session: "conc", Dir: dir})
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	const workers = 8
	const perWorker = 50
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				ChallengeStep(w, "TC", "step")
			}
		}(i)
	}
	wg.Wait()
	if err := w.Err(); err != nil {
		t.Fatalf("writer error under concurrency: %v", err)
	}
	_ = w.Close()

	f, _ := os.Open(w.StreamPath())
	defer f.Close()
	sc := bufio.NewScanner(f)
	seen := map[uint64]bool{}
	n := 0
	for sc.Scan() {
		var ev Event
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			t.Fatalf("corrupt concurrent line %q: %v", sc.Text(), err)
		}
		if seen[ev.Seq] {
			t.Fatalf("duplicate seq %d", ev.Seq)
		}
		seen[ev.Seq] = true
		n++
	}
	if n != workers*perWorker {
		t.Fatalf("got %d events, want %d", n, workers*perWorker)
	}
	// Sequence numbers must be exactly 1..N.
	for i := uint64(1); i <= uint64(n); i++ {
		if !seen[i] {
			t.Fatalf("missing seq %d", i)
		}
	}
}

// TestNopSink proves the no-op sink is safe and silent.
func TestNopSink(t *testing.T) {
	s := NopSink()
	s.Emit(Event{Type: EventLog, Detail: "x"})
	if err := s.Err(); err != nil {
		t.Fatalf("nop sink Err: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("nop sink Close: %v", err)
	}
}

// TestMonitor_MalformedLineSurfaced proves a corrupt stream line is
// surfaced as an EventError, never silently dropped (no-bluff).
func TestMonitor_MalformedLineSurfaced(t *testing.T) {
	dir := t.TempDir()
	streamPath := filepath.Join(dir, "s.jsonl")
	content := `{"seq":1,"type":"session_start","session":"x"}` + "\n" +
		`this-is-not-json` + "\n" +
		`{"seq":2,"type":"session_end","session":"x","verdict":"PASS"}` + "\n"
	if err := os.WriteFile(streamPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	evs, err := Collect(ctx, streamPath, WithPollInterval(10*time.Millisecond))
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if len(evs) != 3 {
		t.Fatalf("expected 3 events (incl. error), got %d: %+v", len(evs), evs)
	}
	if evs[1].Type != EventError || evs[1].Reason != "malformed_stream_line" {
		t.Fatalf("middle event should be malformed-error, got %+v", evs[1])
	}
}

// TestTruncate proves over-long fields are bounded.
func TestTruncate(t *testing.T) {
	long := make([]byte, maxDetailLen+500)
	for i := range long {
		long[i] = 'a'
	}
	dir := t.TempDir()
	w, _ := NewWriter(Config{Session: "t", Dir: dir})
	Logf(w, "p", string(long))
	_ = w.Close()
	data, _ := os.ReadFile(w.StreamPath())
	var ev Event
	_ = json.Unmarshal(data[:len(data)-1], &ev)
	if len([]rune(ev.Detail)) > maxDetailLen {
		t.Fatalf("detail not truncated: %d runes", len([]rune(ev.Detail)))
	}
}
