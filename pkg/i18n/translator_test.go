// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package i18n

import (
	"context"
	"testing"
)

// TestNoopTranslatorT verifies that NoopTranslator.T returns the
// messageID verbatim and never errors. This is the contract every
// migrated call site depends on to preserve pre-migration English
// output when no real backend has been wired.
func TestNoopTranslatorT(t *testing.T) {
	tr := NoopTranslator{}
	ctx := context.Background()

	const id = "helixqa_cli_banner"
	got, err := tr.T(ctx, id, nil)
	if err != nil {
		t.Fatalf("NoopTranslator.T returned error: %v", err)
	}
	if got != id {
		t.Fatalf("NoopTranslator.T returned %q, want messageID %q (verbatim contract)", got, id)
	}

	// With template data — must STILL return verbatim.
	got, err = tr.T(ctx, id, map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("NoopTranslator.T with templateData returned error: %v", err)
	}
	if got != id {
		t.Fatalf("NoopTranslator.T with templateData returned %q, want %q", got, id)
	}
}

// TestNoopTranslatorTPlural verifies that NoopTranslator.TPlural
// returns the messageID verbatim across count values.
func TestNoopTranslatorTPlural(t *testing.T) {
	tr := NoopTranslator{}
	ctx := context.Background()

	const id = "helixqa_run_banks_flag_usage"
	for _, count := range []int{0, 1, 2, 5, 100} {
		got, err := tr.TPlural(ctx, id, count, nil)
		if err != nil {
			t.Fatalf("NoopTranslator.TPlural(count=%d) returned error: %v", count, err)
		}
		if got != id {
			t.Fatalf("NoopTranslator.TPlural(count=%d) returned %q, want %q", count, got, id)
		}
	}
}

// fakeTranslator is the test-only backend that returns a sentinel
// for every messageID, used by call-site migration tests to assert
// the seam is exercised (rather than the pre-migration literal
// being printed directly).
type fakeTranslator struct{}

func (fakeTranslator) T(_ context.Context, messageID string, _ map[string]any) (string, error) {
	return "<TRANSLATED:" + messageID + ">", nil
}

func (fakeTranslator) TPlural(_ context.Context, messageID string, _ int, _ map[string]any) (string, error) {
	return "<TRANSLATED-PLURAL:" + messageID + ">", nil
}

// TestSetTranslatorActivation verifies SetTranslator installs the
// backend and ActiveTranslator returns it. After ResetForTest the
// default NoopTranslator MUST be restored. This is the global-
// seam contract every test that wants to intercept user-facing
// output relies on.
func TestSetTranslatorActivation(t *testing.T) {
	t.Cleanup(ResetForTest)

	// Default is NoopTranslator.
	if _, ok := ActiveTranslator().(NoopTranslator); !ok {
		t.Fatalf("ActiveTranslator default = %T, want NoopTranslator", ActiveTranslator())
	}

	SetTranslator(fakeTranslator{})
	got, err := ActiveTranslator().T(context.Background(), "helixqa_cli_banner", nil)
	if err != nil {
		t.Fatalf("T returned error after SetTranslator: %v", err)
	}
	const want = "<TRANSLATED:helixqa_cli_banner>"
	if got != want {
		t.Fatalf("T after SetTranslator = %q, want sentinel %q", got, want)
	}

	// nil SetTranslator MUST be a no-op (existing backend retained).
	SetTranslator(nil)
	got, _ = ActiveTranslator().T(context.Background(), "helixqa_cli_banner", nil)
	if got != want {
		t.Fatalf("after SetTranslator(nil), expected fakeTranslator retained; got %q", got)
	}

	ResetForTest()
	if _, ok := ActiveTranslator().(NoopTranslator); !ok {
		t.Fatalf("after ResetForTest, ActiveTranslator = %T, want NoopTranslator", ActiveTranslator())
	}
}
