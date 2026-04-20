// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package sglang

import (
	"context"
	"errors"
	"image"
	"strings"
	"testing"

	"digital.vasic.helixqa/pkg/agent/action"
)

// ---------------------------------------------------------------------------
// Schema.Check — baseline action.Validate integration
// ---------------------------------------------------------------------------

func TestCheck_BaselineValidateFails(t *testing.T) {
	// action.Kind="click" but X<0 → Action.Validate() fails.
	a := action.Action{Kind: action.KindClick, X: -1, Y: 0}
	if err := (Schema{}).Check(a); !errors.Is(err, ErrBaseValidate) {
		t.Fatalf("err = %v, want ErrBaseValidate", err)
	}
}

func TestCheck_EmptySchemaAcceptsValidAction(t *testing.T) {
	a := action.Action{Kind: action.KindClick, X: 10, Y: 20}
	if err := (Schema{}).Check(a); err != nil {
		t.Fatalf("empty schema should accept valid action: %v", err)
	}
}

// ---------------------------------------------------------------------------
// AllowedKinds
// ---------------------------------------------------------------------------

func TestCheck_AllowedKindsFilter(t *testing.T) {
	s := Schema{AllowedKinds: []action.Kind{action.KindClick, action.KindDone}}

	if err := s.Check(action.Action{Kind: action.KindClick, X: 10, Y: 20}); err != nil {
		t.Fatalf("allowed Kind rejected: %v", err)
	}
	if err := s.Check(action.Action{Kind: action.KindDone}); err != nil {
		t.Fatalf("allowed Kind rejected: %v", err)
	}

	// Type isn't in AllowedKinds.
	err := s.Check(action.Action{Kind: action.KindType, Text: "hello"})
	if !errors.Is(err, ErrDisallowedKind) {
		t.Fatalf("disallowed Kind = %v, want ErrDisallowedKind", err)
	}
}

func TestCheck_EmptyAllowedKindsMeansAll(t *testing.T) {
	// No AllowedKinds set → every valid Kind passes.
	s := Schema{}
	cases := []action.Action{
		{Kind: action.KindClick, X: 0, Y: 0},
		{Kind: action.KindType, Text: "hi"},
		{Kind: action.KindKey, Key: "ENTER"},
		{Kind: action.KindDone},
	}
	for _, a := range cases {
		if err := s.Check(a); err != nil {
			t.Errorf("empty AllowedKinds rejected %v: %v", a.Kind, err)
		}
	}
}

// ---------------------------------------------------------------------------
// RequireReason
// ---------------------------------------------------------------------------

func TestCheck_RequireReasonAccepts(t *testing.T) {
	s := Schema{RequireReason: true}
	a := action.Action{Kind: action.KindClick, X: 10, Y: 20, Reason: "btn"}
	if err := s.Check(a); err != nil {
		t.Fatalf("non-empty Reason rejected: %v", err)
	}
}

func TestCheck_RequireReasonRejectsEmpty(t *testing.T) {
	s := Schema{RequireReason: true}
	a := action.Action{Kind: action.KindClick, X: 10, Y: 20, Reason: ""}
	if err := s.Check(a); !errors.Is(err, ErrMissingReason) {
		t.Fatalf("empty Reason = %v, want ErrMissingReason", err)
	}
}

func TestCheck_RequireReasonRejectsWhitespaceOnly(t *testing.T) {
	s := Schema{RequireReason: true}
	a := action.Action{Kind: action.KindClick, X: 10, Y: 20, Reason: "   \t\n"}
	if err := s.Check(a); !errors.Is(err, ErrMissingReason) {
		t.Fatalf("whitespace-only Reason = %v, want ErrMissingReason", err)
	}
}

// ---------------------------------------------------------------------------
// MaxTextLen (measured in runes)
// ---------------------------------------------------------------------------

func TestCheck_MaxTextLenRejectsOverflow(t *testing.T) {
	s := Schema{MaxTextLen: 5}
	a := action.Action{Kind: action.KindType, Text: "123456"} // 6 runes
	err := s.Check(a)
	if !errors.Is(err, ErrTextTooLong) {
		t.Fatalf("overflow Text = %v, want ErrTextTooLong", err)
	}
}

