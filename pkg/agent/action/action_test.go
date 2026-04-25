// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package action

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

func TestValidate_Click(t *testing.T) {
	ok := Action{Kind: KindClick, X: 10, Y: 20, Reason: "button"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid click: %v", err)
	}
	bad := Action{Kind: KindClick, X: -1, Y: 0}
	if err := bad.Validate(); !errors.Is(err, ErrInvalidNumeric) {
		t.Fatalf("negative click X: %v, want ErrInvalidNumeric", err)
	}
}

func TestValidate_Type(t *testing.T) {
	ok := Action{Kind: KindType, Text: "admin"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid type: %v", err)
	}
	bad := Action{Kind: KindType}
	if err := bad.Validate(); !errors.Is(err, ErrMissingField) {
		t.Fatalf("empty text: %v, want ErrMissingField", err)
	}
}

func TestValidate_Scroll(t *testing.T) {
	ok := Action{Kind: KindScroll, DY: -100}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid scroll: %v", err)
	}
	bad := Action{Kind: KindScroll}
	if err := bad.Validate(); !errors.Is(err, ErrMissingField) {
		t.Fatalf("empty scroll: %v, want ErrMissingField", err)
	}
}

func TestValidate_Wait(t *testing.T) {
	ok := Action{Kind: KindWait, DurationMs: 250}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid wait: %v", err)
	}
	zero := Action{Kind: KindWait, DurationMs: 0}
	if err := zero.Validate(); !errors.Is(err, ErrInvalidNumeric) {
		t.Fatalf("zero wait: %v, want ErrInvalidNumeric", err)
	}
	neg := Action{Kind: KindWait, DurationMs: -1}
	if err := neg.Validate(); !errors.Is(err, ErrInvalidNumeric) {
		t.Fatalf("negative wait: %v, want ErrInvalidNumeric", err)
	}
}

func TestValidate_Done_RequiresNothing(t *testing.T) {
	if err := (Action{Kind: KindDone}).Validate(); err != nil {
		t.Fatalf("bare done: %v", err)
	}
}

func TestValidate_Key(t *testing.T) {
	ok := Action{Kind: KindKey, Key: "ENTER"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid key: %v", err)
	}
	bad := Action{Kind: KindKey}
	if err := bad.Validate(); !errors.Is(err, ErrMissingField) {
		t.Fatalf("empty key: %v, want ErrMissingField", err)
	}
}

func TestValidate_Swipe(t *testing.T) {
	ok := Action{Kind: KindSwipe, X: 0, Y: 100, X2: 0, Y2: 500, DurationMs: 200}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid swipe: %v", err)
	}
	bad := Action{Kind: KindSwipe, X: -1, Y: 0, X2: 0, Y2: 0}
	if err := bad.Validate(); !errors.Is(err, ErrInvalidNumeric) {
		t.Fatalf("negative swipe X: %v, want ErrInvalidNumeric", err)
	}
	negDur := Action{Kind: KindSwipe, X: 0, Y: 0, X2: 100, Y2: 100, DurationMs: -1}
	if err := negDur.Validate(); !errors.Is(err, ErrInvalidNumeric) {
		t.Fatalf("negative swipe DurationMs: %v, want ErrInvalidNumeric", err)
	}
}

func TestValidate_OpenApp(t *testing.T) {
	ok := Action{Kind: KindOpenApp, Target: "com.example.app"}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid open_app: %v", err)
	}
	bad := Action{Kind: KindOpenApp}
	if err := bad.Validate(); !errors.Is(err, ErrMissingField) {
		t.Fatalf("empty open_app Target: %v, want ErrMissingField", err)
	}
}

func TestValidate_UnknownKind(t *testing.T) {
	a := Action{Kind: "teleport"}
	err := a.Validate()
	if !errors.Is(err, ErrUnknownKind) {
		t.Fatalf("unknown kind: %v, want ErrUnknownKind", err)
	}
	if !strings.Contains(err.Error(), "teleport") {
		t.Fatalf("error should mention the unknown kind: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Summary — human-readable log strings
// ---------------------------------------------------------------------------

func TestSummary_EveryKind(t *testing.T) {
	cases := []struct {
		a    Action
		want string
	}{
		{Action{Kind: KindClick, X: 10, Y: 20, Reason: "btn"}, "click (10, 20) — btn"},
		{Action{Kind: KindType, Text: "hello", Reason: "input"}, `type "hello" — input`},
		{Action{Kind: KindScroll, DX: 0, DY: -50, Reason: "list"}, "scroll Δ(0, -50) — list"},
		{Action{Kind: KindWait, DurationMs: 250, Reason: "spinner"}, "wait 250ms — spinner"},
		{Action{Kind: KindDone, Reason: "confirmed"}, "done — confirmed"},
		{Action{Kind: KindKey, Key: "ENTER", Reason: "submit"}, "key ENTER — submit"},
		{Action{Kind: KindSwipe, X: 10, Y: 20, X2: 100, Y2: 200, DurationMs: 300, Reason: "drag"},
			"swipe (10,20)→(100,200) over 300ms — drag"},
		{Action{Kind: KindOpenApp, Target: "com.app", Reason: "start"}, `open_app "com.app" — start`},
	}
	for _, c := range cases {
		if got := c.a.Summary(); got != c.want {
			t.Errorf("Summary(%v):\n got: %q\nwant: %q", c.a.Kind, got, c.want)
		}
	}
}

func TestSummary_UnknownKind(t *testing.T) {
	a := Action{Kind: "teleport"}
	if got := a.Summary(); !strings.Contains(got, "unknown") {
		t.Fatalf("unknown-kind summary = %q", got)
	}
}

// ---------------------------------------------------------------------------
// JSON round-trip + ParseJSON
// ---------------------------------------------------------------------------

func TestParseJSON_HappyPath(t *testing.T) {
	in := `{"kind":"click","x":120,"y":340,"reason":"login button"}`
	a, err := ParseJSON([]byte(in))
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if a.Kind != KindClick || a.X != 120 || a.Y != 340 || a.Reason != "login button" {
		t.Fatalf("parsed wrong: %+v", a)
	}
}

func TestParseJSON_MalformedJSON(t *testing.T) {
	if _, err := ParseJSON([]byte("not json")); err == nil {
		t.Fatal("malformed JSON should fail")
	}
}

func TestParseJSON_InvalidAction(t *testing.T) {
	// Valid JSON but Kind violates Validate.
	in := `{"kind":"type","text":""}`
	if _, err := ParseJSON([]byte(in)); !errors.Is(err, ErrMissingField) {
		t.Fatalf("empty text after parse: %v, want ErrMissingField", err)
	}
}

func TestAction_JSONRoundTrip(t *testing.T) {
	original := Action{
		Kind:       KindSwipe,
		X:          10,
		Y:          20,
		X2:         100,
		Y2:         200,
		DurationMs: 250,
		Reason:     "swipe right",
	}
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	a, err := ParseJSON(b)
	if err != nil {
		t.Fatalf("round-trip ParseJSON: %v", err)
	}
	if a != original {
		t.Fatalf("round-trip differs:\n got: %+v\nwant: %+v", a, original)
	}
}

func TestAction_OmitEmptyKeepsWireCompact(t *testing.T) {
	// A bare Done action marshals to {"kind":"done"} — no zero fields leak.
	a := Action{Kind: KindDone}
	b, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"kind":"done"}` {
		t.Fatalf("bare done wire = %q", string(b))
	}
}
