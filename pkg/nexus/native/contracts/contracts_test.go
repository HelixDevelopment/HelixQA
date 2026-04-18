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
