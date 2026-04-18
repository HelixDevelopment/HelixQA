// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"context"
	"io"
	"testing"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ── bench stubs (lightweight, zero allocs where possible) ─────────────────

type benchFrameData struct{}

func (b *benchFrameData) AsBytes() ([]byte, error)                  { return []byte{0}, nil }
func (b *benchFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (b *benchFrameData) Release() error                            { return nil }

type benchCapture struct{ seq uint64 }

func (c *benchCapture) Name() string                                             { return "bench" }
func (c *benchCapture) Start(_ context.Context, _ contracts.CaptureConfig) error { return nil }
func (c *benchCapture) Stop() error                                              { return nil }
func (c *benchCapture) Stats() contracts.CaptureStats                            { return contracts.CaptureStats{} }
func (c *benchCapture) Close() error                                             { return nil }

// Frames returns a fresh buffered channel with one frame per call so the
// bench engine always finds a frame without blocking.
func (c *benchCapture) Frames() <-chan contracts.Frame {
	ch := make(chan contracts.Frame, 1)
	c.seq++
	ch <- contracts.Frame{Seq: c.seq, Timestamp: time.Now(), Width: 1, Height: 1, Data: &benchFrameData{}}
	return ch
}

type benchVision struct{}

func (v *benchVision) Analyze(_ context.Context, _ contracts.Frame) (*contracts.Analysis, error) {
	return &contracts.Analysis{DispatchedTo: "local-cpu"}, nil
}
func (v *benchVision) Match(_ context.Context, _ contracts.Frame, _ contracts.Template) ([]contracts.Match, error) {
	return nil, nil
}
func (v *benchVision) Diff(_ context.Context, _, _ contracts.Frame) (*contracts.DiffResult, error) {
	return &contracts.DiffResult{TotalDelta: 1.0, SameShape: true}, nil
}
func (v *benchVision) OCR(_ context.Context, _ contracts.Frame, _ contracts.Rect) (contracts.OCRResult, error) {
	return contracts.OCRResult{}, nil
}

type benchInteractor struct{}

func (i *benchInteractor) Click(_ context.Context, _ contracts.Point, _ contracts.ClickOptions) error {
	return nil
}
func (i *benchInteractor) Type(_ context.Context, _ string, _ contracts.TypeOptions) error {
	return nil
}
func (i *benchInteractor) Scroll(_ context.Context, _ contracts.Point, _, _ float64) error {
	return nil
}
func (i *benchInteractor) Key(_ context.Context, _ contracts.KeyCode, _ contracts.KeyOptions) error {
	return nil
}
func (i *benchInteractor) Drag(_ context.Context, _, _ contracts.Point, _ contracts.DragOptions) error {
	return nil
}

type benchObserver struct{}

func (o *benchObserver) Start(_ context.Context, _ contracts.Target) error { return nil }
func (o *benchObserver) Events() <-chan contracts.Event {
	ch := make(chan contracts.Event)
	close(ch)
	return ch
}
func (o *benchObserver) Snapshot(_ time.Time, _ time.Duration) ([]contracts.Event, error) {
	return nil, nil
}
func (o *benchObserver) Stop() error { return nil }

type benchRecorder struct{}

func (r *benchRecorder) AttachSource(_ contracts.CaptureSource) error            { return nil }
func (r *benchRecorder) Start(_ context.Context, _ contracts.RecordConfig) error { return nil }
func (r *benchRecorder) Clip(_ time.Time, _ time.Duration, out io.Writer, _ contracts.ClipOptions) error {
	_, err := io.WriteString(out, `{}`)
	return err
}
func (r *benchRecorder) LiveStream(_ context.Context) (string, error) { return "", nil }
func (r *benchRecorder) Stop() error                                  { return nil }

// ── benchmarks ─────────────────────────────────────────────────────────────

func newBenchEngine() *Engine {
	return New(&benchCapture{}, &benchVision{}, &benchInteractor{}, &benchObserver{}, &benchRecorder{})
}

func BenchmarkEngine_Click(b *testing.B) {
	eng := newBenchEngine()
	ctx := context.Background()
	a := Action{Kind: ActionClick, At: contracts.Point{X: 100, Y: 200}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = eng.Perform(ctx, a)
	}
}

func BenchmarkEngine_Type(b *testing.B) {
	eng := newBenchEngine()
	ctx := context.Background()
	a := Action{Kind: ActionType, Text: "benchmark text input"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = eng.Perform(ctx, a)
	}
}

func BenchmarkEngine_Scroll(b *testing.B) {
	eng := newBenchEngine()
	ctx := context.Background()
	a := Action{Kind: ActionScroll, At: contracts.Point{X: 50, Y: 50}, DY: 3}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = eng.Perform(ctx, a)
	}
}

func BenchmarkEngine_Key(b *testing.B) {
	eng := newBenchEngine()
	ctx := context.Background()
	a := Action{Kind: ActionKey, Key: contracts.KeyEnter}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = eng.Perform(ctx, a)
	}
}

func BenchmarkEngine_Drag(b *testing.B) {
	eng := newBenchEngine()
	ctx := context.Background()
	a := Action{Kind: ActionDrag, At: contracts.Point{X: 0, Y: 0}, To: contracts.Point{X: 100, Y: 100}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = eng.Perform(ctx, a)
	}
}

func BenchmarkEngine_Capture(b *testing.B) {
	ctx := context.Background()
	a := Action{Kind: ActionCapture}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng := newBenchEngine() // fresh frame per iteration
		_, _ = eng.Perform(ctx, a)
	}
}

func BenchmarkEngine_Analyze(b *testing.B) {
	ctx := context.Background()
	a := Action{Kind: ActionAnalyze}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		eng := newBenchEngine() // fresh frame per iteration
		_, _ = eng.Perform(ctx, a)
	}
}

func BenchmarkEngine_RecordClip(b *testing.B) {
	eng := newBenchEngine()
	ctx := context.Background()
	a := Action{Kind: ActionRecordClip, ClipAround: time.Now().UnixNano(), ClipWindow: int64(time.Second)}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = eng.Perform(ctx, a)
	}
}
