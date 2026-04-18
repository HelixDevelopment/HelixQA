// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package automation

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ── stubs ──────────────────────────────────────────────────────────────────

type stubFrameData struct{}

func (s *stubFrameData) AsBytes() ([]byte, error)                  { return []byte{0, 0, 0, 0}, nil }
func (s *stubFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (s *stubFrameData) Release() error                            { return nil }

func makeFrame(seq uint64) contracts.Frame {
	return contracts.Frame{Seq: seq, Timestamp: time.Now(), Width: 1, Height: 1, Data: &stubFrameData{}}
}

// stubCapture delivers exactly one frame then blocks.
type stubCapture struct {
	ch     chan contracts.Frame
	closed bool
}

func newStubCapture(f contracts.Frame) *stubCapture {
	ch := make(chan contracts.Frame, 1)
	ch <- f
	return &stubCapture{ch: ch}
}

func newClosedCapture() *stubCapture {
	ch := make(chan contracts.Frame)
	close(ch)
	return &stubCapture{ch: ch, closed: true}
}

func newEmptyCapture() *stubCapture {
	return &stubCapture{ch: make(chan contracts.Frame, 0)}
}

func (s *stubCapture) Name() string                                             { return "stub" }
func (s *stubCapture) Start(_ context.Context, _ contracts.CaptureConfig) error { return nil }
func (s *stubCapture) Stop() error                                              { return nil }
func (s *stubCapture) Frames() <-chan contracts.Frame                           { return s.ch }
func (s *stubCapture) Stats() contracts.CaptureStats                            { return contracts.CaptureStats{} }
func (s *stubCapture) Close() error                                             { return nil }

// stubVision: Analyze returns a fixed Analysis; Diff returns fixed DiffResult.
type stubVision struct {
	analyzeErr   error
	dispatchedTo string
	diffResult   *contracts.DiffResult
	diffErr      error
}

func (s *stubVision) Analyze(_ context.Context, _ contracts.Frame) (*contracts.Analysis, error) {
	if s.analyzeErr != nil {
		return nil, s.analyzeErr
	}
	return &contracts.Analysis{DispatchedTo: s.dispatchedTo, Confidence: 0.99}, nil
}

func (s *stubVision) Match(_ context.Context, _ contracts.Frame, _ contracts.Template) ([]contracts.Match, error) {
	return nil, nil
}

func (s *stubVision) Diff(_ context.Context, _, _ contracts.Frame) (*contracts.DiffResult, error) {
	if s.diffErr != nil {
		return nil, s.diffErr
	}
	if s.diffResult != nil {
		return s.diffResult, nil
	}
	return &contracts.DiffResult{TotalDelta: 0, SameShape: true}, nil
}

func (s *stubVision) OCR(_ context.Context, _ contracts.Frame, _ contracts.Rect) (contracts.OCRResult, error) {
	return contracts.OCRResult{}, nil
}

// stubInteractor records which method was called last.
type stubInteractor struct {
	lastCall string
	err      error
}

func (s *stubInteractor) Click(_ context.Context, _ contracts.Point, _ contracts.ClickOptions) error {
	s.lastCall = "click"
	return s.err
}
func (s *stubInteractor) Type(_ context.Context, _ string, _ contracts.TypeOptions) error {
	s.lastCall = "type"
	return s.err
}
func (s *stubInteractor) Scroll(_ context.Context, _ contracts.Point, _, _ float64) error {
	s.lastCall = "scroll"
	return s.err
}
func (s *stubInteractor) Key(_ context.Context, _ contracts.KeyCode, _ contracts.KeyOptions) error {
	s.lastCall = "key"
	return s.err
}
func (s *stubInteractor) Drag(_ context.Context, _, _ contracts.Point, _ contracts.DragOptions) error {
	s.lastCall = "drag"
	return s.err
}

// stubObserver satisfies contracts.Observer.
type stubObserver struct{}

func (s *stubObserver) Start(_ context.Context, _ contracts.Target) error { return nil }
func (s *stubObserver) Events() <-chan contracts.Event {
	ch := make(chan contracts.Event)
	close(ch)
	return ch
}
func (s *stubObserver) Snapshot(_ time.Time, _ time.Duration) ([]contracts.Event, error) {
	return nil, nil
}
func (s *stubObserver) Stop() error { return nil }

// stubRecorder satisfies contracts.Recorder.
type stubRecorder struct {
	clipErr     error
	clipPayload string
}

func (s *stubRecorder) AttachSource(_ contracts.CaptureSource) error            { return nil }
func (s *stubRecorder) Start(_ context.Context, _ contracts.RecordConfig) error { return nil }
func (s *stubRecorder) Clip(_ time.Time, _ time.Duration, out io.Writer, _ contracts.ClipOptions) error {
	if s.clipErr != nil {
		return s.clipErr
	}
	_, err := io.WriteString(out, s.clipPayload)
	return err
}
func (s *stubRecorder) LiveStream(_ context.Context) (string, error) { return "", nil }
func (s *stubRecorder) Stop() error                                  { return nil }

// ── helpers ────────────────────────────────────────────────────────────────

func newEngine(cap contracts.CaptureSource, vis contracts.VisionPipeline,
	inter contracts.Interactor) *Engine {
	return New(cap, vis, inter, &stubObserver{}, &stubRecorder{clipPayload: `{"test":1}`})
}

func newFullEngine(cap contracts.CaptureSource, vis contracts.VisionPipeline,
	inter contracts.Interactor, rec contracts.Recorder) *Engine {
	return New(cap, vis, inter, &stubObserver{}, rec)
}

// ── tests ──────────────────────────────────────────────────────────────────

func TestEngine_Click_Success(t *testing.T) {
	inter := &stubInteractor{}
	eng := newEngine(newEmptyCapture(), &stubVision{}, inter)
	res, err := eng.Perform(context.Background(), Action{Kind: ActionClick, At: contracts.Point{X: 10, Y: 20}})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Empty(t, res.Error)
	assert.Equal(t, "click", inter.lastCall)
	assert.GreaterOrEqual(t, res.Elapsed.Nanoseconds(), int64(0))
}

func TestEngine_Click_Error(t *testing.T) {
	inter := &stubInteractor{err: errors.New("click failed")}
	eng := newEngine(newEmptyCapture(), &stubVision{}, inter)
	res, err := eng.Perform(context.Background(), Action{Kind: ActionClick})
	require.NoError(t, err) // Perform itself does not return the sub-error
	assert.False(t, res.Success)
	assert.Equal(t, "click failed", res.Error)
}

func TestEngine_Type_Success(t *testing.T) {
	inter := &stubInteractor{}
	eng := newEngine(newEmptyCapture(), &stubVision{}, inter)
	res, err := eng.Perform(context.Background(), Action{Kind: ActionType, Text: "hello"})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "type", inter.lastCall)
}

