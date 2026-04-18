// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package coordinate

import "testing"

func TestCoordClick_BuildsActionWithScaledXY(t *testing.T) {
	cfg := Config{Screen: ScalingTarget{Width: 1920, Height: 1080}}
	a, err := CoordClick(cfg, ScaleAPIToScreen, 0.5, 0.5)
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != KindCoordClick {
		t.Errorf("kind = %q, want %q", a.Kind, KindCoordClick)
	}
	if a.X != 960 || a.Y != 540 {
		t.Errorf("scaled XY wrong: (%d, %d)", a.X, a.Y)
	}
}

func TestCoordClick_ErrorsOnOutOfBounds(t *testing.T) {
	cfg := Config{Screen: ScalingTarget{Width: 100, Height: 100}}
	_, err := CoordClick(cfg, ScaleAPIToScreen, 1.0, 1.0)
	if err == nil {
		t.Error("expected out-of-bounds error")
	}
}

func TestCoordType_EmptyTextRejected(t *testing.T) {
	cfg := Config{Screen: ScalingTarget{Width: 1920, Height: 1080}}
	_, err := CoordType(cfg, ScaleAPIToScreen, 0.1, 0.1, "")
	if err == nil {
		t.Error("empty text must error")
	}
}

func TestCoordType_BuildsScaledActionWithText(t *testing.T) {
	cfg := Config{Screen: ScalingTarget{Width: 1920, Height: 1080}}
	a, err := CoordType(cfg, ScaleAPIToScreen, 0.5, 0.5, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != KindCoordType || a.Text != "hello" {
		t.Errorf("unexpected action: %+v", a)
	}
}

func TestCoordDrag_ScalesBothEndpoints(t *testing.T) {
	cfg := Config{Screen: ScalingTarget{Width: 1920, Height: 1080}}
	a, err := CoordDrag(cfg, ScaleAPIToScreen, 0.25, 0.25, 0.75, 0.75)
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != KindCoordDrag || a.X != 480 || a.Y != 270 {
		t.Errorf("start endpoint wrong: %+v", a)
	}
	if endX := a.Params["end_x"].(int); endX != 1440 {
		t.Errorf("end_x = %d, want 1440", endX)
	}
	if endY := a.Params["end_y"].(int); endY != 810 {
		t.Errorf("end_y = %d, want 810", endY)
	}
}

func TestCoordScroll_CarriesDeltas(t *testing.T) {
	cfg := Config{Screen: ScalingTarget{Width: 1920, Height: 1080}}
	a, err := CoordScroll(cfg, ScaleAPIToScreen, 0.5, 0.5, 0, -320)
	if err != nil {
		t.Fatal(err)
	}
	if a.Params["dy"].(int) != -320 {
		t.Errorf("dy carried wrongly: %+v", a.Params)
	}
}

func TestIsCoordKind(t *testing.T) {
	cases := map[string]bool{
		KindCoordClick:  true,
		KindCoordType:   true,
		KindCoordDrag:   true,
		KindCoordScroll: true,
		"click":         false,
		"":              false,
	}
	for k, want := range cases {
		if got := IsCoordKind(k); got != want {
			t.Errorf("IsCoordKind(%q) = %v, want %v", k, got, want)
		}
	}
}
