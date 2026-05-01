//go:build helixqa_external


// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"digital.vasic.helixqa/pkg/agent/action"
	"digital.vasic.helixqa/pkg/agent/graph"
	"digital.vasic.helixqa/pkg/agent/ground"
	"digital.vasic.helixqa/pkg/agent/omniparser"
	"digital.vasic.helixqa/pkg/agent/sglang"
	"digital.vasic.helixqa/pkg/agent/uitars"
)

// TestAgentStack_GoalLoopEndToEnd wires the complete Phase-3 agent
// stack together:
//
//	graph.Runner
//	  ├── Screenshotter    — a fake emitting a plain gradient
//	  ├── Resolver         — ground.Grounder
//	  │     ├── Actor      — sglang.Guard wrapping uitars.Client
//	  │     │     └── UI-TARS httptest server (emits JSON actions)
//	  │     └── Detector   — omniparser.Client
//	  │                     └── OmniParser httptest server
//	  └── Executor         — a recording fake
//
// and drives it toward a goal that the UI-TARS mock answers with
// a Click → Type → Done sequence. Asserts:
//
//   - Every layer interoperates (no type mismatches).
//   - SGLang's schema validation accepts valid Actions.
//   - The Grounder snaps the VLM click coord to the OmniParser
//     element's center.
//   - The Runner terminates on the Done action.
//   - The Executor receives exactly the expected 2 actions
//     (click + type; Done is recorded but not dispatched).
func TestAgentStack_GoalLoopEndToEnd(t *testing.T) {
	// Article XI §11.5 classification: this test wires Helix's
	// agent-stack glue (Runner → Resolver → Grounder → Actor →
	// SGLang → UI-TARS) against scripted EXTERNAL services
	// (UI-TARS VLM + OmniParser). UI-TARS and OmniParser are
	// HelixQA's external dependencies — a "real" version requires
	// running their containers with GPU + a model file, neither
	// of which fits CI.
	//
	// SKIP-OK: #BLUFF-HELIXQA-AGENT-STACK-001 — httptest mocks
	// stand in for two external services (UI-TARS VLM,
	// OmniParser). The mocks ARE the test's chosen integration
	// boundary — they're scripted to specific responses and the
	// assertions verify Helix's glue handles them correctly.
	// Reclassification path: move to tests/integration/ (more
	// honest naming) AND add a parallel real-container variant
	// gated by HELIXQA_REAL_AGENT_STACK_E2E env var. Production
	// validation today happens via HelixQA's autonomous pipeline
	// hitting the real Mi Box 4 (qa-results/session-*/) where
	// these components are exercised end-to-end with real
	// hardware. This test's value is type/glue contract, not
	// production parity.
	// ----- Mock UI-TARS server -----
	// Returns a 3-step sequence: click → type → done. The per-call
	// counter ensures we track which step of the conversation the
	// Runner is on.
	var uitarsCalls atomic.Int32
	uitarsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		step := uitarsCalls.Add(1)
		var content string
		switch step {
		case 1:
			content = `{"kind":"click","x":130,"y":335,"reason":"Sign-In button"}`
		case 2:
			content = `{"kind":"type","text":"admin","reason":"username field"}`
		default:
			content = `{"kind":"done","reason":"login complete"}`
		}
		resp := map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": content}}},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer uitarsSrv.Close()

	// ----- Mock OmniParser server -----
	// Always returns one big "Sign In" button at (100, 320)-(200,
	// 360). The UI-TARS mock clicks at (130, 335) which is inside
	// that rect, so the Grounder snaps the coord to the button
	// center (150, 340).
	omniSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/parse" {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := map[string]any{
			"width":  1920,
			"height": 1080,
			"elements": []map[string]any{
				{
					"bbox":        []int{100, 320, 200, 360},
					"type":        "button",
					"text":        "Sign In",
					"interactive": true,
					"confidence":  0.95,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer omniSrv.Close()

	// ----- Wire the stack -----
	screenshot := fakeScreenshot(1920, 1080)
	shot := &e2eScreenshotter{img: screenshot}
	uitarsC := uitars.New(uitarsSrv.URL)
	omniC := omniparser.New(omniSrv.URL)

	// SGLang guard wraps UI-TARS with schema validation. Schema
	// allows only Click / Type / Done for this test — if UI-TARS
	// hallucinated a Scroll, the guard would reject it.
	guard := &sglang.Guard{
		Actor:        uitarsC,
		Schema:       sglang.Schema{AllowedKinds: []action.Kind{action.KindClick, action.KindType, action.KindDone}},
		MaxRetries:   2,
	}

	grounder := &ground.Grounder{
		Actor:         guard,
		Detector:      omniC,
		SnapToNearest: true,
	}

	var executor e2eExecutor
	runner := &graph.Runner{
		Screenshotter: shot,
		Resolver:      grounder,
		Executor:      &executor,
		MaxSteps:      10,
	}

	// ----- Run -----
	result, err := runner.Run(context.Background(), "Log in as admin")
	if err != nil {
		t.Fatalf("Runner.Run: %v", err)
	}

	// ----- Assertions -----
	if !result.Done {
		t.Fatalf("expected Done termination, got %+v", result)
	}
	if result.Steps != 3 {
		t.Fatalf("Steps = %d, want 3 (click + type + done)", result.Steps)
	}
	if len(executor.executed) != 2 {
		t.Fatalf("executor saw %d actions, want 2 (Done excluded)", len(executor.executed))
	}

	// The click should be grounded to (150, 340) — OmniParser
	// button center.
	click := executor.executed[0]
	if click.Kind != action.KindClick {
		t.Fatalf("first action = %v, want click", click.Kind)
	}
	if click.X != 150 || click.Y != 340 {
		t.Fatalf("grounded click coords = (%d, %d), want (150, 340) — grounder failed to snap",
			click.X, click.Y)
	}
	// The Reason should contain both UI-TARS's original reason AND
	// the grounding note.
	if !strings.Contains(click.Reason, "Sign-In button") {
		t.Errorf("original reason lost: %q", click.Reason)
	}
	if !strings.Contains(click.Reason, "grounded to element center") {
		t.Errorf("grounding annotation missing: %q", click.Reason)
	}

	// The type action should pass through unchanged (no grounding
	// for type actions — only click / swipe get coord-snapped).
	typ := executor.executed[1]
	if typ.Kind != action.KindType || typ.Text != "admin" {
		t.Fatalf("second action = %+v, want type 'admin'", typ)
	}

	// UI-TARS should have been hit exactly 3 times (one per Runner
	// step; no SGLang retries since all responses are valid).
	if uitarsCalls.Load() != 3 {
		t.Fatalf("UI-TARS called %d times, want 3", uitarsCalls.Load())
	}
}

// TestAgentStack_SchemaRetryFlow proves sglang.Guard catches a
// VLM-emitted Action that violates the schema and retries —
// demonstrating the outer-loop safety net when UI-TARS
// hallucinates.
func TestAgentStack_SchemaRetryFlow(t *testing.T) {
	// UI-TARS mock: first response is an invalid Scroll (not in
	// AllowedKinds), second is a valid Click.
	var call atomic.Int32
	uitarsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := call.Add(1)
		var content string
		switch n {
		case 1:
			content = `{"kind":"scroll","dx":0,"dy":-100,"reason":"wrong kind"}`
		case 2:
			content = `{"kind":"click","x":10,"y":10,"reason":"corrected"}`
		default:
			content = `{"kind":"done","reason":"end"}`
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{"message": map[string]any{"content": content}}},
		})
	}))
	defer uitarsSrv.Close()

	shot := &e2eScreenshotter{img: fakeScreenshot(100, 100)}
	guard := &sglang.Guard{
		Actor:      uitars.New(uitarsSrv.URL),
		Schema:     sglang.Schema{AllowedKinds: []action.Kind{action.KindClick, action.KindDone}},
		MaxRetries: 3,
	}
	// No Detector — Grounder bypasses grounding for a bare Actor path.
	grounder := &ground.Grounder{Actor: guard}
	var exec e2eExecutor
	runner := &graph.Runner{
		Screenshotter: shot,
		Resolver:      grounder,
		Executor:      &exec,
		MaxSteps:      5,
	}
	_, err := runner.Run(context.Background(), "test")
	if err != nil {
		t.Fatalf("Runner: %v", err)
	}
	// UI-TARS called 3 times: step-1 retry-pair (invalid + valid)
	// produced step 1's executed click; step 2 was the Done.
	if call.Load() != 3 {
		t.Fatalf("UI-TARS called %d times, want 3", call.Load())
	}
	// Executor saw exactly 1 action (the corrected click); Done
	// was recorded but not dispatched.
	if len(exec.executed) != 1 {
		t.Fatalf("executor saw %d actions, want 1", len(exec.executed))
	}
}

