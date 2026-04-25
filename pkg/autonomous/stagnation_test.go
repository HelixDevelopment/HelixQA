// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"image"
	"image/color"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Legacy byte-hash path — StagnationDetector.AddScreenshot + IsStagnant.
// ---------------------------------------------------------------------------

func TestStagnationDetector_EmptyState(t *testing.T) {
	sd := NewStagnationDetector()
	if sd.IsStagnant() {
		t.Fatal("empty detector should not be stagnant")
	}
	if sd.GetStagnantDuration() != 0 {
		t.Fatal("empty detector should have zero stagnant duration")
	}
	if sd.GetCurrentScreen() != "unknown" {
		t.Fatalf("empty detector current screen = %q, want 'unknown'", sd.GetCurrentScreen())
	}
	if sd.LastChangeProbability() != 0 {
		t.Fatalf("empty detector cp = %v, want 0", sd.LastChangeProbability())
	}
	if sd.IsChangePoint() {
		t.Fatal("empty detector should not report change point")
	}
}

func TestStagnationDetector_AddScreenshot_EmptyIgnored(t *testing.T) {
	sd := NewStagnationDetector()
	sd.AddScreenshot(nil, "home")
	sd.AddScreenshot([]byte{}, "home")
	if sd.GetCurrentScreen() != "unknown" {
		t.Fatal("empty screenshots must not be recorded")
	}
}

func TestStagnationDetector_AddScreenshot_BoundedHistory(t *testing.T) {
	sd := NewStagnationDetector()
	for i := 0; i < 50; i++ {
		data := []byte{byte(i), 0xFF, 0xAA, 0x55}
		sd.AddScreenshot(data, "home")
	}
	if len(sd.history) != 30 {
		t.Fatalf("history len = %d, want 30 (maxHistory bound)", len(sd.history))
	}
}

func TestStagnationDetector_StagnantAfterIdenticalBurst(t *testing.T) {
	sd := NewStagnationDetector()
	// Shrink the stagnation window for test speed.
	sd.stagnantTime = 5 * time.Millisecond
	data := []byte{1, 2, 3, 4}
	for i := 0; i < 5; i++ {
		sd.AddScreenshot(data, "home")
	}
	time.Sleep(10 * time.Millisecond)
	// Add 3 more IDENTICAL inside the new window.
	for i := 0; i < 3; i++ {
		sd.AddScreenshot(data, "home")
	}
	if !sd.IsStagnant() {
		t.Fatal("identical burst should mark detector stagnant")
	}
	if sd.GetStagnantDuration() <= 0 {
		t.Fatal("stagnant duration should be positive")
	}
	if got := sd.GetCurrentScreen(); got != "home" {
		t.Fatalf("current screen = %q, want 'home'", got)
	}
}

func TestStagnationDetector_NotStagnantWhenDataChanges(t *testing.T) {
	sd := NewStagnationDetector()
	sd.stagnantTime = 5 * time.Millisecond
	sd.AddScreenshot([]byte{1, 2, 3}, "home")
	sd.AddScreenshot([]byte{1, 2, 3}, "home")
	time.Sleep(10 * time.Millisecond)
	sd.AddScreenshot([]byte{1, 2, 3}, "home")
	sd.AddScreenshot([]byte{9, 9, 9}, "home") // different
	sd.AddScreenshot([]byte{1, 2, 3}, "home")
	if sd.IsStagnant() {
		t.Fatal("changing data should not be stagnant")
	}
}

func TestStagnationDetector_NotStagnantBelowMinSnapshots(t *testing.T) {
	sd := NewStagnationDetector()
	sd.AddScreenshot([]byte{1}, "home")
	sd.AddScreenshot([]byte{1}, "home")
	if sd.IsStagnant() {
		t.Fatal("fewer than 3 snapshots must not be stagnant")
	}
}

func TestStagnationDetector_Reset(t *testing.T) {
	sd := NewStagnationDetector()
	sd.AddScreenshot([]byte{1, 2, 3}, "home")
	sd.Reset()
	if len(sd.history) != 0 {
		t.Fatal("Reset should clear history")
	}
	if sd.GetCurrentScreen() != "unknown" {
		t.Fatal("Reset should restore unknown screen")
	}
}

func TestStagnationDetector_ComputeHash_EmptyAndShortData(t *testing.T) {
	sd := NewStagnationDetector()
	if got := sd.computeHash(nil); got != 0 {
		t.Fatalf("computeHash(nil) = %d, want 0", got)
	}
	if got := sd.computeHash([]byte{}); got != 0 {
		t.Fatalf("computeHash(empty) = %d, want 0", got)
	}
	// Short data still hashes (sampling just returns repeated indices).
	if got := sd.computeHash([]byte{1, 2}); got == 0 {
		t.Error("computeHash(short) should be non-zero")
	}
}

func TestStagnationDetector_StagnantDuration_NoHistoryOrShortWindow(t *testing.T) {
	sd := NewStagnationDetector()
	// Not stagnant → duration 0.
	if d := sd.GetStagnantDuration(); d != 0 {
		t.Fatalf("empty stagnant duration = %v, want 0", d)
	}
}

