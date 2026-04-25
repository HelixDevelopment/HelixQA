// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ground

import (
	"context"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/agent/action"
	"digital.vasic.helixqa/pkg/agent/omniparser"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type fakeActor struct {
	out action.Action
	err error
}

func (f *fakeActor) Act(ctx context.Context, img image.Image, instruction string) (action.Action, error) {
	if err := ctx.Err(); err != nil {
		return action.Action{}, err
	}
	if f.err != nil {
		return action.Action{}, f.err
	}
	return f.out, nil
}

type fakeDetector struct {
	out omniparser.Result
	err error
}

func (f *fakeDetector) Parse(ctx context.Context, img image.Image) (omniparser.Result, error) {
	if err := ctx.Err(); err != nil {
		return omniparser.Result{}, err
	}
	if f.err != nil {
		return omniparser.Result{}, f.err
	}
	return f.out, nil
}

// ---------------------------------------------------------------------------
// Fixture helpers
// ---------------------------------------------------------------------------

func tinyImg() image.Image {
	return image.NewRGBA(image.Rect(0, 0, 8, 8))
}

func elem(x1, y1, x2, y2 int, typ, text string, interactive bool, conf float64) omniparser.Element {
	return omniparser.Element{
		BBox:        image.Rect(x1, y1, x2, y2),
		Type:        typ,
		Text:        text,
		Interactive: interactive,
		Confidence:  conf,
	}
}

// ---------------------------------------------------------------------------
// Resolve — happy path
// ---------------------------------------------------------------------------

func TestResolve_NonClickActionPassesThrough(t *testing.T) {
	a := action.Action{Kind: action.KindType, Text: "admin", Reason: "fill"}
	g := &Grounder{
		Actor:    &fakeActor{out: a},
		Detector: &fakeDetector{}, // would be called but shouldn't be for type
	}
	got, err := g.Resolve(context.Background(), tinyImg(), "go")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != a {
		t.Fatalf("type action should pass through unchanged: %+v", got)
	}
}

func TestResolve_NilDetectorBypassesGrounding(t *testing.T) {
	a := action.Action{Kind: action.KindClick, X: 120, Y: 340, Reason: "btn"}
	g := &Grounder{Actor: &fakeActor{out: a}} // Detector nil
	got, err := g.Resolve(context.Background(), tinyImg(), "go")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != a {
		t.Fatalf("nil detector should bypass grounding: %+v", got)
	}
}

func TestResolve_ClickInsideElement_SnapsToCenterAndAnnotates(t *testing.T) {
	// VLM proposes (120, 340) — inside a 100×320–200×360 button.
	// Grounder snaps to center (150, 340) and appends a grounding
	// note to Reason.
	proposed := action.Action{Kind: action.KindClick, X: 120, Y: 340, Reason: "Sign-In button"}
	detected := omniparser.Result{
		Elements: []omniparser.Element{
			elem(100, 320, 200, 360, "button", "Sign in", true, 0.9),
		},
	}
	g := &Grounder{
		Actor:    &fakeActor{out: proposed},
		Detector: &fakeDetector{out: detected},
	}
	got, err := g.Resolve(context.Background(), tinyImg(), "go")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got.X != 150 || got.Y != 340 {
		t.Fatalf("snapped coords = (%d, %d), want (150, 340)", got.X, got.Y)
	}
	if !strings.Contains(got.Reason, "grounded to element center") {
		t.Fatalf("Reason missing grounding annotation: %q", got.Reason)
	}
	if !strings.Contains(got.Reason, "Sign-In button") {
		t.Fatalf("original Reason lost: %q", got.Reason)
	}
}

func TestResolve_ClickInsideElement_PrefersSmallestEnclosing(t *testing.T) {
	// Nested elements: huge background group + small button.
	proposed := action.Action{Kind: action.KindClick, X: 150, Y: 150}
	detected := omniparser.Result{
		Elements: []omniparser.Element{
			elem(0, 0, 1000, 1000, "group", "background", true, 0.9),
			elem(100, 100, 200, 200, "button", "Login", true, 0.9),
		},
	}
	g := &Grounder{
		Actor:    &fakeActor{out: proposed},
		Detector: &fakeDetector{out: detected},
	}
	got, _ := g.Resolve(context.Background(), tinyImg(), "go")
	// Center of the inner button is (150, 150) — same as the click.
	if got.X != 150 || got.Y != 150 {
		t.Fatalf("coords = (%d, %d)", got.X, got.Y)
	}
	if !strings.Contains(got.Reason, "Login") {
		t.Fatalf("Reason should mention the inner button: %q", got.Reason)
	}
}

func TestResolve_ClickInsideElement_FiltersLowConfidence(t *testing.T) {
	// A low-confidence element contains the click; a high-confidence
	// element also contains it. The high-conf one should win.
	proposed := action.Action{Kind: action.KindClick, X: 50, Y: 50}
	detected := omniparser.Result{
		Elements: []omniparser.Element{
			elem(0, 0, 1000, 1000, "phantom", "noise", true, 0.1), // below default 0.5
			elem(10, 10, 100, 100, "button", "Real", true, 0.95),
		},
	}
	g := &Grounder{
		Actor:    &fakeActor{out: proposed},
		Detector: &fakeDetector{out: detected},
	}
	got, _ := g.Resolve(context.Background(), tinyImg(), "go")
	if !strings.Contains(got.Reason, "Real") {
		t.Fatalf("low-conf phantom should be filtered: %q", got.Reason)
	}
}

// ---------------------------------------------------------------------------
// SnapToNearest behaviour
// ---------------------------------------------------------------------------

func TestResolve_SnapToNearest_SnapsWhenClose(t *testing.T) {
	// Click at (250, 340) — 50px right of the button (ends at 200).
	// MaxSnapDist defaults to 64, so the snap fires.
	proposed := action.Action{Kind: action.KindClick, X: 250, Y: 340}
	detected := omniparser.Result{
		Elements: []omniparser.Element{
			elem(100, 320, 200, 360, "button", "Sign in", true, 0.9),
		},
	}
	g := &Grounder{
		Actor:         &fakeActor{out: proposed},
		Detector:      &fakeDetector{out: detected},
		SnapToNearest: true,
	}
	got, _ := g.Resolve(context.Background(), tinyImg(), "go")
	// Button center is (150, 340).
	if got.X != 150 || got.Y != 340 {
		t.Fatalf("snap-to-nearest coords = (%d, %d), want (150, 340)", got.X, got.Y)
	}
	if !strings.Contains(got.Reason, "snapped") {
		t.Fatalf("Reason should mention snap: %q", got.Reason)
	}
}

func TestResolve_SnapToNearest_DisabledPassesThrough(t *testing.T) {
	proposed := action.Action{Kind: action.KindClick, X: 250, Y: 340, Reason: "btn"}
	detected := omniparser.Result{
		Elements: []omniparser.Element{
			elem(100, 320, 200, 360, "button", "Sign in", true, 0.9),
		},
	}
	g := &Grounder{
		Actor:         &fakeActor{out: proposed},
		Detector:      &fakeDetector{out: detected},
		SnapToNearest: false,
	}
	got, _ := g.Resolve(context.Background(), tinyImg(), "go")
	if got.X != 250 || got.Y != 340 {
		t.Fatalf("with SnapToNearest=false, coords should stay at (250, 340), got (%d, %d)", got.X, got.Y)
	}
	if got.Reason != "btn" {
		t.Fatalf("Reason should be unchanged: %q", got.Reason)
	}
}

func TestResolve_SnapToNearest_RespectsMaxSnapDist(t *testing.T) {
	// Click at (2000, 340) — 1800px from the button. Default
	// MaxSnapDist=64, so no snap.
	proposed := action.Action{Kind: action.KindClick, X: 2000, Y: 340, Reason: "btn"}
	detected := omniparser.Result{
		Elements: []omniparser.Element{
			elem(100, 320, 200, 360, "button", "Sign in", true, 0.9),
		},
	}
	g := &Grounder{
		Actor:         &fakeActor{out: proposed},
		Detector:      &fakeDetector{out: detected},
		SnapToNearest: true,
	}
	got, _ := g.Resolve(context.Background(), tinyImg(), "go")
	if got.X != 2000 || got.Y != 340 {
		t.Fatalf("click beyond MaxSnapDist should pass through, got (%d, %d)", got.X, got.Y)
	}
}

func TestResolve_SnapToNearest_CustomMaxSnapDist(t *testing.T) {
	// Click at (300, 340) — 100px right of the button. Default
	// MaxSnapDist=64 would NOT snap. Custom 200 DOES snap.
	proposed := action.Action{Kind: action.KindClick, X: 300, Y: 340, Reason: "btn"}
	detected := omniparser.Result{
		Elements: []omniparser.Element{
			elem(100, 320, 200, 360, "button", "Sign in", true, 0.9),
		},
	}
	g := &Grounder{
		Actor:         &fakeActor{out: proposed},
		Detector:      &fakeDetector{out: detected},
		SnapToNearest: true,
		MaxSnapDist:   200,
	}
	got, _ := g.Resolve(context.Background(), tinyImg(), "go")
	if got.X != 150 {
		t.Fatalf("custom MaxSnapDist=200 should allow snap, got X=%d", got.X)
	}
}

func TestResolve_SnapToNearest_NoElementsAvailable(t *testing.T) {
	proposed := action.Action{Kind: action.KindClick, X: 100, Y: 100, Reason: "btn"}
	detected := omniparser.Result{Elements: nil}
	g := &Grounder{
		Actor:         &fakeActor{out: proposed},
		Detector:      &fakeDetector{out: detected},
		SnapToNearest: true,
	}
	got, _ := g.Resolve(context.Background(), tinyImg(), "go")
	if got != proposed {
		t.Fatalf("empty grid should pass through: %+v", got)
	}
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestResolve_NilActorError(t *testing.T) {
	g := &Grounder{}
	if _, err := g.Resolve(context.Background(), tinyImg(), "go"); !errors.Is(err, ErrNoActor) {
		t.Fatalf("nil Actor: %v, want ErrNoActor", err)
	}
}

func TestResolve_ActorErrorPropagates(t *testing.T) {
	g := &Grounder{Actor: &fakeActor{err: errors.New("boom")}, Detector: &fakeDetector{}}
	_, err := g.Resolve(context.Background(), tinyImg(), "go")
	if err == nil || !strings.Contains(err.Error(), "Actor.Act") {
		t.Fatalf("Actor error should wrap: %v", err)
	}
}

func TestResolve_DetectorErrorPropagates(t *testing.T) {
	g := &Grounder{
		Actor:    &fakeActor{out: action.Action{Kind: action.KindClick, X: 0, Y: 0}},
		Detector: &fakeDetector{err: errors.New("sidecar down")},
	}
	_, err := g.Resolve(context.Background(), tinyImg(), "go")
	if err == nil || !strings.Contains(err.Error(), "Detector.Parse") {
		t.Fatalf("Detector error should wrap: %v", err)
	}
}

func TestResolve_ContextCanceled(t *testing.T) {
	g := &Grounder{
		Actor:    &fakeActor{out: action.Action{Kind: action.KindClick, X: 10, Y: 10}},
		Detector: &fakeDetector{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.Resolve(ctx, tinyImg(), "go"); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

// ---------------------------------------------------------------------------
// Validate output always passes Action.Validate()
// ---------------------------------------------------------------------------

func TestResolve_OutputPassesValidate(t *testing.T) {
	proposed := action.Action{Kind: action.KindClick, X: 120, Y: 340, Reason: "btn"}
	detected := omniparser.Result{
		Elements: []omniparser.Element{
			elem(100, 320, 200, 360, "button", "Sign in", true, 0.9),
		},
	}
	g := &Grounder{
		Actor:    &fakeActor{out: proposed},
		Detector: &fakeDetector{out: detected},
	}
	got, _ := g.Resolve(context.Background(), tinyImg(), "go")
	if err := got.Validate(); err != nil {
		t.Fatalf("grounded action failed Validate: %v", err)
	}
}

// ---------------------------------------------------------------------------
// distToRect — math sanity
// ---------------------------------------------------------------------------

func TestDistToRect_PointInside(t *testing.T) {
	r := image.Rect(10, 10, 20, 20)
	if got := distToRect(image.Point{X: 15, Y: 15}, r); got != 0 {
		t.Fatalf("inside = %v, want 0", got)
	}
}

func TestDistToRect_PointRight(t *testing.T) {
	// Rect ends at X=20 (exclusive), so nearest X is 19. Point at 25
	// → dx = 25 - 19 = 6.
	r := image.Rect(10, 10, 20, 20)
	got := distToRect(image.Point{X: 25, Y: 15}, r)
	if got != 6 {
		t.Fatalf("right-of = %v, want 6", got)
	}
}

func TestDistToRect_PointLeft(t *testing.T) {
	r := image.Rect(10, 10, 20, 20)
	// Point at X=5 → dx = 10 - 5 = 5.
	got := distToRect(image.Point{X: 5, Y: 15}, r)
	if got != 5 {
		t.Fatalf("left-of = %v, want 5", got)
	}
}

func TestDistToRect_PointAbove(t *testing.T) {
	r := image.Rect(10, 10, 20, 20)
	// Point at (15, 3) → dy = 10 - 3 = 7.
	got := distToRect(image.Point{X: 15, Y: 3}, r)
	if got != 7 {
		t.Fatalf("above = %v, want 7", got)
	}
}

func TestDistToRect_PointDiagonal(t *testing.T) {
	r := image.Rect(0, 0, 10, 10)
	// Point at (13, 14) → dx=4 (past max X=10, nearest X=9, 13-9=4),
	// dy=5 (14-9=5). distance=sqrt(16+25)=sqrt(41).
	got := distToRect(image.Point{X: 13, Y: 14}, r)
	// Allow small float rounding.
	want := 6.4031242374328485
	if got < want-0.0001 || got > want+0.0001 {
		t.Fatalf("diagonal = %v, want ≈ %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// findContaining / findNearest unit tests (pure funcs)
// ---------------------------------------------------------------------------

func TestFindContaining_IgnoresNonInteractive(t *testing.T) {
	elements := []omniparser.Element{
		elem(0, 0, 100, 100, "image", "", false, 0.9),
	}
	if _, ok := findContaining(elements, image.Point{X: 50, Y: 50}, 0.5); ok {
		t.Fatal("non-interactive should not match")
	}
}

func TestFindNearest_Empty(t *testing.T) {
	if _, _, ok := findNearest(nil, image.Point{X: 10, Y: 10}, 0.5); ok {
		t.Fatal("empty elements should return false")
	}
}

func TestFindNearest_FiltersConfidence(t *testing.T) {
	elements := []omniparser.Element{
		elem(0, 0, 10, 10, "btn", "", true, 0.1), // below 0.5 floor
	}
	if _, _, ok := findNearest(elements, image.Point{X: 5, Y: 5}, 0.5); ok {
		t.Fatal("low-conf element should be filtered")
	}
}

// ---------------------------------------------------------------------------
// Utility
// ---------------------------------------------------------------------------

func TestMergeReason_EmptyVLM(t *testing.T) {
	if got := mergeReason("", "note"); got != "note" {
		t.Fatalf("empty VLM merge = %q", got)
	}
}

func TestMergeReason_Both(t *testing.T) {
	if got := mergeReason("reason", "note"); got != "reason | note" {
		t.Fatalf("both merge = %q", got)
	}
}

func TestTruncate(t *testing.T) {
	if truncate("short", 10) != "short" {
		t.Fatal("short truncate")
	}
	if truncate("0123456789", 5) != "01234..." {
		t.Fatal("long truncate")
	}
}

func TestPointIn_Boundaries(t *testing.T) {
	r := image.Rect(10, 10, 20, 20)
	if !pointIn(r, image.Point{X: 10, Y: 10}) {
		t.Error("Min inclusive")
	}
	if pointIn(r, image.Point{X: 20, Y: 20}) {
		t.Error("Max exclusive")
	}
}

func TestBboxArea(t *testing.T) {
	if bboxArea(image.Rect(0, 0, 10, 20)) != 200 {
		t.Fatal("normal area")
	}
	bad := image.Rectangle{Min: image.Point{X: 10, Y: 10}, Max: image.Point{X: 5, Y: 5}}
	if bboxArea(bad) != 0 {
		t.Fatal("inverted area should be 0")
	}
}

// ---------------------------------------------------------------------------
// Ensure the package compiles against real uitars/omniparser Clients
// ---------------------------------------------------------------------------

func TestActorAndDetectorInterfaces_SatisfiedByFakes(t *testing.T) {
	// Static-type assertion — if the Actor/Detector interfaces drift
	// the build fails here.
	var _ Actor = &fakeActor{}
	var _ Detector = &fakeDetector{}
	_ = color.RGBA{} // keep the image/color import if test refactors
}
