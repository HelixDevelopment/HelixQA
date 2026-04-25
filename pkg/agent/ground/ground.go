// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package ground bridges the VLM-produced action layer (pkg/agent/uitars)
// and the GUI element detector (pkg/agent/omniparser) into a single
// "grounded" agent that cross-validates click coordinates against a
// detector-confirmed element grid.
//
// The problem: raw VLM click coordinates are noisy. UI-TARS might
// propose (120, 340) for a "Sign In" button whose actual bbox is
// (100, 320)-(200, 360). Executing the raw click works half the time
// (the point is inside the bbox) and misses the other half (just
// outside, or on the padding). OmniParser gives us a deterministic
// element grid; the grounder snaps the VLM coordinate to the
// element center when the VLM click falls inside or near an element.
//
// The grounder is CGO-free and requires no running services at
// test time — it consumes two narrow interfaces (Actor, Detector)
// that the real UI-TARS / OmniParser clients satisfy, and that
// tests mock directly.
package ground

import (
	"context"
	"errors"
	"fmt"
	"image"
	"math"

	"digital.vasic.helixqa/pkg/agent/action"
	"digital.vasic.helixqa/pkg/agent/omniparser"
)

// Actor is the minimal contract the VLM backend must satisfy.
// Satisfied by *uitars.Client and by any future Phase-3 VLM client
// (SGLang-wrapped, OpenAI-compatible, Claude vision, etc.) that
// returns a single action.Action per screenshot+instruction pair.
type Actor interface {
	Act(ctx context.Context, screenshot image.Image, instruction string) (action.Action, error)
}

// Detector is the minimal contract the element grid provider must
// satisfy. Satisfied by *omniparser.Client. Nil is allowed —
// when Detector is nil the grounder degrades to a pass-through that
// executes Actor output unchanged (useful when the sidecar isn't
// deployed yet).
type Detector interface {
	Parse(ctx context.Context, screenshot image.Image) (omniparser.Result, error)
}

// Grounder cross-validates VLM actions against a detector-confirmed
// element grid.
type Grounder struct {
	Actor    Actor
	Detector Detector

	// SnapToNearest — when true, click coordinates that fall outside
	// every interactive element are snapped to the nearest element
	// (if one is within MaxSnapDist pixels). When false, clicks that
	// miss every element are passed through unchanged; execution can
	// still succeed if the VLM's coord happens to be on a true
	// interactable the detector missed (common for custom widgets).
	SnapToNearest bool

	// MinConfidence filters out low-confidence OmniParser elements
	// from the grounding pool. Default 0.5 — the typical confidence
	// floor below which OmniParser detections are often phantom
	// elements in busy screens.
	MinConfidence float64

	// MaxSnapDist caps snap-to-nearest at this pixel distance.
	// Beyond this, the raw VLM coord is returned unchanged even with
	// SnapToNearest=true — prevents nonsensical snaps across huge
	// empty regions of the screen. Default 64 px.
	MaxSnapDist int
}

// Sentinel errors.
var (
	ErrNoActor = errors.New("helixqa/agent/ground: Grounder.Actor is nil")
)

// Resolve drives the full Actor → Detector → grounding pipeline:
//
//  1. Ask the Actor for a proposed action.
//  2. If the action isn't a click (or Detector is nil), return
//     the Actor's output unchanged.
//  3. Parse the screenshot through the Detector.
//  4. If the click falls inside an interactive element, snap to
//     that element's center; annotate the Reason with the matched
//     element's type + text.
//  5. If the click misses every element and SnapToNearest is set,
//     snap to the nearest interactive element within MaxSnapDist.
//  6. Otherwise return the raw action as-is.
//
// The returned Action always passes action.Validate(); grounding
// never produces an invalid action.
func (g *Grounder) Resolve(ctx context.Context, screenshot image.Image, instruction string) (action.Action, error) {
	if g.Actor == nil {
		return action.Action{}, ErrNoActor
	}

	proposed, err := g.Actor.Act(ctx, screenshot, instruction)
	if err != nil {
		return action.Action{}, fmt.Errorf("ground: Actor.Act: %w", err)
	}

	if proposed.Kind != action.KindClick || g.Detector == nil {
		return proposed, nil
	}

	result, err := g.Detector.Parse(ctx, screenshot)
	if err != nil {
		return action.Action{}, fmt.Errorf("ground: Detector.Parse: %w", err)
	}

	grounded := g.groundClick(proposed, result)
	return grounded, nil
}

