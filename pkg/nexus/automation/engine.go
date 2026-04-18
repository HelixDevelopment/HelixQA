// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"bytes"
	"context"
	"fmt"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// Engine unifies the P1–P5 primitives behind one Perform() call. It
// is stateless across Actions: every Perform() does dispatch → verify
// → evidence-collect and returns a Result the Agent consumes.
//
// Engine never decides WHAT to do — the LLM / Agent state machine
// produces every Action; Engine only executes and reports.
type Engine struct {
	capture  contracts.CaptureSource
	vision   contracts.VisionPipeline
	interact contracts.Interactor
	observer contracts.Observer
	recorder contracts.Recorder
	clock    func() time.Time
}

// New wires the five P1–P5 sub-engines into an Engine. All
// parameters must be non-nil; callers that do not need a particular
// sub-engine must supply a stub that satisfies the interface.
func New(
	cap contracts.CaptureSource,
	vis contracts.VisionPipeline,
	inter contracts.Interactor,
	obs contracts.Observer,
	rec contracts.Recorder,
) *Engine {
	if cap == nil {
		panic("automation.New: nil CaptureSource")
	}
	if vis == nil {
		panic("automation.New: nil VisionPipeline")
	}
	if inter == nil {
		panic("automation.New: nil Interactor")
	}
	if obs == nil {
		panic("automation.New: nil Observer")
	}
	if rec == nil {
		panic("automation.New: nil Recorder")
	}
	return &Engine{
		capture:  cap,
		vision:   vis,
		interact: inter,
		observer: obs,
		recorder: rec,
		clock:    time.Now,
	}
}

// Perform dispatches one pre-decided Action to the appropriate
// sub-engine, records evidence, and returns a Result. The caller
// must not mutate a in flight.
//
// Perform is safe for concurrent use: each call operates on its own
// stack-local Result and does not mutate Engine state.
func (e *Engine) Perform(ctx context.Context, a Action) (res Result, err error) { //nolint:unparam
	start := e.clock()
	defer func() {
		elapsed := e.clock().Sub(start)
		if elapsed < 1 {
			elapsed = 1 // floor at 1 ns; any real dispatch takes at least this
		}
		res.Elapsed = elapsed
	}()

	switch a.Kind {
	case ActionClick:
		err := e.interact.Click(ctx, a.At, contracts.ClickOptions{Button: a.Button})
		if err != nil {
			res.Error = err.Error()
			return res, nil
		}
		res.Success = true

	case ActionType:
		err := e.interact.Type(ctx, a.Text, contracts.TypeOptions{})
		if err != nil {
			res.Error = err.Error()
			return res, nil
		}
		res.Success = true

	case ActionScroll:
		err := e.interact.Scroll(ctx, a.At, float64(a.DX), float64(a.DY))
		if err != nil {
			res.Error = err.Error()
			return res, nil
		}
		res.Success = true

	case ActionKey:
		err := e.interact.Key(ctx, a.Key, contracts.KeyOptions{})
		if err != nil {
			res.Error = err.Error()
			return res, nil
		}
		res.Success = true

	case ActionDrag:
		err := e.interact.Drag(ctx, a.At, a.To, contracts.DragOptions{Button: a.Button})
		if err != nil {
			res.Error = err.Error()
			return res, nil
		}
		res.Success = true

	case ActionCapture:
		// Pull the latest available frame; non-blocking.
		select {
		case f, ok := <-e.capture.Frames():
			if !ok {
				res.Error = "capture: frames channel closed"
				return res, nil
			}
			ref := fmt.Sprintf("seq-%d", f.Seq)
			_ = f.Data.Release()
			res.Success = true
			res.Evidence = append(res.Evidence, EvidenceRef{
				Kind: "screenshot_before",
				Ref:  ref,
			})
		default:
			res.Error = "capture: no frame available"
		}

	case ActionAnalyze:
		select {
		case f, ok := <-e.capture.Frames():
			if !ok {
				res.Error = "analyze: frames channel closed"
				return res, nil
			}
			analysis, err := e.vision.Analyze(ctx, f)
			_ = f.Data.Release()
			if err != nil {
				res.Error = err.Error()
				return res, nil
			}
			res.Success = true
			if analysis != nil {
				res.DispatchedTo = analysis.DispatchedTo
			}
		default:
			res.Error = "analyze: no frame available"
		}

	case ActionRecordClip:
		around := time.Unix(0, a.ClipAround)
		window := time.Duration(a.ClipWindow)
		var buf bytes.Buffer
		err := e.recorder.Clip(around, window, &buf, contracts.ClipOptions{})
		if err != nil {
			res.Error = err.Error()
			return res, nil
		}
		res.Success = true
		res.Evidence = append(res.Evidence, EvidenceRef{
			Kind: "clip",
			Ref:  fmt.Sprintf("%d bytes", buf.Len()),
		})

	default:
		return res, fmt.Errorf("automation: unsupported action kind %q", a.Kind)
	}

	return res, nil
}