func TestCheck_MaxTextLenAcceptsBoundary(t *testing.T) {
	s := Schema{MaxTextLen: 5}
	a := action.Action{Kind: action.KindType, Text: "12345"}
	if err := s.Check(a); err != nil {
		t.Fatalf("boundary Text rejected: %v", err)
	}
}

func TestCheck_MaxTextLenCountsRunesNotBytes(t *testing.T) {
	// 3 Cyrillic runes = 6 bytes. MaxTextLen=3 should accept.
	s := Schema{MaxTextLen: 3}
	a := action.Action{Kind: action.KindType, Text: "абв"}
	if err := s.Check(a); err != nil {
		t.Fatalf("3-rune Cyrillic rejected with MaxTextLen=3: %v", err)
	}
	// 4-rune: rejected.
	a2 := action.Action{Kind: action.KindType, Text: "абвг"}
	if err := s.Check(a2); !errors.Is(err, ErrTextTooLong) {
		t.Fatalf("4-rune Cyrillic with MaxTextLen=3: err=%v, want ErrTextTooLong", err)
	}
}

func TestCheck_MaxTextLenZeroMeansUncapped(t *testing.T) {
	s := Schema{MaxTextLen: 0}
	a := action.Action{Kind: action.KindType, Text: strings.Repeat("x", 10000)}
	if err := s.Check(a); err != nil {
		t.Fatalf("MaxTextLen=0 should accept arbitrarily long text: %v", err)
	}
}

// ---------------------------------------------------------------------------
// ScreenBounds
// ---------------------------------------------------------------------------

func TestCheck_ScreenBoundsAcceptsInsideClick(t *testing.T) {
	s := Schema{ScreenBounds: image.Rect(0, 0, 1920, 1080)}
	a := action.Action{Kind: action.KindClick, X: 500, Y: 400}
	if err := s.Check(a); err != nil {
		t.Fatalf("inside click rejected: %v", err)
	}
}

func TestCheck_ScreenBoundsRejectsOutsideClick(t *testing.T) {
	s := Schema{ScreenBounds: image.Rect(0, 0, 1920, 1080)}
	a := action.Action{Kind: action.KindClick, X: 5000, Y: 400}
	if err := s.Check(a); !errors.Is(err, ErrOutOfBounds) {
		t.Fatalf("outside click = %v, want ErrOutOfBounds", err)
	}
}

func TestCheck_ScreenBoundsChecksBothSwipeEndpoints(t *testing.T) {
	s := Schema{ScreenBounds: image.Rect(0, 0, 1000, 1000)}

	// Start inside, end outside.
	endOut := action.Action{Kind: action.KindSwipe, X: 100, Y: 100, X2: 2000, Y2: 100, DurationMs: 100}
	if err := s.Check(endOut); !errors.Is(err, ErrOutOfBounds) {
		t.Fatalf("swipe end outside = %v, want ErrOutOfBounds", err)
	}

	// Start outside, end inside.
	startOut := action.Action{Kind: action.KindSwipe, X: 2000, Y: 100, X2: 500, Y2: 500, DurationMs: 100}
	if err := s.Check(startOut); !errors.Is(err, ErrOutOfBounds) {
		t.Fatalf("swipe start outside = %v, want ErrOutOfBounds", err)
	}

	// Both inside.
	ok := action.Action{Kind: action.KindSwipe, X: 100, Y: 100, X2: 500, Y2: 500, DurationMs: 100}
	if err := s.Check(ok); err != nil {
		t.Fatalf("swipe inside rejected: %v", err)
	}
}

func TestCheck_ScreenBoundsIgnoredForNonCoordKinds(t *testing.T) {
	// Type / Key / Done have no coords — bounds check should not
	// fire.
	s := Schema{ScreenBounds: image.Rect(0, 0, 1920, 1080)}
	cases := []action.Action{
		{Kind: action.KindType, Text: "hello"},
		{Kind: action.KindKey, Key: "ENTER"},
		{Kind: action.KindDone},
		{Kind: action.KindWait, DurationMs: 100},
	}
	for _, a := range cases {
		if err := s.Check(a); err != nil {
			t.Errorf("%v should not trigger bounds check: %v", a.Kind, err)
		}
	}
}

