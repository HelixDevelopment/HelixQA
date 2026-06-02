// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package visionnav

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"digital.vasic.helixqa/pkg/llm"
)

// compile-time proof that any pkg/llm.Provider satisfies the local
// VisionDecider contract — so every LLM backend (anthropic, openai,
// google, ollama, ...) can drive the vision-nav loop without per-backend
// glue. If llm.Provider drifts, this fails to compile.
var _ VisionDecider = (llm.Provider)(nil)

// fakeVision is a unit-test fake VisionDecider (mocks permitted in unit
// tests only, §11.4.27).
type fakeVision struct {
	supports bool
	reply    string
	err      error
	gotImage []byte
	gotPromp string
}

func (f *fakeVision) Vision(_ context.Context, image []byte, prompt string) (*llm.Response, error) {
	f.gotImage = image
	f.gotPromp = prompt
	if f.err != nil {
		return nil, f.err
	}
	return &llm.Response{Content: f.reply, Model: "fake"}, nil
}
func (f *fakeVision) SupportsVision() bool { return f.supports }
func (f *fakeVision) Name() string         { return "fake" }

func TestNewLLMProvider_Validation(t *testing.T) {
	if _, err := NewLLMProvider(nil, []string{"g"}, ""); err == nil {
		t.Fatal("expected error for nil backend")
	}
	if _, err := NewLLMProvider(&fakeVision{supports: false}, []string{"g"}, ""); err == nil {
		t.Fatal("expected error for non-vision backend")
	}
	if _, err := NewLLMProvider(&fakeVision{supports: true}, nil, ""); err == nil {
		t.Fatal("expected error for no goals")
	}
	if _, err := NewLLMProvider(&fakeVision{supports: true}, []string{"  ", ""}, ""); err == nil {
		t.Fatal("expected error for only-blank goals")
	}
	p, err := NewLLMProvider(&fakeVision{supports: true}, []string{"Home"}, "tap x y")
	if err != nil {
		t.Fatalf("valid construction: %v", err)
	}
	if p.Name() != "llm:fake" {
		t.Fatalf("Name = %q, want llm:fake", p.Name())
	}
}

func TestLLMProvider_Decide_NoScreenshotYet(t *testing.T) {
	p, _ := NewLLMProvider(&fakeVision{supports: true}, []string{"Home"}, "")
	d, err := p.Decide(context.Background(), Observation{StepNumber: 1})
	if err != nil {
		t.Fatalf("Decide step1: %v", err)
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("step1 decision invalid: %v", err)
	}
	if d.Action != "launch" {
		t.Fatalf("step1 action = %q, want launch", d.Action)
	}
}

func TestLLMProvider_Decide_ParsesReply(t *testing.T) {
	dir := t.TempDir()
	shot := filepath.Join(dir, "frame.png")
	if err := os.WriteFile(shot, []byte("PNGBYTES"), 0o644); err != nil {
		t.Fatal(err)
	}
	fv := &fakeVision{
		supports: true,
		reply: "Looking at the home grid.\n" +
			"ACTION: tap 540 960\n" +
			"RATIONALE: the target tile is centered at that coordinate\n" +
			"EXPECT: needs-review\n",
	}
	p, _ := NewLLMProvider(fv, []string{"Now Playing"}, "tap x y | key KEYCODE | back")
	d, err := p.Decide(context.Background(), Observation{StepNumber: 2, LastImagePath: shot})
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if d.Action != "tap 540 960" {
		t.Fatalf("Action = %q", d.Action)
	}
	if d.Rationale == "" {
		t.Fatal("Rationale empty")
	}
	if d.ExpectedVerdict != "needs-review" {
		t.Fatalf("ExpectedVerdict = %q", d.ExpectedVerdict)
	}
	// Image bytes reached the backend, and the goal text reached the prompt.
	if string(fv.gotImage) != "PNGBYTES" {
		t.Fatalf("backend got image %q", fv.gotImage)
	}
	if !contains(fv.gotPromp, "Now Playing") {
		t.Fatal("prompt missing goal text")
	}
}

func TestLLMProvider_Decide_RejectsBluffReply(t *testing.T) {
	dir := t.TempDir()
	shot := filepath.Join(dir, "frame.png")
	_ = os.WriteFile(shot, []byte("PNGBYTES"), 0o644)

	cases := map[string]string{
		"no action":    "RATIONALE: did stuff\n",
		"no rationale": "ACTION: tap 1 2\n",
		"empty reply":  "",
		"prose only":   "I think you should look around a bit.",
	}
	for name, reply := range cases {
		t.Run(name, func(t *testing.T) {
			fv := &fakeVision{supports: true, reply: reply}
			p, _ := NewLLMProvider(fv, []string{"Home"}, "")
			if _, err := p.Decide(context.Background(), Observation{StepNumber: 2, LastImagePath: shot}); err == nil {
				t.Fatalf("expected bluff reply %q to be rejected", reply)
			}
		})
	}
}

func TestLLMProvider_Decide_BackendError(t *testing.T) {
	dir := t.TempDir()
	shot := filepath.Join(dir, "frame.png")
	_ = os.WriteFile(shot, []byte("PNGBYTES"), 0o644)
	fv := &fakeVision{supports: true, err: errors.New("rate limited")}
	p, _ := NewLLMProvider(fv, []string{"Home"}, "")
	if _, err := p.Decide(context.Background(), Observation{StepNumber: 2, LastImagePath: shot}); err == nil {
		t.Fatal("expected backend error to propagate")
	}
}

func TestLLMProvider_Decide_MissingScreenshotFile(t *testing.T) {
	fv := &fakeVision{supports: true, reply: "ACTION: back\nRATIONALE: go up\n"}
	p, _ := NewLLMProvider(fv, []string{"Home"}, "")
	if _, err := p.Decide(context.Background(), Observation{StepNumber: 2, LastImagePath: "/no/such/file.png"}); err == nil {
		t.Fatal("expected error for missing screenshot file")
	}
}

func TestParseDecision_CaseInsensitiveFields(t *testing.T) {
	d, err := parseDecision("action: home\nrationale: reset to top\nexpect: pass\n")
	if err != nil {
		t.Fatalf("parseDecision: %v", err)
	}
	if d.Action != "home" || d.Rationale != "reset to top" || d.ExpectedVerdict != "pass" {
		t.Fatalf("parsed = %+v", d)
	}
}

// contains is a tiny strings.Contains to avoid an extra import in tests.
func contains(haystack, needle string) bool {
	return containsSubstring(haystack, needle)
}
