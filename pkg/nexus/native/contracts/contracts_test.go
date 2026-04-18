// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package contracts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── A1: Capture ──────────────────────────────────────────────────────────────

func TestCaptureConfig_ZeroValue(t *testing.T) {
	var cfg CaptureConfig
	assert.Equal(t, 0, cfg.FrameRate)
	assert.Equal(t, 0, cfg.Width)
	assert.Equal(t, 0, cfg.Height)
}

func TestFrame_FieldsAccessible(t *testing.T) {
	now := time.Now()
	f := Frame{
		Seq:       42,
		Timestamp: now,
		Width:     1920,
		Height:    1080,
		Stride:    7680,
		Format:    PixelFormatBGRA8,
		Metadata:  map[string]string{"window": "chromium"},
	}
	assert.Equal(t, uint64(42), f.Seq)
	assert.Equal(t, now, f.Timestamp)
	assert.Equal(t, 1920, f.Width)
	assert.Equal(t, "chromium", f.Metadata["window"])
}

func TestPixelFormat_Known(t *testing.T) {
	assert.NotEmpty(t, PixelFormatBGRA8)
	assert.NotEmpty(t, PixelFormatNV12)
	assert.NotEmpty(t, PixelFormatI420)
	assert.NotEmpty(t, PixelFormatH264)
}

// ── A2: Vision ───────────────────────────────────────────────────────────────

func TestAnalysis_FieldsAccessible(t *testing.T) {
	a := Analysis{
		Elements: []UIElement{
			{Kind: "button", Rect: Rect{X: 0, Y: 0, W: 100, H: 30}},
		},
		TextRegions: []OCRBlock{
			{Text: "Login", Rect: Rect{X: 10, Y: 5, W: 40, H: 12}},
		},
		DetectedChanges: []ChangeRegion{
			{Rect: Rect{X: 0, Y: 0, W: 10, H: 10}},
		},
		Confidence:   0.92,
		DispatchedTo: "thinker-cuda",
		LatencyMs:    12,
	}
	require.Len(t, a.Elements, 1)
	assert.Equal(t, "button", a.Elements[0].Kind)
	assert.Equal(t, Rect{X: 0, Y: 0, W: 100, H: 30}, a.Elements[0].Rect)
	require.Len(t, a.TextRegions, 1)
	assert.Equal(t, "Login", a.TextRegions[0].Text)
	require.Len(t, a.DetectedChanges, 1)
	assert.Equal(t, 0.92, a.Confidence)
	assert.Equal(t, "thinker-cuda", a.DispatchedTo)
	assert.Equal(t, 12, a.LatencyMs)
}

func TestRect_Zero(t *testing.T) {
	var r Rect
	assert.Equal(t, 0, r.X)
	assert.Equal(t, 0, r.Y)
	assert.Equal(t, 0, r.W)
	assert.Equal(t, 0, r.H)
}

func TestTemplate_HasBytes(t *testing.T) {
	tmpl := Template{
		Name:  "play-button",
		Bytes: []byte{0x00, 0x01},
	}
	assert.Equal(t, "play-button", tmpl.Name)
	assert.Len(t, tmpl.Bytes, 2)
}

// ── A3: Interact ─────────────────────────────────────────────────────────────

func TestPoint_Arithmetic(t *testing.T) {
	p := Point{X: 10, Y: 20}
	got := p.Translate(5, -2)
	assert.Equal(t, Point{X: 15, Y: 18}, got)
}

func TestClickOptions_DefaultsZero(t *testing.T) {
	var o ClickOptions
	assert.Equal(t, ClickLeft, o.Button)
	assert.Equal(t, 0, o.Clicks)
}

func TestKeyCode_Known(t *testing.T) {
	assert.NotEmpty(t, string(KeyEnter))
	assert.NotEmpty(t, string(KeyEscape))
	assert.NotEmpty(t, string(KeyTab))
}

// ── A4: Observe ──────────────────────────────────────────────────────────────

func TestEvent_Kinds(t *testing.T) {
	assert.NotEmpty(t, string(EventKindSyscall))
	assert.NotEmpty(t, string(EventKindDBus))
	assert.NotEmpty(t, string(EventKindCDP))
	assert.NotEmpty(t, string(EventKindAXTree))
}

func TestTarget_ZeroValid(t *testing.T) {
	var tgt Target
	assert.Empty(t, tgt.ProcessName)
	assert.Equal(t, 0, tgt.PID)
}

// ── A5: Record ───────────────────────────────────────────────────────────────