// ---------------------------------------------------------------------------
// Fakes — local to this test file so the e2e test is self-contained.
// ---------------------------------------------------------------------------

type e2eScreenshotter struct {
	img image.Image
}

func (s *e2eScreenshotter) Screenshot(ctx context.Context) (image.Image, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.img, nil
}

type e2eExecutor struct {
	executed []action.Action
}

func (e *e2eExecutor) Execute(ctx context.Context, a action.Action) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	e.executed = append(e.executed, a)
	return nil
}

// fakeScreenshot produces a deterministic gradient-looking image
// of the given dimensions. Content doesn't matter for this test —
// UI-TARS + OmniParser are mocked and don't actually inspect
// pixels; we just need a valid image.Image to thread through the
// call chain.
func fakeScreenshot(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x & 0xFF), uint8(y & 0xFF), 128, 255})
		}
	}
	return img
}

// Tiny sanity check — confirms the fakeScreenshot helper itself
// works, so downstream test failures point at the stack logic
// rather than fixture synthesis.
func TestFakeScreenshot_SanityCheck(t *testing.T) {
	img := fakeScreenshot(8, 8)
	if b := img.Bounds(); b.Dx() != 8 || b.Dy() != 8 {
		t.Fatalf("dims = %v", b)
	}
	// Force a type-assertion to ensure the concrete type is what
	// downstream test code expects.
	rgba, ok := img.(*image.RGBA)
	if !ok || rgba == nil {
		t.Fatal("expected *image.RGBA")
	}
	// Pixel at (5, 3) should be (5, 3, 128, 255).
	got := rgba.RGBAAt(5, 3)
	want := color.RGBA{5, 3, 128, 255}
	if got != want {
		t.Fatalf("pixel = %v, want %v", got, want)
	}
}

// A final asserter confirming the fmt import is exercised — keeps
// fmt in the import block unconditionally referenced by test
// code even if future refactors remove it from the main assertion
// paths above.
func TestAgentStack_SanityFmtUsed(t *testing.T) {
	s := fmt.Sprintf("x=%d", 42)
	if s != "x=42" {
		t.Fatalf("fmt broken: %q", s)
	}
}
