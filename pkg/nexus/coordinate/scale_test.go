// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package coordinate

import (
	"errors"
	"testing"
)

func TestScaleCoordinates_NormalizedToRealScreen(t *testing.T) {
	cfg := Config{Screen: ScalingTarget{Width: 1920, Height: 1080}}
	x, y, err := ScaleCoordinates(cfg, ScaleAPIToScreen, 0.5, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if x != 960 || y != 540 {
		t.Errorf("normalised centre → (%d, %d), want (960, 540)", x, y)
	}
}

func TestScaleCoordinates_NormalizedOutOfBoundsFloat(t *testing.T) {
	cfg := Config{Screen: ScalingTarget{Width: 100, Height: 100}}
	_, _, err := ScaleCoordinates(cfg, ScaleAPIToScreen, 1.0, 1.0)
	if err == nil || !errors.Is(err, ErrOutOfBounds) {
		t.Errorf("normalised (1.0, 1.0) should overflow 100x100, got %v", err)
	}
}

func TestScaleCoordinates_APIToScreenDefaultTargets(t *testing.T) {
	// Screen is 1920x1080 (16:9). Canonical 1280x720 target matches.
	// API coord (640, 360) should map to (960, 540).
	cfg := Config{Screen: ScalingTarget{Width: 1920, Height: 1080}}
	x, y, err := ScaleCoordinates(cfg, ScaleAPIToScreen, 640, 360)
	if err != nil {
		t.Fatal(err)
	}
	if x != 960 || y != 540 {
		t.Errorf("API→screen: got (%d, %d), want (960, 540)", x, y)
	}
}

func TestScaleCoordinates_ScreenToAPIRoundTrip(t *testing.T) {
	cfg := Config{Screen: ScalingTarget{Width: 1920, Height: 1080}}
	// (960, 540) real → (640, 360) in the matched 1280x720 canonical.
	x, y, err := ScaleCoordinates(cfg, ScaleScreenToAPI, 960, 540)
	if err != nil {
		t.Fatal(err)
	}
	if x != 640 || y != 360 {
		t.Errorf("screen→API: got (%d, %d), want (640, 360)", x, y)
	}
}

func TestScaleCoordinates_NoAspectMatchClampsInBounds(t *testing.T) {
	// Portrait screen: no default 4:3 / 16:9 target matches.
	cfg := Config{Screen: ScalingTarget{Width: 360, Height: 640}, AspectTolerance: 0.01}
	x, y, err := ScaleCoordinates(cfg, ScaleAPIToScreen, 100, 200)
	if err != nil {
		t.Fatal(err)
	}
	if x != 100 || y != 200 {
		t.Errorf("no-match path should pass through, got (%d, %d)", x, y)
	}
	_, _, err = ScaleCoordinates(cfg, ScaleAPIToScreen, 400, 700)
	if err == nil {
		t.Error("out-of-bound coord must error even on no-match path")
	}
}

func TestScaleCoordinates_RejectsZeroScreen(t *testing.T) {
	_, _, err := ScaleCoordinates(Config{}, ScaleAPIToScreen, 10, 10)
	if err == nil {
		t.Error("zero-screen config must error")
	}
}

func TestNormalizedToScreen_Convenience(t *testing.T) {
	x, y, err := NormalizedToScreen(ScalingTarget{Width: 2560, Height: 1440}, 0.25, 0.75)
	if err != nil {
		t.Fatal(err)
	}
	if x != 640 || y != 1080 {
		t.Errorf("NormalizedToScreen got (%d, %d), want (640, 1080)", x, y)
	}
}

func TestPickTargetByAspect_SelectsClosest(t *testing.T) {
	screen := ScalingTarget{Width: 1920, Height: 1200}
	t2, ok := pickTargetByAspect(screen, MaxScalingTargets, 0.05)
	if !ok {
		t.Fatal("expected match under 5% tolerance")
	}
	// 1920x1200 is 16:10 → WXGA 1280x800.
	if t2.Width != 1280 || t2.Height != 800 {
		t.Errorf("closest 16:10 target = %+v, want 1280x800", t2)
	}
}

func FuzzScaleCoordinates(f *testing.F) {
	f.Add(0.5, 0.5, 1920, 1080)
	f.Add(640.0, 360.0, 1280, 720)
	f.Add(-1.0, -1.0, 100, 100)
	f.Add(1e9, 1e9, 100, 100)
	f.Fuzz(func(t *testing.T, x, y float64, w, h int) {
		if w <= 0 || h <= 0 {
			return
		}
		cfg := Config{Screen: ScalingTarget{Width: w, Height: h}}
		// Must never panic.
		_, _, _ = ScaleCoordinates(cfg, ScaleAPIToScreen, x, y)
		_, _, _ = ScaleCoordinates(cfg, ScaleScreenToAPI, x, y)
	})
}

func BenchmarkScaleCoordinates_NormalizedPath(b *testing.B) {
	cfg := Config{Screen: ScalingTarget{Width: 1920, Height: 1080}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ScaleCoordinates(cfg, ScaleAPIToScreen, 0.5, 0.5)
	}
}

func BenchmarkScaleCoordinates_AspectMatch(b *testing.B) {
	cfg := Config{Screen: ScalingTarget{Width: 1920, Height: 1080}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = ScaleCoordinates(cfg, ScaleAPIToScreen, 640, 360)
	}
}
