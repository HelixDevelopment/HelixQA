// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package coordinate

import (
	"errors"
	"fmt"

	"digital.vasic.helixqa/pkg/nexus"
)

// Canonical Kind constants for coordinate-grounded Actions. Every
// Adapter that supports coord-mode (pkg/nexus/browser,
// pkg/nexus/desktop, pkg/nexus/mobile ADB tap) translates these
// into its own primitives.
const (
	KindCoordClick  = "coord_click"
	KindCoordType   = "coord_type"
	KindCoordDrag   = "coord_drag"
	KindCoordScroll = "coord_scroll"
)

// CoordClick returns a nexus.Action that clicks at the scaled
// (x, y). The caller supplies ScaleCoordinates config + raw
// model-space coordinate; the builder scales and records the final
// pixel X/Y on the Action so the Adapter can dispatch without
// re-running the scaler.
func CoordClick(cfg Config, mode ScaleMode, x, y float64) (nexus.Action, error) {
	px, py, err := ScaleCoordinates(cfg, mode, x, y)
	if err != nil {
		return nexus.Action{}, fmt.Errorf("coord_click: %w", err)
	}
	return nexus.Action{Kind: KindCoordClick, X: px, Y: py}, nil
}

// CoordType emits a coord-anchored type action. Adapters click at
// (X, Y) first (so the focus lands on the right widget) and then
// type Text.
func CoordType(cfg Config, mode ScaleMode, x, y float64, text string) (nexus.Action, error) {
	if text == "" {
		return nexus.Action{}, errors.New("coord_type: empty text")
	}
	px, py, err := ScaleCoordinates(cfg, mode, x, y)
	if err != nil {
		return nexus.Action{}, fmt.Errorf("coord_type: %w", err)
	}
	return nexus.Action{Kind: KindCoordType, X: px, Y: py, Text: text}, nil
}

// CoordDrag emits a drag from (startX, startY) to (endX, endY). The
// end coordinates ride on the Params map so the Action stays
// single-shape; Adapters read Params["end_x"] / Params["end_y"].
func CoordDrag(cfg Config, mode ScaleMode, startX, startY, endX, endY float64) (nexus.Action, error) {
	sx, sy, err := ScaleCoordinates(cfg, mode, startX, startY)
	if err != nil {
		return nexus.Action{}, fmt.Errorf("coord_drag start: %w", err)
	}
	ex, ey, err := ScaleCoordinates(cfg, mode, endX, endY)
	if err != nil {
		return nexus.Action{}, fmt.Errorf("coord_drag end: %w", err)
	}
	return nexus.Action{
		Kind:   KindCoordDrag,
		X:      sx,
		Y:      sy,
		Params: map[string]any{"end_x": ex, "end_y": ey},
	}, nil
}

// CoordScroll emits a coord-anchored scroll. Params carry dx / dy
// in pre-scaled pixels so adapters skip the scaler on dispatch.
func CoordScroll(cfg Config, mode ScaleMode, x, y float64, dx, dy int) (nexus.Action, error) {
	px, py, err := ScaleCoordinates(cfg, mode, x, y)
	if err != nil {
		return nexus.Action{}, fmt.Errorf("coord_scroll: %w", err)
	}
	return nexus.Action{
		Kind:   KindCoordScroll,
		X:      px,
		Y:      py,
		Params: map[string]any{"dx": dx, "dy": dy},
	}, nil
}

// IsCoordKind reports whether k is one of the canonical coord
// action kinds. Helpers use this to decide whether to run an
// Adapter's coord-mode dispatch path vs. the element-ref path.
func IsCoordKind(k string) bool {
	switch k {
	case KindCoordClick, KindCoordType, KindCoordDrag, KindCoordScroll:
		return true
	}
	return false
}
