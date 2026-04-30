// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Explorer hook surface — defines the interface a future LLM-driven
// vision-nav implementation calls to capture evidence + record
// findings during autonomous test exploration.
//
// Constitution §11.4: this is the SCAFFOLDING. The actual LLM
// integration is not in this package; that's Issues.md C6 future
// work. By defining the interface now, the LLM integration can
// land later without touching pkg/audio or rewriting the
// captured-evidence layer.

package visionnav

import (
	"context"
	"fmt"
	"time"

	"digital.vasic.helixqa/pkg/audio"
)

// Explorer is the interface a vision-nav driver implements. The
// driver is the thing (LLM, scripted bank, human reviewer) that
// decides WHAT to test next. It calls back into Capture* methods
// to record evidence as it explores.
//
// Future LLM implementation will satisfy this interface by reading
// screen captures + audio chunks, calling pkg/llm to decide the
// next action, then invoking CaptureFinding to persist the verdict.
type Explorer interface {
	// Name identifies the explorer in logs (e.g.
	// "anthropic-claude-vision-nav-v1", "scripted-bank-yaml").
	Name() string

	// CaptureFinding records a single discovery + verdict using
	// the supplied audio + image evidence sources. The Explorer
	// implementation is responsible for ensuring the audio /
	// image paths point at content from the moment of discovery
	// (not a different time).
	CaptureFinding(ctx context.Context, opts FindingOptions) (*Evidence, error)
}

// FindingOptions describes a discovery the explorer wants to record.
type FindingOptions struct {
	// Description in the explorer's own words. Required.
	Description string
	// Verdict is "pass", "fail", or "needs-review". Required.
	Verdict string
	// AudioPath is the path to the audio clip captured at the
	// moment of discovery. Optional; if empty, no Whisper
	// transcript is produced.
	AudioPath string
	// AudioOpts are forwarded to WhisperClient.Transcribe.
	AudioOpts audio.TranscribeOptions
	// ImagePath is the path to the screen frame captured at the
	// moment of discovery. Optional; if empty, no Tesseract OCR
	// is produced.
	ImagePath string
	// ImageOpts are forwarded to TesseractClient.OCR.
	ImageOpts audio.OCROptions
	// Notes are free-form context (e.g. "audio dipped at step 3").
	Notes string
}

// DefaultExplorer is the standard implementation of Explorer that
// uses the Phase 23.8/23.9 pkg/audio Whisper + Tesseract HTTP
// clients. It does NOT decide what to explore — that's the caller's
// job. It just turns FindingOptions into validated Evidence and
// hands it to the sink.
//
// Future LLM-driven explorers can either embed DefaultExplorer for
// the capture-and-persist plumbing, or implement Explorer directly.
type DefaultExplorer struct {
	name      string
	whisper   *audio.WhisperClient
	tesseract *audio.TesseractClient
	sink      EvidenceSink
}

// NewDefaultExplorer wires the named explorer with Whisper + Tesseract
// clients and an EvidenceSink. Pass nil whisper to disable audio
// capture, nil tesseract to disable OCR — but at least ONE must be
// non-nil (Validate enforces that no Evidence ships with neither
// transcript nor OCR).
func NewDefaultExplorer(
	name string,
	whisper *audio.WhisperClient,
	tesseract *audio.TesseractClient,
	sink EvidenceSink,
) (*DefaultExplorer, error) {
	if name == "" {
		return nil, fmt.Errorf("visionnav: explorer name is required")
	}
	if sink == nil {
		return nil, fmt.Errorf("visionnav: EvidenceSink is required")
	}
	if whisper == nil && tesseract == nil {
		return nil, fmt.Errorf("visionnav: at least one of whisper/tesseract must be non-nil " +
			"(otherwise no Evidence can ever satisfy the §11.4 captured-evidence rule)")
	}
	return &DefaultExplorer{
		name:      name,
		whisper:   whisper,
		tesseract: tesseract,
		sink:      sink,
	}, nil
}

// Name returns the explorer's identifier.
func (e *DefaultExplorer) Name() string { return e.name }

// CaptureFinding implements Explorer. Calls Whisper if AudioPath set
// AND whisper client wired; calls Tesseract if ImagePath set AND
// tesseract client wired; persists the resulting Evidence via the
// sink. Returns the Evidence so the caller can also act on it
// (e.g. abort exploration on first "fail" verdict).
func (e *DefaultExplorer) CaptureFinding(ctx context.Context, opts FindingOptions) (*Evidence, error) {
	ev := &Evidence{
		Timestamp:   time.Now().UTC(),
		Description: opts.Description,
		Verdict:     opts.Verdict,
		Notes:       opts.Notes,
	}

	if opts.AudioPath != "" && e.whisper != nil {
		t, err := e.whisper.Transcribe(ctx, opts.AudioPath, opts.AudioOpts)
		if err != nil {
			return nil, fmt.Errorf("visionnav: %s: whisper transcribe failed: %w", e.name, err)
		}
		ev.Transcript = t
	}

	if opts.ImagePath != "" && e.tesseract != nil {
		txt, err := e.tesseract.OCR(ctx, opts.ImagePath, opts.ImageOpts)
		if err != nil {
			return nil, fmt.Errorf("visionnav: %s: tesseract OCR failed: %w", e.name, err)
		}
		ev.OCRSnapshot = txt
	}

	if err := e.sink.Record(ctx, ev); err != nil {
		return nil, fmt.Errorf("visionnav: %s: sink record failed: %w", e.name, err)
	}
	return ev, nil
}