func TestCheck_ScreenBoundsZeroMeansUncapped(t *testing.T) {
	// Zero-value rect → no bounds check.
	s := Schema{}
	a := action.Action{Kind: action.KindClick, X: 10000, Y: 10000}
	if err := s.Check(a); err != nil {
		t.Fatalf("zero ScreenBounds should not enforce: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Guard.Act — retry behaviour
// ---------------------------------------------------------------------------

// scriptedActor returns actions[0..N-1] in order, then an error.
type scriptedActor struct {
	actions   []action.Action
	calls     int
	lastInstr string
}

func (s *scriptedActor) Act(ctx context.Context, img image.Image, instruction string) (action.Action, error) {
	s.lastInstr = instruction
	if s.calls >= len(s.actions) {
		return action.Action{}, errors.New("scriptedActor: out of actions")
	}
	a := s.actions[s.calls]
	s.calls++
	return a, nil
}

func tinyImg() image.Image { return image.NewRGBA(image.Rect(0, 0, 8, 8)) }

func TestGuard_FirstAttemptValidPassesThrough(t *testing.T) {
	actor := &scriptedActor{actions: []action.Action{{Kind: action.KindClick, X: 10, Y: 20}}}
	g := &Guard{Actor: actor, Schema: Schema{}}
	a, err := g.Act(context.Background(), tinyImg(), "go")
	if err != nil {
		t.Fatalf("Act: %v", err)
	}
	if a.Kind != action.KindClick {
		t.Fatalf("got %v", a)
	}
	if actor.calls != 1 {
		t.Fatalf("actor called %d times, want 1", actor.calls)
	}
}

func TestGuard_InvalidThenValidRetries(t *testing.T) {
	// First attempt returns Text that exceeds MaxTextLen; second is
	// valid.
	actor := &scriptedActor{actions: []action.Action{
		{Kind: action.KindType, Text: "toolong"},
		{Kind: action.KindType, Text: "ok"},
	}}
	g := &Guard{
		Actor:      actor,
		Schema:     Schema{MaxTextLen: 4},
		MaxRetries: 2,
	}
	a, err := g.Act(context.Background(), tinyImg(), "fill field")
	if err != nil {
		t.Fatalf("Act: %v", err)
	}
	if a.Text != "ok" {
		t.Fatalf("got %v", a)
	}
	if actor.calls != 2 {
		t.Fatalf("actor called %d times, want 2", actor.calls)
	}
	// Retry instruction should contain the original + the retry
	// header with error text.
	if !strings.Contains(actor.lastInstr, "fill field") {
		t.Fatal("retry instruction should preserve original")
	}
	if !strings.Contains(actor.lastInstr, "Text exceeds MaxTextLen") {
		t.Fatal("retry instruction should contain validation error")
	}
}

func TestGuard_OnRetryHookFiresPerRetry(t *testing.T) {
	actor := &scriptedActor{actions: []action.Action{
		{Kind: action.KindType, Text: "toolong"},
		{Kind: action.KindType, Text: "alsolong"},
		{Kind: action.KindType, Text: "ok"},
	}}
	var hooks int
	g := &Guard{
		Actor:      actor,
		Schema:     Schema{MaxTextLen: 4},
		MaxRetries: 3,
		OnRetry:    func(attempt int, err error) { hooks++ },
	}
	if _, err := g.Act(context.Background(), tinyImg(), "go"); err != nil {
		t.Fatalf("Act: %v", err)
	}
	if hooks != 2 {
		t.Fatalf("OnRetry fired %d times, want 2 (two failed attempts before success)", hooks)
	}
}

func TestGuard_RetriesExhaustedReturnsSentinel(t *testing.T) {
	actor := &scriptedActor{actions: []action.Action{
		{Kind: action.KindType, Text: "too_long"},
		{Kind: action.KindType, Text: "also_too_long"},
		{Kind: action.KindType, Text: "still_too_long"},
	}}
	g := &Guard{
		Actor:      actor,
		Schema:     Schema{MaxTextLen: 4},
		MaxRetries: 2,
	}
	_, err := g.Act(context.Background(), tinyImg(), "go")
	if !errors.Is(err, ErrRetriesExhausted) {
		t.Fatalf("err = %v, want ErrRetriesExhausted", err)
	}
	if actor.calls != 3 {
		t.Fatalf("actor called %d times, want 3 (1 initial + 2 retries)", actor.calls)
	}
}

func TestGuard_ZeroMaxRetriesUsesDefaultOfTwo(t *testing.T) {
	actor := &scriptedActor{actions: []action.Action{
		{Kind: action.KindType, Text: "nope1"},
		{Kind: action.KindType, Text: "nope2"},
		{Kind: action.KindType, Text: "ok"},
	}}
	g := &Guard{Actor: actor, Schema: Schema{MaxTextLen: 4}} // MaxRetries=0 → default 2
	a, err := g.Act(context.Background(), tinyImg(), "go")
	if err != nil {
		t.Fatalf("Act: %v", err)
	}
	if a.Text != "ok" {
		t.Fatalf("got %v", a)
	}
	if actor.calls != 3 {
		t.Fatalf("actor called %d times, want 3 (default MaxRetries=2 + initial)", actor.calls)
	}
}

func TestGuard_CustomRetryHeader(t *testing.T) {
	actor := &scriptedActor{actions: []action.Action{
		{Kind: action.KindType, Text: "toolong"},
		{Kind: action.KindType, Text: "ok"},
	}}
	g := &Guard{
		Actor:       actor,
		Schema:      Schema{MaxTextLen: 4},
		MaxRetries:  2,
		RetryHeader: "Custom retry preamble: %s — please fix.",
	}
	_, _ = g.Act(context.Background(), tinyImg(), "go")
	if !strings.Contains(actor.lastInstr, "Custom retry preamble:") {
		t.Fatalf("custom retry header not applied: %q", actor.lastInstr)
	}
}

func TestGuard_ActorErrorPropagatesWithAttempt(t *testing.T) {
	g := &Guard{
		Actor:  ActorFunc(func(ctx context.Context, img image.Image, instr string) (action.Action, error) {
			return action.Action{}, errors.New("VLM down")
		}),
		Schema: Schema{},
	}
	_, err := g.Act(context.Background(), tinyImg(), "go")
	if err == nil || !strings.Contains(err.Error(), "attempt 1") {
		t.Fatalf("Actor error should wrap with attempt: %v", err)
	}
}

func TestGuard_NilActorReturnsErrNoActor(t *testing.T) {
	g := &Guard{}
	if _, err := g.Act(context.Background(), tinyImg(), "go"); !errors.Is(err, ErrNoActor) {
		t.Fatalf("nil Actor = %v, want ErrNoActor", err)
	}
}

func TestGuard_ContextCanceled(t *testing.T) {
	actor := &scriptedActor{actions: []action.Action{{Kind: action.KindDone}}}
	g := &Guard{Actor: actor, Schema: Schema{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := g.Act(ctx, tinyImg(), "go"); err == nil {
		t.Fatal("canceled ctx should fail")
	}
}

// ---------------------------------------------------------------------------
// ActorFunc adapter
// ---------------------------------------------------------------------------

func TestActorFunc_Dispatches(t *testing.T) {
	called := false
	af := ActorFunc(func(ctx context.Context, img image.Image, instr string) (action.Action, error) {
		called = true
		return action.Action{Kind: action.KindDone}, nil
	})
	if _, err := af.Act(context.Background(), tinyImg(), "go"); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("ActorFunc didn't dispatch")
	}
}

// ---------------------------------------------------------------------------
// pointIn helper
// ---------------------------------------------------------------------------

func TestPointIn_MinInclusiveMaxExclusive(t *testing.T) {
	r := image.Rect(10, 10, 20, 20)
	if !pointIn(r, image.Point{X: 10, Y: 10}) {
		t.Error("Min should be inclusive")
	}
	if pointIn(r, image.Point{X: 20, Y: 20}) {
		t.Error("Max should be exclusive")
	}
	if !pointIn(r, image.Point{X: 15, Y: 15}) {
		t.Error("interior")
	}
	if pointIn(r, image.Point{X: 5, Y: 15}) {
		t.Error("left of rect")
	}
}