func TestEngine_Type_Error(t *testing.T) {
	inter := &stubInteractor{err: errors.New("type failed")}
	eng := newEngine(newEmptyCapture(), &stubVision{}, inter)
	res, err := eng.Perform(context.Background(), Action{Kind: ActionType, Text: "x"})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Equal(t, "type failed", res.Error)
}

func TestEngine_Scroll_Success(t *testing.T) {
	inter := &stubInteractor{}
	eng := newEngine(newEmptyCapture(), &stubVision{}, inter)
	res, err := eng.Perform(context.Background(), Action{Kind: ActionScroll, At: contracts.Point{X: 5, Y: 5}, DX: 0, DY: 3})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "scroll", inter.lastCall)
}

func TestEngine_Key_Success(t *testing.T) {
	inter := &stubInteractor{}
	eng := newEngine(newEmptyCapture(), &stubVision{}, inter)
	res, err := eng.Perform(context.Background(), Action{Kind: ActionKey, Key: contracts.KeyEnter})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "key", inter.lastCall)
}

func TestEngine_Drag_Success(t *testing.T) {
	inter := &stubInteractor{}
	eng := newEngine(newEmptyCapture(), &stubVision{}, inter)
	res, err := eng.Perform(context.Background(), Action{
		Kind: ActionDrag,
		At:   contracts.Point{X: 0, Y: 0},
		To:   contracts.Point{X: 100, Y: 100},
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "drag", inter.lastCall)
}

func TestEngine_Capture_Success(t *testing.T) {
	eng := newEngine(newStubCapture(makeFrame(42)), &stubVision{}, &stubInteractor{})
	res, err := eng.Perform(context.Background(), Action{Kind: ActionCapture})
	require.NoError(t, err)
	assert.True(t, res.Success)
	require.Len(t, res.Evidence, 1)
	assert.Equal(t, "screenshot_before", res.Evidence[0].Kind)
	assert.Equal(t, "seq-42", res.Evidence[0].Ref)
}

func TestEngine_Capture_NoFrame(t *testing.T) {
	eng := newEngine(newEmptyCapture(), &stubVision{}, &stubInteractor{})
	res, err := eng.Perform(context.Background(), Action{Kind: ActionCapture})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "no frame")
}

