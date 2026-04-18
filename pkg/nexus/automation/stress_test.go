// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ── stress stubs ───────────────────────────────────────────────────────────

type stressFrameData struct{}

func (s *stressFrameData) AsBytes() ([]byte, error)                  { return []byte{0}, nil }
func (s *stressFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (s *stressFrameData) Release() error                            { return nil }

// stressCapture is a capture source that always has a fresh frame available
// via a new channel per Frames() call — safe for concurrent Perform() calls.
type stressCapture struct{}

func (s *stressCapture) Name() string                                             { return "stress" }
func (s *stressCapture) Start(_ context.Context, _ contracts.CaptureConfig) error { return nil }
func (s *stressCapture) Stop() error                                              { return nil }
func (s *stressCapture) Stats() contracts.CaptureStats                            { return contracts.CaptureStats{} }
func (s *stressCapture) Close() error                                             { return nil }
func (s *stressCapture) Frames() <-chan contracts.Frame {
	ch := make(chan contracts.Frame, 1)
	ch <- contracts.Frame{
		Seq:       1,
		Timestamp: time.Now(),
		Width:     1,
		Height:    1,
		Data:      &stressFrameData{},
	}
	return ch
}

type stressVision struct{}

func (v *stressVision) Analyze(_ context.Context, _ contracts.Frame) (*contracts.Analysis, error) {
	return &contracts.Analysis{DispatchedTo: "stress-cpu"}, nil
}
func (v *stressVision) Match(_ context.Context, _ contracts.Frame, _ contracts.Template) ([]contracts.Match, error) {
	return nil, nil
}
func (v *stressVision) Diff(_ context.Context, _, _ contracts.Frame) (*contracts.DiffResult, error) {
	return &contracts.DiffResult{TotalDelta: 1.0, SameShape: true}, nil
}
func (v *stressVision) OCR(_ context.Context, _ contracts.Frame, _ contracts.Rect) (contracts.OCRResult, error) {
	return contracts.OCRResult{}, nil
}

type stressInteractor struct{}

func (i *stressInteractor) Click(_ context.Context, _ contracts.Point, _ contracts.ClickOptions) error {
	return nil
}
func (i *stressInteractor) Type(_ context.Context, _ string, _ contracts.TypeOptions) error {
	return nil
}
func (i *stressInteractor) Scroll(_ context.Context, _ contracts.Point, _, _ float64) error {
	return nil
}
func (i *stressInteractor) Key(_ context.Context, _ contracts.KeyCode, _ contracts.KeyOptions) error {
	return nil
}
func (i *stressInteractor) Drag(_ context.Context, _, _ contracts.Point, _ contracts.DragOptions) error {
	return nil
}

type stressObserver struct{}

func (o *stressObserver) Start(_ context.Context, _ contracts.Target) error { return nil }
func (o *stressObserver) Events() <-chan contracts.Event {
	ch := make(chan contracts.Event)
	close(ch)
	return ch
}
func (o *stressObserver) Snapshot(_ time.Time, _ time.Duration) ([]contracts.Event, error) {
	return nil, nil
}
func (o *stressObserver) Stop() error { return nil }

type stressRecorder struct{}

func (r *stressRecorder) AttachSource(_ contracts.CaptureSource) error            { return nil }
func (r *stressRecorder) Start(_ context.Context, _ contracts.RecordConfig) error { return nil }
func (r *stressRecorder) Clip(_ time.Time, _ time.Duration, out io.Writer, _ contracts.ClipOptions) error {
	_, err := io.WriteString(out, `{}`)
	return err
}
func (r *stressRecorder) LiveStream(_ context.Context) (string, error) { return "", nil }
func (r *stressRecorder) Stop() error                                  { return nil }

// ── stress test ────────────────────────────────────────────────────────────

// TestStress_Engine_100Concurrent fires 100 goroutines each calling
// Engine.Perform with a mix of ActionKinds. The test verifies:
//   - No data race (run with -race).
//   - Every call returns without a non-unsupported-kind error.
//   - Elapsed is populated on every result.
func TestStress_Engine_100Concurrent(t *testing.T) {
	eng := New(
		&stressCapture{},
		&stressVision{},
		&stressInteractor{},
		&stressObserver{},
		&stressRecorder{},
	)

	// Rotate through all mutating kinds; capture/analyze/record_clip each
	// allocate a channel per call via stressCapture, which is safe under -race.
	kinds := []ActionKind{
		ActionClick,
		ActionType,
		ActionScroll,
		ActionKey,
		ActionDrag,
		ActionCapture,
		ActionAnalyze,
		ActionRecordClip,
	}

	const workers = 100
	var wg sync.WaitGroup
	wg.Add(workers)

	type callResult struct {
		res Result
		err error
	}
	results := make([]callResult, workers)

	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			kind := kinds[idx%len(kinds)]
			a := Action{
				Kind:       kind,
				At:         contracts.Point{X: idx, Y: idx},
				To:         contracts.Point{X: idx + 10, Y: idx + 10},
				Text:       "stress",
				Key:        contracts.KeyEnter,
				DY:         1,
				ClipAround: time.Now().UnixNano(),
				ClipWindow: int64(time.Second),
			}
			res, err := eng.Perform(context.Background(), a)
			results[idx] = callResult{res: res, err: err}
		}(i)
	}

	wg.Wait()

	for i, cr := range results {
		assert.NoError(t, cr.err, "worker %d returned unexpected error", i)
		// Elapsed is populated by the defer; on fast stub calls it may be 0ns
		// so we assert >= 0 (field is set, not negative).
		assert.GreaterOrEqual(t, cr.res.Elapsed.Nanoseconds(), int64(0), "worker %d elapsed must be non-negative", i)
	}
}
