// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package desktop

import (
	"context"
	"strings"
	"testing"
)

// Phase-6 coord dispatch regression suite for LinuxEngine.

func TestLinuxEngine_CoordClick_RoutesMousemoveThenClick(t *testing.T) {
	r := &fakeRunner{}
	e := NewLinuxEngine("").WithCommandRunner(r.run)
	if err := e.CoordClick(context.Background(), 42, 87); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 2 {
		t.Fatalf("expected 2 xdotool calls (mousemove + click), got %d", len(r.calls))
	}
	if r.calls[0][0] != "xdotool" || r.calls[0][1] != "mousemove" ||
		r.calls[0][2] != "42" || r.calls[0][3] != "87" {
		t.Errorf("first call wrong: %v", r.calls[0])
	}
	if r.calls[1][0] != "xdotool" || r.calls[1][1] != "click" || r.calls[1][2] != "1" {
		t.Errorf("second call wrong: %v", r.calls[1])
	}
}

func TestLinuxEngine_CoordClick_RefusesOnWayland(t *testing.T) {
	r := &fakeRunner{}
	e := NewLinuxEngine("").AsWayland().WithCommandRunner(r.run)
	err := e.CoordClick(context.Background(), 10, 10)
	if err == nil || !strings.Contains(err.Error(), "Wayland") {
		t.Errorf("expected Wayland refusal, got %v", err)
	}
	if len(r.calls) != 0 {
		t.Error("Wayland refusal must not invoke xdotool")
	}
}

func TestLinuxEngine_CoordClick_RefusesNegativeCoords(t *testing.T) {
	r := &fakeRunner{}
	e := NewLinuxEngine("").WithCommandRunner(r.run)
	err := e.CoordClick(context.Background(), -1, 10)
	if err == nil {
		t.Fatal("expected negative-coord refusal")
	}
}

func TestLinuxEngine_CoordType_TapsThenTypes(t *testing.T) {
	r := &fakeRunner{}
	e := NewLinuxEngine("").WithCommandRunner(r.run)
	if err := e.CoordType(context.Background(), 100, 200, "hello"); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 3 {
		t.Fatalf("expected 3 xdotool calls (mousemove + click + type), got %d", len(r.calls))
	}
	last := r.calls[2]
	if last[0] != "xdotool" || last[1] != "type" || last[len(last)-1] != "hello" {
		t.Errorf("type call wrong: %v", last)
	}
}

func TestLinuxEngine_CoordType_RefusesEmptyText(t *testing.T) {
	e := NewLinuxEngine("").WithCommandRunner((&fakeRunner{}).run)
	err := e.CoordType(context.Background(), 1, 2, "")
	if err == nil {
		t.Fatal("expected empty-text refusal")
	}
}

func TestLinuxEngine_CoordScroll_FiresButton4On_NegativeDy(t *testing.T) {
	r := &fakeRunner{}
	e := NewLinuxEngine("").WithCommandRunner(r.run)
	// dy = -240 → 2 ticks of button 4 (up).
	if err := e.CoordScroll(context.Background(), 0, 0, 0, -240); err != nil {
		t.Fatal(err)
	}
	// mousemove + 2 clicks of button 4.
	if len(r.calls) != 3 {
		t.Fatalf("call count = %d, want 3", len(r.calls))
	}
	for i := 1; i < 3; i++ {
		if r.calls[i][0] != "xdotool" || r.calls[i][1] != "click" || r.calls[i][2] != "4" {
			t.Errorf("click %d wrong: %v", i, r.calls[i])
		}
	}
}

func TestLinuxEngine_CoordScroll_FiresButton5OnPositiveDy(t *testing.T) {
	r := &fakeRunner{}
	e := NewLinuxEngine("").WithCommandRunner(r.run)
	if err := e.CoordScroll(context.Background(), 50, 50, 0, 360); err != nil {
		t.Fatal(err)
	}
	// 360/120 = 3 ticks of button 5.
	if len(r.calls) != 4 {
		t.Fatalf("call count = %d, want 4 (mousemove + 3 clicks)", len(r.calls))
	}
	for i := 1; i < 4; i++ {
		if r.calls[i][2] != "5" {
			t.Errorf("expected button 5, got %v", r.calls[i])
		}
	}
}

func TestLinuxEngine_CoordScroll_RefusesOnWayland(t *testing.T) {
	r := &fakeRunner{}
	e := NewLinuxEngine("").AsWayland().WithCommandRunner(r.run)
	err := e.CoordScroll(context.Background(), 0, 0, 0, 10)
	if err == nil || !strings.Contains(err.Error(), "Wayland") {
		t.Errorf("Wayland must refuse coord scroll, got %v", err)
	}
}