// groundClick is the pure-function core of Resolve: given a raw
// click-kind action and a parsed element grid, emit the grounded
// action. Unit-testable without any HTTP or VLM dependency.
func (g *Grounder) groundClick(proposed action.Action, result omniparser.Result) action.Action {
	minConf := g.MinConfidence
	if minConf <= 0 {
		minConf = 0.5
	}
	maxSnap := g.MaxSnapDist
	if maxSnap <= 0 {
		maxSnap = 64
	}

	p := image.Point{X: proposed.X, Y: proposed.Y}

	// Phase 1: is the click already inside an interactive element?
	if hit, ok := findContaining(result.Elements, p, minConf); ok {
		return snapTo(proposed, hit, "grounded to element center")
	}

	// Phase 2: snap-to-nearest, if configured.
	if g.SnapToNearest {
		nearest, dist, found := findNearest(result.Elements, p, minConf)
		if found && dist <= float64(maxSnap) {
			return snapTo(proposed, nearest, fmt.Sprintf("snapped (%.0fpx) to nearest element", dist))
		}
	}

	// Phase 3: keep the raw VLM action.
	return proposed
}

// snapTo replaces proposed.X, Y with the element's center and
// appends a grounding note to Reason.
func snapTo(proposed action.Action, e omniparser.Element, note string) action.Action {
	c := e.Center()
	out := proposed
	out.X = c.X
	out.Y = c.Y
	out.Reason = mergeReason(proposed.Reason, fmt.Sprintf("%s [%s: %s]", note, e.Type, truncate(e.Text, 40)))
	return out
}

// findContaining returns the smallest interactive element that
// contains p and passes the confidence floor.
func findContaining(elements []omniparser.Element, p image.Point, minConf float64) (omniparser.Element, bool) {
	var best omniparser.Element
	var bestArea int
	found := false
	for _, e := range elements {
		if !e.Interactive || e.Confidence < minConf {
			continue
		}
		if !pointIn(e.BBox, p) {
			continue
		}
		area := bboxArea(e.BBox)
		if !found || area < bestArea {
			best = e
			bestArea = area
			found = true
		}
	}
	return best, found
}

// findNearest returns the interactive element whose bbox is closest
// to p (Euclidean distance from p to the nearest point of the bbox).
// Returns (_, 0, false) when no element meets the confidence floor.
func findNearest(elements []omniparser.Element, p image.Point, minConf float64) (omniparser.Element, float64, bool) {
	var best omniparser.Element
	bestDist := math.Inf(1)
	found := false
	for _, e := range elements {
		if !e.Interactive || e.Confidence < minConf {
			continue
		}
		d := distToRect(p, e.BBox)
		if d < bestDist {
			bestDist = d
			best = e
			found = true
		}
	}
	if !found {
		return omniparser.Element{}, 0, false
	}
	return best, bestDist, true
}

// distToRect returns the Euclidean distance from p to the nearest
// point of r. Returns 0 when p is inside r.
func distToRect(p image.Point, r image.Rectangle) float64 {
	dx := 0
	dy := 0
	if p.X < r.Min.X {
		dx = r.Min.X - p.X
	} else if p.X >= r.Max.X {
		dx = p.X - (r.Max.X - 1)
	}
	if p.Y < r.Min.Y {
		dy = r.Min.Y - p.Y
	} else if p.Y >= r.Max.Y {
		dy = p.Y - (r.Max.Y - 1)
	}
	return math.Sqrt(float64(dx*dx + dy*dy))
}

func pointIn(r image.Rectangle, p image.Point) bool {
	return p.X >= r.Min.X && p.X < r.Max.X && p.Y >= r.Min.Y && p.Y < r.Max.Y
}

func bboxArea(r image.Rectangle) int {
	if r.Max.X <= r.Min.X || r.Max.Y <= r.Min.Y {
		return 0
	}
	return (r.Max.X - r.Min.X) * (r.Max.Y - r.Min.Y)
}

// mergeReason concatenates the VLM-provided Reason with the
// grounding note. Keeps the VLM reason first (most informative) and
// appends the grounding audit trail.
func mergeReason(vlm, grounding string) string {
	if vlm == "" {
		return grounding
	}
	return vlm + " | " + grounding
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