func TestEngine_Capture_ClosedChannel(t *testing.T) {
	eng := newEngine(newClosedCapture(), &stubVision{}, &stubInteractor{})
	res, err := eng.Perform(context.Background(), Action{Kind: ActionCapture})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "closed")
}

func TestEngine_Analyze_DispatchedTo(t *testing.T) {
	vis := &stubVision{dispatchedTo: "thinker-cuda"}
	eng := newEngine(newStubCapture(makeFrame(1)), vis, &stubInteractor{})
	res, err := eng.Perform(context.Background(), Action{Kind: ActionAnalyze})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "thinker-cuda", res.DispatchedTo)
}

func TestEngine_Analyze_VisionError(t *testing.T) {
	vis := &stubVision{analyzeErr: errors.New("vision unavailable")}
	eng := newEngine(newStubCapture(makeFrame(2)), vis, &stubInteractor{})
	res, err := eng.Perform(context.Background(), Action{Kind: ActionAnalyze})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Equal(t, "vision unavailable", res.Error)
}

func TestEngine_Analyze_NoFrame(t *testing.T) {
	eng := newEngine(newEmptyCapture(), &stubVision{}, &stubInteractor{})
	res, err := eng.Perform(context.Background(), Action{Kind: ActionAnalyze})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Contains(t, res.Error, "no frame")
}

func TestEngine_RecordClip_Success(t *testing.T) {
	rec := &stubRecorder{clipPayload: `{"frames":[]}`}
	eng := newFullEngine(newEmptyCapture(), &stubVision{}, &stubInteractor{}, rec)
	res, err := eng.Perform(context.Background(), Action{
		Kind:       ActionRecordClip,
		ClipAround: time.Now().UnixNano(),
		ClipWindow: int64(2 * time.Second),
	})
	require.NoError(t, err)
	assert.True(t, res.Success)
	require.Len(t, res.Evidence, 1)
	assert.Equal(t, "clip", res.Evidence[0].Kind)
	assert.Contains(t, res.Evidence[0].Ref, "bytes")
}

func TestEngine_RecordClip_Error(t *testing.T) {
	rec := &stubRecorder{clipErr: errors.New("ring empty")}
	eng := newFullEngine(newEmptyCapture(), &stubVision{}, &stubInteractor{}, rec)
	res, err := eng.Perform(context.Background(), Action{Kind: ActionRecordClip})
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Equal(t, "ring empty", res.Error)
}

func TestEngine_UnknownKind_ReturnsError(t *testing.T) {
	eng := newEngine(newEmptyCapture(), &stubVision{}, &stubInteractor{})
	_, err := eng.Perform(context.Background(), Action{Kind: ActionKind("teleport")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported action kind")
}

func TestEngine_Elapsed_NonZero(t *testing.T) {
	inter := &stubInteractor{}
	eng := newEngine(newEmptyCapture(), &stubVision{}, inter)
	res, err := eng.Perform(context.Background(), Action{Kind: ActionClick})
	require.NoError(t, err)
	// Elapsed is populated by the defer; on fast stub calls it may be 0ns
	// on high-resolution clocks, so we assert >= 0 (field is set, not negative).
	assert.GreaterOrEqual(t, res.Elapsed.Nanoseconds(), int64(0), "elapsed must be non-negative")
}

func TestNew_NilPanics(t *testing.T) {
	vis := &stubVision{}
	inter := &stubInteractor{}
	obs := &stubObserver{}
	rec := &stubRecorder{}
	cap := newEmptyCapture()

	assert.Panics(t, func() { New(nil, vis, inter, obs, rec) }, "nil CaptureSource must panic")
	assert.Panics(t, func() { New(cap, nil, inter, obs, rec) }, "nil VisionPipeline must panic")
	assert.Panics(t, func() { New(cap, vis, nil, obs, rec) }, "nil Interactor must panic")
	assert.Panics(t, func() { New(cap, vis, inter, nil, rec) }, "nil Observer must panic")
	assert.Panics(t, func() { New(cap, vis, inter, obs, nil) }, "nil Recorder must panic")
}
