// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package conduit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// maxDetailLen bounds the Detail / Reason / Step fields so a single
// JSONL line stays parseable and the stream does not blow up if a
// caller passes a huge blob.
const maxDetailLen = 2048

// Sink is the producer-side contract the QA session calls to emit
// conductor events. It is intentionally small so the pipeline and
// coordinator can depend on the interface, not the concrete Writer
// (keeps autonomous code decoupled and unit-testable).
type Sink interface {
	// Emit records one event. It assigns the sequence number and
	// timestamp; callers leave Event.Seq / Event.Time zero. Emit is
	// safe for concurrent use and never panics — emission failures
	// are surfaced via Err(), never by crashing the session.
	Emit(ev Event)

	// Err returns the first non-nil write error observed, if any, so
	// a caller can fail a session that could not record evidence.
	Err() error

	// Close flushes and releases the underlying files.
	Close() error
}

// nopSink is a Sink that discards events. NewSessionPipeline /
// SessionCoordinator use it when no conductor channel is configured,
// so call sites never need nil checks.
type nopSink struct{}

func (nopSink) Emit(Event)   {}
func (nopSink) Err() error   { return nil }
func (nopSink) Close() error { return nil }

// NopSink returns a no-op Sink.
func NopSink() Sink { return nopSink{} }

// Writer is the concrete file-backed Sink. It appends every event to
// an append-only JSONL stream and overwrites a status file with the
// latest snapshot after each event.
//
// Both files are written through an atomic-rename for the status file
// (so a conductor never reads a half-written snapshot) and a single
// O_APPEND write for the stream (atomic for lines below PIPE_BUF on
// POSIX; we also guard with a mutex so concurrent emitters interleave
// at line boundaries).
type Writer struct {
	session string

	streamPath string
	statusPath string

	mu       sync.Mutex
	stream   *os.File
	seq      uint64
	started  time.Time
	state    string
	curPhase string
	counts   Counts
	finalV   Verdict
	lastEv   *Event

	firstErr atomic.Value // holds error
}

// Config configures a Writer.
type Config struct {
	// Session is the QA session identifier embedded in every event.
	Session string

	// Dir is the directory the channel files are written to. It is
	// created if absent. Defaults to the current directory.
	Dir string

	// StreamName overrides the JSONL stream filename. Defaults to
	// "conduit.events.jsonl".
	StreamName string

	// StatusName overrides the status filename. Defaults to
	// "conduit.status.json".
	StatusName string
}

// NewWriter creates a file-backed conductor channel. The stream file
// is opened in append mode (existing content from a prior session in
// the same file is preserved; callers wanting a clean run use a fresh
// Dir per session, which is the recommended pattern).
func NewWriter(cfg Config) (*Writer, error) {
	if cfg.Session == "" {
		return nil, fmt.Errorf("conduit: Session is required")
	}
	dir := cfg.Dir
	if dir == "" {
		dir = "."
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("conduit: create dir %q: %w", dir, err)
	}
	streamName := cfg.StreamName
	if streamName == "" {
		streamName = "conduit.events.jsonl"
	}
	statusName := cfg.StatusName
	if statusName == "" {
		statusName = "conduit.status.json"
	}
	streamPath := filepath.Join(dir, streamName)
	f, err := os.OpenFile(streamPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("conduit: open stream %q: %w", streamPath, err)
	}
	w := &Writer{
		session:    cfg.Session,
		streamPath: streamPath,
		statusPath: filepath.Join(dir, statusName),
		stream:     f,
		started:    time.Now().UTC(),
		state:      "starting",
	}
	return w, nil
}

// StreamPath returns the absolute-or-relative path of the JSONL
// stream file, so a conductor can be told where to tail.
func (w *Writer) StreamPath() string { return w.streamPath }

// StatusPath returns the path of the status snapshot file.
func (w *Writer) StatusPath() string { return w.statusPath }

// Emit implements Sink.
func (w *Writer) Emit(ev Event) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.seq++
	ev.Seq = w.seq
	if ev.Time.IsZero() {
		ev.Time = time.Now().UTC()
	} else {
		ev.Time = ev.Time.UTC()
	}
	if ev.Session == "" {
		ev.Session = w.session
	}
	ev.Step = truncate(ev.Step, maxDetailLen)
	ev.Reason = truncate(ev.Reason, maxDetailLen)
	ev.Detail = truncate(ev.Detail, maxDetailLen)

	w.applyState(ev)

	// 1) Append to the JSONL stream.
	line, err := json.Marshal(ev)
	if err != nil {
		w.setErr(fmt.Errorf("conduit: marshal event seq=%d: %w", ev.Seq, err))
		return
	}
	line = append(line, '\n')
	if _, err := w.stream.Write(line); err != nil {
		w.setErr(fmt.Errorf("conduit: append stream: %w", err))
		return
	}

	// 2) Overwrite the status snapshot atomically.
	if err := w.writeStatusLocked(&ev); err != nil {
		w.setErr(err)
	}
}

// applyState folds the event into the running status snapshot.
func (w *Writer) applyState(ev Event) {
	switch ev.Type {
	case EventSessionStart:
		w.state = "running"
	case EventSessionEnd:
		w.state = "ended"
		w.finalV = ev.Verdict
	case EventPhaseStart:
		w.curPhase = ev.Phase
	case EventPhaseError:
		w.counts.Errors++
	case EventChallengeStart:
		w.counts.ChallengesStarted++
	case EventChallengeVerdict:
		switch ev.Verdict {
		case VerdictPass:
			w.counts.Pass++
		case VerdictFail:
			w.counts.Fail++
		case VerdictSkip:
			w.counts.Skip++
		case VerdictOperatorBlocked:
			w.counts.OperatorBlocked++
		}
	case EventEvidenceCaptured:
		w.counts.EvidenceCaptured++
	case EventError:
		w.counts.Errors++
	case EventLLMCall:
		w.counts.LLMCalls++
	case EventVisionCall:
		w.counts.VisionCalls++
	}
	cp := ev
	w.lastEv = &cp
}

// writeStatusLocked writes the status snapshot via temp-file +
// atomic rename so a concurrent reader never sees a partial file.
func (w *Writer) writeStatusLocked(last *Event) error {
	st := Status{
		Session:      w.session,
		State:        w.state,
		CurrentPhase: w.curPhase,
		StartedAt:    w.started,
		UpdatedAt:    time.Now().UTC(),
		LastSeq:      w.seq,
		LastEvent:    last,
		Counts:       w.counts,
		FinalVerdict: w.finalV,
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return fmt.Errorf("conduit: marshal status: %w", err)
	}
	tmp := w.statusPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("conduit: write status tmp: %w", err)
	}
	if err := os.Rename(tmp, w.statusPath); err != nil {
		return fmt.Errorf("conduit: rename status: %w", err)
	}
	return nil
}

// Err implements Sink.
func (w *Writer) Err() error {
	if v := w.firstErr.Load(); v != nil {
		if e, ok := v.(error); ok {
			return e
		}
	}
	return nil
}

func (w *Writer) setErr(err error) {
	if err == nil {
		return
	}
	w.firstErr.CompareAndSwap(nil, err)
}

// Close flushes and closes the stream file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.stream == nil {
		return nil
	}
	err := w.stream.Close()
	w.stream = nil
	return err
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
