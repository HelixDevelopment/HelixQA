// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package visionnav scaffolds the captured-evidence hook surface
// for HelixQA's autonomous vision-navigated testing pipeline
// (Issues.md C6).
//
// Constitution §11.4: every discovery a future LLM-driven explorer
// makes MUST produce captured positive evidence per finding —
// Whisper transcript of audio at the moment of discovery, Tesseract
// OCR of the screen frame, both stored alongside the verdict.
// Without this, the autonomous pipeline becomes a bluff factory:
// the LLM says "found a bug at step N" with no replayable evidence.
//
// This package defines:
//   - Evidence struct: the captured-evidence record per finding
//   - EvidenceSink interface: how evidence gets persisted
//   - DefaultSink: implementation backed by pkg/audio Whisper + Tesseract
//
// LLM integration is NOT in this commit. The package provides the
// hook surface so future autonomy work has a stable target. See
// Issues.md C6 operator-unblock runbook for next steps.

package visionnav

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"digital.vasic.helixqa/pkg/audio"
)

// Evidence is the captured-evidence record for a single discovery
// the vision-nav explorer makes. Every field is required; a record
// with empty Transcript or empty OCRSnapshot is a bluff.
type Evidence struct {
	// Timestamp the evidence was captured (UTC, ISO 8601).
	Timestamp time.Time `json:"timestamp"`
	// Description of the discovery in the explorer's own words
	// (e.g. "Settings dialog opened with title 'Network'").
	// Free-form but the explorer MUST author one.
	Description string `json:"description"`
	// Transcript is the Whisper transcript of audio captured at the
	// moment of discovery. Carries per-word probability so a reviewer
	// can flag low-confidence transcriptions without re-running.
	Transcript *audio.TranscribeResult `json:"transcript,omitempty"`
	// OCRSnapshot is the Tesseract OCR of the screen frame at the
	// moment of discovery. Raw text (not trimmed by us) so the
	// reviewer sees exact engine output.
	OCRSnapshot string `json:"ocr_snapshot"`
	// Verdict is "pass", "fail", or "needs-review".
	Verdict string `json:"verdict"`
	// Notes are free-form explorer-supplied context (e.g. "audio
	// volume dipped after step 3, possibly related to D8").
	Notes string `json:"notes,omitempty"`
}

// Validate returns an error if Evidence is bluff-structured (missing
// the captured-evidence fields). This is the §11.4 enforcement gate
// at the data layer — even an explorer that ignores conventions
// can't ship Evidence that lacks BOTH transcript and OCR.
func (e *Evidence) Validate() error {
	if e == nil {
		return fmt.Errorf("visionnav: nil Evidence")
	}
	if e.Description == "" {
		return fmt.Errorf("visionnav: Evidence.Description is empty (no discovery description)")
	}
	if e.Verdict == "" {
		return fmt.Errorf("visionnav: Evidence.Verdict is empty")
	}
	switch e.Verdict {
	case "pass", "fail", "needs-review":
		// ok
	default:
		return fmt.Errorf("visionnav: Evidence.Verdict %q invalid (want pass/fail/needs-review)", e.Verdict)
	}
	// At least one captured-evidence source MUST be present. A
	// pass/fail verdict without either transcript or OCR is a bluff.
	if e.Transcript == nil && e.OCRSnapshot == "" {
		return fmt.Errorf("visionnav: Evidence has neither Transcript nor OCRSnapshot — bluff verdict")
	}
	return nil
}

// EvidenceSink persists Evidence records. Implementations MUST be
// safe for concurrent calls from multiple explorer goroutines.
type EvidenceSink interface {
	// Record persists e. Returns an error if e fails Validate or
	// the underlying storage fails.
	Record(ctx context.Context, e *Evidence) error
	// Count returns how many records have been persisted in this
	// sink so far (lifetime, not per-session). Useful for
	// captured-evidence assertions in higher-level tests.
	Count() int
}

// FileSink writes Evidence records as JSON files to a directory.
// Filename pattern: <timestamp>_<verdict>_<short-desc>.json.
//
// Concurrent-safe: a mutex serializes writes so two explorers
// reporting at the same instant don't collide on filename.
type FileSink struct {
	dir   string
	mu    sync.Mutex
	count int
}

// NewFileSink creates a FileSink that writes into dir (created if
// missing). Returns error if dir cannot be created.
func NewFileSink(dir string) (*FileSink, error) {
	if dir == "" {
		return nil, fmt.Errorf("visionnav: NewFileSink: dir is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("visionnav: mkdir %q: %w", dir, err)
	}
	return &FileSink{dir: dir}, nil
}

// Record validates + writes e to disk. Filename embeds the verdict
// so a `ls` of the sink dir surfaces pass/fail counts at a glance.
func (s *FileSink) Record(_ context.Context, e *Evidence) error {
	if err := e.Validate(); err != nil {
		return err
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Build filename. Strip any path separators from description to
	// avoid directory-escape via crafted Description (defence in
	// depth even though Description comes from in-process explorer).
	short := safeFilenameSegment(e.Description)
	if len(short) > 60 {
		short = short[:60]
	}
	name := fmt.Sprintf("%s_%s_%s.json",
		e.Timestamp.Format("20060102T150405.000000Z"),
		e.Verdict,
		short,
	)
	path := filepath.Join(s.dir, name)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("visionnav: create %q: %w", path, err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(e); err != nil {
		return fmt.Errorf("visionnav: encode %q: %w", path, err)
	}
	s.count++
	return nil
}

// Count returns the lifetime number of records persisted.
func (s *FileSink) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.count
}

// safeFilenameSegment strips path separators + condenses whitespace
// so a multi-word Description becomes a single filesystem-safe token.
func safeFilenameSegment(s string) string {
	out := make([]byte, 0, len(s))
	prevDash := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9'):
			out = append(out, c)
			prevDash = false
		case c == '-' || c == '_':
			out = append(out, c)
			prevDash = (c == '-')
		default:
			if !prevDash {
				out = append(out, '-')
				prevDash = true
			}
		}
	}
	if len(out) == 0 {
		return "unnamed"
	}
	return string(out)
}
