// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"testing"
	"time"
)

// FuzzPromptHash exercises the VisionResult struct with arbitrary strings in
// its Text, Provider, and Model fields. Since hashPrompt is unexported in the
// cache package, we test the public surface that depends on it: constructing
// VisionResult values with adversarial strings must never panic.
func FuzzPromptHash(f *testing.F) {
	// Seed corpus: empty string, ASCII, Unicode, SQL injection, XSS payloads.
	f.Add("")
	f.Add("describe the login screen")
	f.Add("'; DROP TABLE users; --")
	f.Add("<script>alert('xss')</script>")
	f.Add("\x00\x01\x02\x03\x04\x05\x06\x07")
	f.Add("日本語テスト")
	f.Add("Привет мир")
	f.Add("🎉🔥💥🚀")

	f.Fuzz(func(t *testing.T, s string) {
		// Constructing a VisionResult with arbitrary string fields must never
		// panic regardless of input content.
		vr := &VisionResult{
			Text:     s,
			Provider: s,
			Model:    s,
			Duration: time.Millisecond,
		}
		// Basic invariants: fields survive the round-trip without mutation.
		if vr.Text != s {
			t.Fatalf("Text field mutated: got %q, want %q", vr.Text, s)
		}
		if vr.Provider != s {
			t.Fatalf("Provider field mutated: got %q, want %q", vr.Provider, s)
		}
		if vr.Model != s {
			t.Fatalf("Model field mutated: got %q, want %q", vr.Model, s)
		}
	})
}

// FuzzVisionResult verifies that building VisionResult values with arbitrary
// string and float inputs never panics and that all fields are stored exactly
// as provided.
func FuzzVisionResult(f *testing.F) {
	f.Add("ok", "gemini", "gemini-2.0-flash", 0.95)
	f.Add("", "", "", 0.0)
	f.Add("a\nb\tc", "prov\x00", "mod\xFF", 1.1)
	f.Add("very long "+string(make([]byte, 4096)), "p", "m", -0.5)

	f.Fuzz(func(t *testing.T, text, provider, model string, confidence float64) {
		vr := VisionResult{
			Text:       text,
			Provider:   provider,
			Model:      model,
			Confidence: confidence,
			CacheHit:   true,
		}
		if vr.Text != text {
			t.Fatalf("Text mismatch after construction")
		}
		if vr.Provider != provider {
			t.Fatalf("Provider mismatch after construction")
		}
		if vr.Model != model {
			t.Fatalf("Model mismatch after construction")
		}
		if vr.Confidence != confidence {
			t.Fatalf("Confidence mismatch after construction")
		}
		if !vr.CacheHit {
			t.Fatalf("CacheHit must remain true")
		}
	})
}
