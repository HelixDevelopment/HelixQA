//go:build vision
// +build vision

package detection

import (
	"context"
	"image"
	"testing"

	"digital.vasic.helixqa/pkg/vision/core"
)

// bgrCheckerboard builds raw 3-channel BGR bytes for a w x h checkerboard with
// `cell`-sized squares. Strong corners/edges => ORB finds many keypoints.
func bgrCheckerboard(w, h, cell int) []byte {
	buf := make([]byte, w*h*3)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			on := ((x/cell)+(y/cell))%2 == 0
			var v byte
			if on {
				v = 255
			}
			i := (y*w + x) * 3
			buf[i], buf[i+1], buf[i+2] = v, v, v
		}
	}
	return buf
}

// bgrBlank builds a uniform gray image with no texture => no ORB keypoints.
func bgrBlank(w, h int) []byte {
	buf := make([]byte, w*h*3)
	for i := range buf {
		buf[i] = 128
	}
	return buf
}

func frameFrom(data []byte, w, h int) *core.Frame {
	return &core.Frame{
		Data:   data,
		Bounds: image.Rect(0, 0, w, h),
	}
}

func newDetector(t *testing.T) *ORBDetector {
	t.Helper()
	d, err := NewORBDetector(DefaultORBConfig())
	if err != nil {
		t.Fatalf("NewORBDetector: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestDefaultORBConfig(t *testing.T) {
	c := DefaultORBConfig()
	if c.MaxFeatures != 500 {
		t.Errorf("MaxFeatures = %d, want 500", c.MaxFeatures)
	}
	if c.ScaleFactor != 1.2 {
		t.Errorf("ScaleFactor = %f, want 1.2", c.ScaleFactor)
	}
	if c.MinMatches != 10 {
		t.Errorf("MinMatches = %d, want 10", c.MinMatches)
	}
	if c.MatchRatio != 0.75 {
		t.Errorf("MatchRatio = %f, want 0.75", c.MatchRatio)
	}
}

// TestORBDetect_TexturedYieldsKeypoints proves the detector actually finds
// features on a textured image (not just that it returns without panic).
func TestORBDetect_TexturedYieldsKeypoints(t *testing.T) {
	d := newDetector(t)
	w, h := 200, 200
	frame := frameFrom(bgrCheckerboard(w, h, 20), w, h)

	elems, err := d.Detect(context.Background(), frame)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(elems) == 0 {
		t.Fatal("expected >0 keypoints on a textured checkerboard, got 0")
	}
	// Each returned element must be tagged as coming from ORB and carry a bounds.
	for _, e := range elems {
		if e.Source != "orb" {
			t.Errorf("element source = %q, want orb", e.Source)
		}
		if e.Bounds.Dx() <= 0 && e.Bounds.Dy() <= 0 {
			// keypoint size can be small but the bounds should be derived from it
			t.Errorf("element %s has degenerate bounds %+v", e.ID, e.Bounds.Rectangle)
		}
	}
}

// TestORBDetect_BlankYieldsFewer proves the count is input-dependent: a flat
// image yields strictly fewer keypoints than a textured one (typically zero).
func TestORBDetect_BlankYieldsFewer(t *testing.T) {
	d := newDetector(t)
	w, h := 200, 200

	textured, err := d.Detect(context.Background(), frameFrom(bgrCheckerboard(w, h, 20), w, h))
	if err != nil {
		t.Fatalf("Detect(textured): %v", err)
	}
	blank, err := d.Detect(context.Background(), frameFrom(bgrBlank(w, h), w, h))
	if err != nil {
		t.Fatalf("Detect(blank): %v", err)
	}
	if len(blank) >= len(textured) {
		t.Fatalf("blank keypoints (%d) should be fewer than textured (%d)", len(blank), len(textured))
	}
}

// TestORBDetect_Deterministic proves the same input yields the same keypoint
// count across repeated runs (ORB is deterministic for a fixed image).
func TestORBDetect_Deterministic(t *testing.T) {
	d := newDetector(t)
	w, h := 160, 160
	data := bgrCheckerboard(w, h, 16)

	first, err := d.Detect(context.Background(), frameFrom(data, w, h))
	if err != nil {
		t.Fatalf("Detect run 1: %v", err)
	}
	for run := 2; run <= 3; run++ {
		again, err := d.Detect(context.Background(), frameFrom(data, w, h))
		if err != nil {
			t.Fatalf("Detect run %d: %v", run, err)
		}
		if len(again) != len(first) {
			t.Fatalf("run %d keypoint count = %d, want %d (non-deterministic)", run, len(again), len(first))
		}
	}
}

func TestORBDetect_UnsupportedOps(t *testing.T) {
	d := newDetector(t)
	w, h := 64, 64
	frame := frameFrom(bgrBlank(w, h), w, h)

	if _, err := d.DetectType(context.Background(), frame, core.ElementButton); err == nil {
		t.Error("DetectType should return an error (ORB has no type classification)")
	}
	if _, err := d.FindByText(context.Background(), frame, "ok"); err == nil {
		t.Error("FindByText should return an error (ORB has no OCR)")
	}
}

func TestORBFindByRegisteredTemplate_NotRegistered(t *testing.T) {
	d := newDetector(t)
	w, h := 64, 64
	frame := frameFrom(bgrBlank(w, h), w, h)

	_, err := d.FindByRegisteredTemplate(context.Background(), frame, "missing", 0.5)
	if err == nil {
		t.Error("expected error for unregistered template name")
	}
}

func TestORBRegisterTemplate(t *testing.T) {
	d := newDetector(t)
	w, h := 120, 120
	tmpl := bgrCheckerboard(w, h, 12)

	if err := d.RegisterTemplate("tile", tmpl); err != nil {
		t.Fatalf("RegisterTemplate: %v", err)
	}
	// Registering again under the same name must not leak/panic (replaces).
	if err := d.RegisterTemplate("tile", tmpl); err != nil {
		t.Fatalf("RegisterTemplate (replace): %v", err)
	}
}

func TestBoundingBoxFromPoints(t *testing.T) {
	pts := []image.Point{{X: 5, Y: 8}, {X: 1, Y: 20}, {X: 30, Y: 2}, {X: 10, Y: 10}}
	box := boundingBoxFromPoints(pts)
	if box.Min.X != 1 || box.Min.Y != 2 || box.Max.X != 30 || box.Max.Y != 20 {
		t.Errorf("bbox = %+v, want (1,2)-(30,20)", box.Rectangle)
	}
	// Empty input returns a zero rectangle.
	empty := boundingBoxFromPoints(nil)
	if empty.Min.X != 0 || empty.Max.X != 0 {
		t.Errorf("empty bbox should be zero, got %+v", empty.Rectangle)
	}
}