func TestRecordConfig_Defaults(t *testing.T) {
	var cfg RecordConfig
	assert.Equal(t, 0, cfg.FrameRate)
	assert.Equal(t, 0, cfg.BitrateKbps)
	assert.Equal(t, time.Duration(0), cfg.SegmentLength)
}

func TestClipOptions_BurntInDefaults(t *testing.T) {
	var o ClipOptions
	assert.False(t, o.BurntInTimestamp)
	assert.False(t, o.BurntInActionArrow)
}

// ── A6: Remote ───────────────────────────────────────────────────────────────

func TestCapabilityKind_Known(t *testing.T) {
	assert.NotEmpty(t, string(KindCUDAOpenCV))
	assert.NotEmpty(t, string(KindNVENC))
	assert.NotEmpty(t, string(KindTensorRTOCR))
}

func TestCapability_ZeroValue(t *testing.T) {
	var c Capability
	assert.Empty(t, string(c.Kind))
	assert.Equal(t, 0, c.MinVRAM)
	assert.False(t, c.PreferLocal)
}

// ── Spec-review gap fill ──────────────────────────────────────────────────────

func TestCaptureStats_ZeroValue(t *testing.T) {
	var s CaptureStats
	require.Zero(t, s.FramesProduced)
	require.Zero(t, s.FramesDropped)
	require.True(t, s.LastFrameAt.IsZero())
	require.Zero(t, s.AverageLatency)
}

func TestDMABufHandle_FieldsAccessible(t *testing.T) {
	h := DMABufHandle{FD: 7, Width: 1920, Height: 1080, Stride: 7680, Modifier: 0x100}
	require.Equal(t, 7, h.FD)
	require.Equal(t, 1920, h.Width)
	require.Equal(t, uint64(0x100), h.Modifier)
}

func TestOCRResult_FieldsAccessible(t *testing.T) {
	r := OCRResult{
		Blocks:   []OCRBlock{{Text: "hi", Rect: Rect{1, 2, 3, 4}}},
		FullText: "hi",
	}
	require.Len(t, r.Blocks, 1)
	require.Equal(t, "hi", r.FullText)
}

func TestDiffResult_FieldsAccessible(t *testing.T) {
	d := DiffResult{
		Regions:    []ChangeRegion{{Rect: Rect{W: 10, H: 10}, Magnitude: 0.5, PixelCount: 100}},
		TotalDelta: 0.5,
		SameShape:  true,
	}
	require.Len(t, d.Regions, 1)
	require.Equal(t, 0.5, d.TotalDelta)
	require.True(t, d.SameShape)
}

func TestMatch_FieldsAccessible(t *testing.T) {
	m := Match{Rect: Rect{X: 10, Y: 20, W: 30, H: 40}, Confidence: 0.88}
	require.Equal(t, 10, m.Rect.X)
	require.Equal(t, 0.88, m.Confidence)
}

func TestTypeOptions_ZeroValue(t *testing.T) {
	var o TypeOptions
	require.Zero(t, o.DelayPerChar)
	require.False(t, o.ClearFirst)
}

func TestKeyOptions_ZeroValue(t *testing.T) {
	var o KeyOptions
	require.Empty(t, o.Modifiers)
	require.Zero(t, o.HoldFor)
}

func TestDragOptions_ZeroValue(t *testing.T) {
	var o DragOptions
	require.Equal(t, ClickLeft, o.Button)
	require.Zero(t, o.Steps)
	require.Zero(t, o.Duration)
	require.Empty(t, o.Modifiers)
}

func TestEventKind_HookPresent(t *testing.T) {
	require.NotEmpty(t, string(EventKindHook))
}

func TestClipOptions_AnchorPointAndAnnotation(t *testing.T) {
	o := ClipOptions{
		BurntInActionArrow: true,
		AnchorPoint:        Point{X: 100, Y: 200},
		Annotation:         "click here",
	}
	require.Equal(t, 100, o.AnchorPoint.X)
	require.Equal(t, "click here", o.Annotation)
}

// Compile-time guards: ensure the interface identifiers remain
// reachable from this package. Full `var _ Iface = concreteImpl{}`
// guards against signature drift will be added in P1–P5 where real
// implementations land; for P0 these blank declarations are enough
// to fail the build if any interface is renamed or removed.
var (
	_ = (*Interactor)(nil)
	_ = (*CaptureSource)(nil)
	_ = (*VisionPipeline)(nil)
	_ = (*Observer)(nil)
	_ = (*Recorder)(nil)
	_ = (*Worker)(nil)
	_ = (*Dispatcher)(nil)
	_ = (*FrameData)(nil)
)