func TestStagnationDetector_NotStagnantWhenRecentWindowTooSmall(t *testing.T) {
	sd := NewStagnationDetector()
	sd.stagnantTime = 1 * time.Millisecond
	// Add 4 old entries then sleep past the window — recent window
	// will have 0 or 1 snapshots, not the ≥3 needed for stagnation.
	for i := 0; i < 4; i++ {
		sd.AddScreenshot([]byte{1, 2, 3}, "home")
	}
	time.Sleep(5 * time.Millisecond)
	if sd.IsStagnant() {
		t.Fatal("window with < 3 recent snapshots must not be stagnant")
	}
}

func TestStagnationDetector_StagnantDuration_AfterHashChange(t *testing.T) {
	// Directly seed history so the test is deterministic — the timing-
	// dependent AddScreenshot path has its own coverage elsewhere.
	now := time.Now()
	sd := NewStagnationDetector()
	sd.stagnantTime = 300 * time.Millisecond

	// Three old "other-hash" snapshots (outside the stagnation window),
	// then three recent "stagnant-hash" snapshots (inside). IsStagnant
	// checks only the inside set, which is uniform → true. Then
	// GetStagnantDuration walks back through ALL history and finds the
	// hash change in the middle, returning the duration since the
	// transition.
	sd.history = []screenSnapshot{
		{timestamp: now.Add(-800 * time.Millisecond), hash: 0xAA, size: 1, screenName: "boot"},
		{timestamp: now.Add(-750 * time.Millisecond), hash: 0xAA, size: 1, screenName: "boot"},
		{timestamp: now.Add(-700 * time.Millisecond), hash: 0xAA, size: 1, screenName: "boot"},
		{timestamp: now.Add(-200 * time.Millisecond), hash: 0xBB, size: 1, screenName: "home"},
		{timestamp: now.Add(-100 * time.Millisecond), hash: 0xBB, size: 1, screenName: "home"},
		{timestamp: now.Add(-50 * time.Millisecond), hash: 0xBB, size: 1, screenName: "home"},
	}
	if !sd.IsStagnant() {
		t.Fatalf("seeded history should be stagnant: %+v", sd.history)
	}
	d := sd.GetStagnantDuration()
	if d <= 0 || d > 500*time.Millisecond {
		t.Fatalf("stagnant duration = %v, want in (0, 500ms]", d)
	}
}

// ---------------------------------------------------------------------------
// BOCPD + dHash path — StagnationDetector.AddFrame + LastChangeProbability.
// ---------------------------------------------------------------------------

// gradientRGBA_Stag generates a deterministic image for the stagnation tests —
// independent from the one in pkg/vision/hash to keep the test self-contained.
func gradientRGBA_Stag(w, h, seed int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x + seed) & 0xFF),
				G: uint8((y * 3) & 0xFF),
				B: uint8(((x + y) ^ seed) & 0xFF),
				A: 255,
			})
		}
	}
	return img
}

func TestStagnationDetector_AddFrame_FirstFrameReturnsZero(t *testing.T) {
	sd := NewStagnationDetector()
	cp, err := sd.AddFrame(gradientRGBA_Stag(128, 96, 0), "home")
	if err != nil {
		t.Fatalf("AddFrame: %v", err)
	}
	if cp != 0 {
		t.Fatalf("first frame cp = %v, want 0 (no prior frame to diff against)", cp)
	}
	if sd.LastChangeProbability() != 0 {
		t.Fatalf("LastChangeProbability = %v, want 0", sd.LastChangeProbability())
	}
}

func TestStagnationDetector_AddFrame_IdenticalFramesNoChangePoint(t *testing.T) {
	sd := NewStagnationDetector()
	img := gradientRGBA_Stag(256, 192, 0)
	for i := 0; i < 30; i++ {
		if _, err := sd.AddFrame(img, "home"); err != nil {
			t.Fatalf("frame %d: %v", i, err)
		}
	}
	// Identical frames → distance 0 → BOCPD concentrates at long run
	// length → P(r ≤ threshold) stays low after burn-in.
	if sd.IsChangePoint() {
		t.Fatalf("identical-frame stream reported change point; last cp = %v", sd.LastChangeProbability())
	}
}

func TestStagnationDetector_AddFrame_SceneChangeTriggersChangePoint(t *testing.T) {
	sd := NewStagnationDetector()
	// Stable phase: identical frames for burn-in.
	stable := gradientRGBA_Stag(256, 192, 0)
	for i := 0; i < 30; i++ {
		sd.AddFrame(stable, "home")
	}
	preCP := sd.LastChangeProbability()
	if preCP > 0.5 {
		t.Fatalf("stable phase ended with cp = %v (want < 0.5)", preCP)
	}

	// Scene change: completely different image.
	different := gradientRGBA_Stag(256, 192, 127)
	var sawCP bool
	for i := 0; i < 10; i++ {
		cp, err := sd.AddFrame(different, "details")
		if err != nil {
			t.Fatalf("change frame %d: %v", i, err)
		}
		if cp > 0.5 {
			sawCP = true
			break
		}
	}
	if !sawCP {
		t.Fatal("scene change did not produce cp > 0.5 within 10 frames")
	}
}

func TestStagnationDetector_AddFrame_NilImageError(t *testing.T) {
	sd := NewStagnationDetector()
	if _, err := sd.AddFrame(nil, "home"); err == nil {
		t.Fatal("AddFrame(nil) must return error")
	}
}

func TestStagnationDetector_BOCPD_Exposed(t *testing.T) {
	sd := NewStagnationDetector()
	if sd.BOCPD() == nil {
		t.Fatal("BOCPD() must return non-nil after NewStagnationDetector")
	}
}
