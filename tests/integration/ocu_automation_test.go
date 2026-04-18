//go:build integration
// +build integration

// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/nexus/automation"
	"digital.vasic.helixqa/pkg/nexus/automation/agent_bridge"
	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ── in-process stubs for integration tests ─────────────────────────────────

type autoIntFrameData struct{}

func (a *autoIntFrameData) AsBytes() ([]byte, error)                  { return []byte{0, 0, 0, 0}, nil }
func (a *autoIntFrameData) AsDMABuf() (*contracts.DMABufHandle, bool) { return nil, false }
func (a *autoIntFrameData) Release() error                            { return nil }

// autoIntCapture delivers one frame per Frames() call, resetting after each.
type autoIntCapture struct{ seq uint64 }

func (c *autoIntCapture) Name() string                                              { return "integration-auto" }
func (c *autoIntCapture) Start(_ context.Context, _ contracts.CaptureConfig) error { return nil }
func (c *autoIntCapture) Stop() error                                               { return nil }
func (c *autoIntCapture) Stats() contracts.CaptureStats                            { return contracts.CaptureStats{} }
func (c *autoIntCapture) Close() error                                              { return nil }
func (c *autoIntCapture) Frames() <-chan contracts.Frame {
	ch := make(chan contracts.Frame, 1)
	c.seq++
	ch <- contracts.Frame{
		Seq:       c.seq,
		Timestamp: time.Now(),
		Width:     1920,
		Height:    1080,
		Data:      &autoIntFrameData{},
	}
	return ch
}

type autoIntVision struct{}

func (v *autoIntVision) Analyze(_ context.Context, _ contracts.Frame) (*contracts.Analysis, error) {
	return &contracts.Analysis{DispatchedTo: "integration-cpu", Confidence: 0.95}, nil
}
func (v *autoIntVision) Match(_ context.Context, _ contracts.Frame, _ contracts.Template) ([]contracts.Match, error) {
	return nil, nil
}
func (v *autoIntVision) Diff(_ context.Context, _, _ contracts.Frame) (*contracts.DiffResult, error) {
	return &contracts.DiffResult{TotalDelta: 12.5, SameShape: true}, nil
}
func (v *autoIntVision) OCR(_ context.Context, _ contracts.Frame, _ contracts.Rect) (contracts.OCRResult, error) {
	return contracts.OCRResult{FullText: "integration"}, nil
}

type autoIntInteractor struct{ calls []string }

func (i *autoIntInteractor) Click(_ context.Context, _ contracts.Point, _ contracts.ClickOptions) error {
	i.calls = append(i.calls, "click")
	return nil
}
func (i *autoIntInteractor) Type(_ context.Context, _ string, _ contracts.TypeOptions) error {
	i.calls = append(i.calls, "type")
	return nil
}
func (i *autoIntInteractor) Scroll(_ context.Context, _ contracts.Point, _, _ float64) error {
	i.calls = append(i.calls, "scroll")
	return nil
}
func (i *autoIntInteractor) Key(_ context.Context, _ contracts.KeyCode, _ contracts.KeyOptions) error {
	i.calls = append(i.calls, "key")
	return nil
}
func (i *autoIntInteractor) Drag(_ context.Context, _, _ contracts.Point, _ contracts.DragOptions) error {
	i.calls = append(i.calls, "drag")
	return nil
}

type autoIntObserver struct{}

func (o *autoIntObserver) Start(_ context.Context, _ contracts.Target) error { return nil }
func (o *autoIntObserver) Events() <-chan contracts.Event {
	ch := make(chan contracts.Event)
	close(ch)
	return ch
}
func (o *autoIntObserver) Snapshot(_ time.Time, _ time.Duration) ([]contracts.Event, error) {
	return nil, nil
}
func (o *autoIntObserver) Stop() error { return nil }

type autoIntRecorder struct{ written int }

func (r *autoIntRecorder) AttachSource(_ contracts.CaptureSource) error            { return nil }
func (r *autoIntRecorder) Start(_ context.Context, _ contracts.RecordConfig) error { return nil }
func (r *autoIntRecorder) Clip(_ time.Time, _ time.Duration, out io.Writer, _ contracts.ClipOptions) error {
	payload := `{"frames":[{"seq":1}]}`
	n, err := io.WriteString(out, payload)
	r.written += n
	return err
}
func (r *autoIntRecorder) LiveStream(_ context.Context) (string, error) { return "", nil }
func (r *autoIntRecorder) Stop() error                                   { return nil }

// ── integration tests ──────────────────────────────────────────────────────

