// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package conduit

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// Monitor is the conductor-side consumer of the event channel. It
// tails the JSONL stream as the QA session appends to it and delivers
// each parsed Event to the caller in order. It is the mechanism that
// lets an external orchestrator stay in real-time sync with HelixQA
// without parsing stdout.
//
// Monitor follows the file across growth (like `tail -f`) and stops
// when the context is cancelled or — optionally — when it sees a
// terminal session_end event.
type Monitor struct {
	path        string
	pollEvery   time.Duration
	stopOnEnd   bool
	fromStart   bool
	maxLineSize int
}

// MonitorOption configures a Monitor.
type MonitorOption func(*Monitor)

// WithPollInterval sets how often the Monitor checks for new data
// when it has reached EOF. Default 200ms.
func WithPollInterval(d time.Duration) MonitorOption {
	return func(m *Monitor) {
		if d > 0 {
			m.pollEvery = d
		}
	}
}

// StopOnSessionEnd makes Tail return (nil) once a session_end event
// is delivered. Without it, Tail runs until the context is cancelled.
func StopOnSessionEnd() MonitorOption {
	return func(m *Monitor) { m.stopOnEnd = true }
}

// FromStart replays existing events from the beginning of the file
// before following new ones. Default: follow from the current end so
// a late-attaching conductor only sees fresh events. Use FromStart to
// reconstruct full history.
func FromStart() MonitorOption {
	return func(m *Monitor) { m.fromStart = true }
}

// NewMonitor creates a Monitor for the given JSONL stream path.
func NewMonitor(streamPath string, opts ...MonitorOption) *Monitor {
	m := &Monitor{
		path:        streamPath,
		pollEvery:   200 * time.Millisecond,
		maxLineSize: 1 << 20, // 1 MiB per event line ceiling
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// Handler receives each parsed event. Returning a non-nil error stops
// the Tail loop and surfaces the error to the caller.
type Handler func(ev Event) error

// Tail follows the stream and invokes h for every event in order.
// It blocks until: the context is cancelled (returns ctx.Err()), a
// terminal session_end event is seen with StopOnSessionEnd (returns
// nil), the handler returns an error (returned as-is), or an
// unrecoverable read error occurs.
//
// Tail tolerates the file not existing yet (the session may not have
// created it): it waits, polling, until the file appears or the
// context is cancelled.
func (m *Monitor) Tail(ctx context.Context, h Handler) error {
	f, err := m.openWithWait(ctx)
	if err != nil {
		return err
	}
	defer f.Close()

	if !m.fromStart {
		if _, err := f.Seek(0, io.SeekEnd); err != nil {
			return fmt.Errorf("conduit: seek end: %w", err)
		}
	}

	reader := bufio.NewReaderSize(f, 64*1024)
	var partial []byte

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			partial = append(partial, line...)
			if partial[len(partial)-1] == '\n' {
				// Complete line.
				trimmed := dropCR(partial[:len(partial)-1])
				partial = nil
				if len(trimmed) == 0 {
					continue
				}
				if len(trimmed) > m.maxLineSize {
					// Skip absurdly large lines rather than choke.
					continue
				}
				var ev Event
				if jerr := json.Unmarshal(trimmed, &ev); jerr != nil {
					// A malformed line is surfaced as a log-level
					// signal to the handler via an EventError so the
					// conductor knows the stream had a glitch, rather
					// than silently dropping (no-bluff).
					ev = Event{Type: EventError, Reason: "malformed_stream_line", Detail: jerr.Error()}
				}
				if herr := h(ev); herr != nil {
					return herr
				}
				if m.stopOnEnd && ev.Type == EventSessionEnd {
					return nil
				}
			}
		}

		if err == nil {
			continue
		}
		if err == io.EOF {
			// Reached current end — wait for more data.
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(m.pollEvery):
			}
			continue
		}
		return fmt.Errorf("conduit: read stream: %w", err)
	}
}

// Collect tails the stream and returns all events once a terminal
// session_end is seen (or the context is cancelled). It is a
// convenience for conductors that want the whole session at once
// (tests, post-hoc audits). Implies StopOnSessionEnd + FromStart.
func Collect(ctx context.Context, streamPath string, opts ...MonitorOption) ([]Event, error) {
	m := NewMonitor(streamPath, append([]MonitorOption{FromStart(), StopOnSessionEnd()}, opts...)...)
	var out []Event
	err := m.Tail(ctx, func(ev Event) error {
		out = append(out, ev)
		return nil
	})
	return out, err
}

// ReadStatus reads and parses the latest status snapshot file. It is
// the O(1) "where are we now" view for a conductor that does not want
// to tail the whole stream.
func ReadStatus(statusPath string) (*Status, error) {
	data, err := os.ReadFile(statusPath)
	if err != nil {
		return nil, fmt.Errorf("conduit: read status: %w", err)
	}
	var st Status
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("conduit: parse status: %w", err)
	}
	return &st, nil
}

// openWithWait opens the stream file, waiting (polling) for it to
// appear if it does not exist yet.
func (m *Monitor) openWithWait(ctx context.Context) (*os.File, error) {
	for {
		f, err := os.Open(m.path)
		if err == nil {
			return f, nil
		}
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("conduit: open stream %q: %w", m.path, err)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(m.pollEvery):
		}
	}
}

func dropCR(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\r' {
		return b[:len(b)-1]
	}
	return b
}
