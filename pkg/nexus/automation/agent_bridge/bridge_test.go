// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package agent_bridge

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/nexus/automation"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ── minimal stubs so Bridge tests have no dependency on real backends ──────

type stubFrameData struct{}

func (s *stubFrameData) AsBytes() ([]byte, error)                  { return []byte{0}, nil }
func (s *stubFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (s *stubFrameData) Release() error                            { return nil }

type stubCapture struct{ ch chan contracts.Frame }

func newStubCapture() *stubCapture {
	ch := make(chan contracts.Frame, 1)
	ch <- contracts.Frame{Seq: 1, Timestamp: time.Now(), Width: 1, Height: 1, Data: &stubFrameData{}}
	return &stubCapture{ch: ch}
}

func newEmptyCapture() *stubCapture { return &stubCapture{ch: make(chan contracts.Frame)} }

func (s *stubCapture) Name() string                                             { return "stub" }
func (s *stubCapture) Start(_ context.Context, _ contracts.CaptureConfig) error { return nil }
func (s *stubCapture) Stop() error                                              { return nil }
func (s *stubCapture) Frames() <-chan contracts.Frame                           { return s.ch }
func (s *stubCapture) Stats() contracts.CaptureStats                            { return contracts.CaptureStats{} }
func (s *stubCapture) Close() error                                             { return nil }

type stubVision struct{}

func (s *stubVision) Analyze(_ context.Context, _ contracts.Frame) (*contracts.Analysis, error) {
	return &contracts.Analysis{DispatchedTo: "stub-cpu"}, nil
}
func (s *stubVision) Match(_ context.Context, _ contracts.Frame, _ contracts.Template) ([]contracts.Match, error) {
	return nil, nil
}
func (s *stubVision) Diff(_ context.Context, _, _ contracts.Frame) (*contracts.DiffResult, error) {
	return &contracts.DiffResult{TotalDelta: 0, SameShape: true}, nil
}
func (s *stubVision) OCR(_ context.Context, _ contracts.Frame, _ contracts.Rect) (contracts.OCRResult, error) {
	return contracts.OCRResult{}, nil
}

type stubInteractor struct{}

func (s *stubInteractor) Click(_ context.Context, _ contracts.Point, _ contracts.ClickOptions) error {
	return nil
}
func (s *stubInteractor) Type(_ context.Context, _ string, _ contracts.TypeOptions) error {
	return nil
}
func (s *stubInteractor) Scroll(_ context.Context, _ contracts.Point, _, _ float64) error {
	return nil
}
func (s *stubInteractor) Key(_ context.Context, _ contracts.KeyCode, _ contracts.KeyOptions) error {
	return nil
}
func (s *stubInteractor) Drag(_ context.Context, _, _ contracts.Point, _ contracts.DragOptions) error {
	return nil
}

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

type stubRecorder struct{}

func (s *stubRecorder) AttachSource(_ contracts.CaptureSource) error            { return nil }
func (s *stubRecorder) Start(_ context.Context, _ contracts.RecordConfig) error { return nil }
func (s *stubRecorder) Clip(_ time.Time, _ time.Duration, out io.Writer, _ contracts.ClipOptions) error {
	_, err := io.WriteString(out, `{"frames":[]}`)
	return err
}
func (s *stubRecorder) LiveStream(_ context.Context) (string, error) { return "", nil }
func (s *stubRecorder) Stop() error                                  { return nil }

func newTestEngine() *automation.Engine {
	return automation.New(
		newEmptyCapture(),
		&stubVision{},
		&stubInteractor{},
		&stubObserver{},
		&stubRecorder{},
	)
}

// ── tests ──────────────────────────────────────────────────────────────────

// TestBridge_NilEngine_ReturnsError verifies that a Bridge with a nil Engine
// returns an error on ExecuteAction rather than panicking.
func TestBridge_NilEngine_ReturnsError(t *testing.T) {
	b := NewBridge(nil)
	_, err := b.ExecuteAction(context.Background(), automation.Action{Kind: automation.ActionClick})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Engine is nil")
}

// TestBridge_HappyPath_PassesThrough verifies that a successful Engine.Perform
// result is returned unchanged by the Bridge.
func TestBridge_HappyPath_PassesThrough(t *testing.T) {
	b := NewBridge(newTestEngine())
	res, err := b.ExecuteAction(context.Background(), automation.Action{Kind: automation.ActionClick})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.GreaterOrEqual(t, res.Elapsed.Nanoseconds(), int64(0))
}

// TestBridge_ErrorFromEngine_Surfaces verifies that an unsupported ActionKind
// causes Engine.Perform to return an error and Bridge surfaces it.
func TestBridge_ErrorFromEngine_Surfaces(t *testing.T) {
	b := NewBridge(newTestEngine())
	_, err := b.ExecuteAction(context.Background(), automation.Action{Kind: automation.ActionKind("teleport")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported action kind")
}

// TestBridge_AnalyzeAction_DispatchedTo verifies that DispatchedTo from the
// vision backend propagates through the Bridge into the caller's hands.
func TestBridge_AnalyzeAction_DispatchedTo(t *testing.T) {
	eng := automation.New(
		newStubCapture(),
		&stubVision{},
		&stubInteractor{},
		&stubObserver{},
		&stubRecorder{},
	)
	b := NewBridge(eng)
	res, err := b.ExecuteAction(context.Background(), automation.Action{Kind: automation.ActionAnalyze})
	require.NoError(t, err)
	assert.True(t, res.Success)
	assert.Equal(t, "stub-cpu", res.DispatchedTo)
}

// TestNewBridge_NilEngineConstruction verifies that NewBridge(nil) returns a
// non-nil Bridge (lazy nil check happens at ExecuteAction time).