// TestOCU_Automation_SequenceCapture_Click_Analyze_RecordClip exercises the
// full Action sequence Capture → Click → Analyze → RecordClip through the
// Engine with all stub backends and verifies the Result shape for each step.
func TestOCU_Automation_SequenceCapture_Click_Analyze_RecordClip(t *testing.T) {
	cap := &autoIntCapture{}
	inter := &autoIntInteractor{}
	rec := &autoIntRecorder{}

	eng := automation.New(cap, &autoIntVision{}, inter, &autoIntObserver{}, rec)
	ctx := context.Background()

	// Step 1: Capture — should produce a screenshot_before EvidenceRef.
	r1, err := eng.Perform(ctx, automation.Action{Kind: automation.ActionCapture})
	require.NoError(t, err)
	assert.True(t, r1.Success, "Capture must succeed")
	require.Len(t, r1.Evidence, 1, "Capture must produce one EvidenceRef")
	assert.Equal(t, "screenshot_before", r1.Evidence[0].Kind)
	assert.Contains(t, r1.Evidence[0].Ref, "seq-")
	assert.Positive(t, r1.Elapsed)

	// Step 2: Click — should succeed and dispatch to Interactor.
	r2, err := eng.Perform(ctx, automation.Action{
		Kind: automation.ActionClick,
		At:   contracts.Point{X: 960, Y: 540},
	})
	require.NoError(t, err)
	assert.True(t, r2.Success, "Click must succeed")
	assert.Empty(t, r2.Error)
	assert.Contains(t, inter.calls, "click")

	// Step 3: Analyze — should return DispatchedTo from the vision backend.
	r3, err := eng.Perform(ctx, automation.Action{Kind: automation.ActionAnalyze})
	require.NoError(t, err)
	assert.True(t, r3.Success, "Analyze must succeed")
	assert.Equal(t, "integration-cpu", r3.DispatchedTo)

	// Step 4: RecordClip — should produce a clip EvidenceRef.
	r4, err := eng.Perform(ctx, automation.Action{
		Kind:       automation.ActionRecordClip,
		ClipAround: time.Now().UnixNano(),
		ClipWindow: int64(2 * time.Second),
	})
	require.NoError(t, err)
	assert.True(t, r4.Success, "RecordClip must succeed")
	require.Len(t, r4.Evidence, 1, "RecordClip must produce one EvidenceRef")
	assert.Equal(t, "clip", r4.Evidence[0].Kind)
	assert.Contains(t, r4.Evidence[0].Ref, "bytes")
	assert.Positive(t, rec.written, "Recorder must have written clip data")
}

// TestOCU_Automation_Bridge_FullSequence runs the same sequence through the
// Bridge adapter to verify end-to-end wiring from bridge to sub-engines.
func TestOCU_Automation_Bridge_FullSequence(t *testing.T) {
	cap := &autoIntCapture{}
	inter := &autoIntInteractor{}
	rec := &autoIntRecorder{}

	eng := automation.New(cap, &autoIntVision{}, inter, &autoIntObserver{}, rec)
	b := agent_bridge.NewBridge(eng)
	ctx := context.Background()

	actions := []automation.Action{
		{Kind: automation.ActionClick, At: contracts.Point{X: 100, Y: 200}},
		{Kind: automation.ActionType, Text: "integration test"},
		{Kind: automation.ActionKey, Key: contracts.KeyEnter},
		{Kind: automation.ActionScroll, At: contracts.Point{X: 50, Y: 50}, DY: 2},
		{Kind: automation.ActionDrag, At: contracts.Point{X: 0, Y: 0}, To: contracts.Point{X: 200, Y: 200}},
	}

	for _, a := range actions {
		res, err := b.ExecuteAction(ctx, a)
		require.NoError(t, err, "Bridge.ExecuteAction must not error for kind %q", a.Kind)
		assert.True(t, res.Success, "action %q must succeed", a.Kind)
		assert.Positive(t, res.Elapsed, "Elapsed must be non-zero for kind %q", a.Kind)
	}

	// Verify interactor received all five calls in order.
	assert.Equal(t, []string{"click", "type", "key", "scroll", "drag"}, inter.calls)
}

// TestOCU_Automation_AllElapsed_NonZero verifies that every ActionKind
// populates Result.Elapsed with a positive duration.
func TestOCU_Automation_AllElapsed_NonZero(t *testing.T) {
	cap := &autoIntCapture{}
	eng := automation.New(cap, &autoIntVision{}, &autoIntInteractor{}, &autoIntObserver{}, &autoIntRecorder{})
	ctx := context.Background()

	cases := []automation.Action{
		{Kind: automation.ActionClick},
		{Kind: automation.ActionType, Text: "x"},
		{Kind: automation.ActionScroll},
		{Kind: automation.ActionKey, Key: contracts.KeyTab},
		{Kind: automation.ActionDrag},
		{Kind: automation.ActionRecordClip, ClipAround: time.Now().UnixNano(), ClipWindow: int64(time.Second)},
	}

	for _, a := range cases {
		res, err := eng.Perform(ctx, a)
		require.NoError(t, err, "kind %q must not return error", a.Kind)
		assert.Positive(t, res.Elapsed, "kind %q: Elapsed must be positive", a.Kind)
	}

	// Capture and Analyze use fresh channels — test separately.
	r, err := eng.Perform(ctx, automation.Action{Kind: automation.ActionCapture})
	require.NoError(t, err)
	assert.Positive(t, r.Elapsed)

	r, err = eng.Perform(ctx, automation.Action{Kind: automation.ActionAnalyze})
	require.NoError(t, err)
	assert.Positive(t, r.Elapsed)
}
