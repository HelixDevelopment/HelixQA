// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"testing"

	"digital.vasic.helixqa/pkg/testbank"
)

// Fixture regression for the per-test opt-out.
//
// Prior to this commit, the foreground-drift guard emitted a
// CRITICAL finding for every test that navigated away from the
// target package — including `tv-search-voice` and `tv-channel-*`
// tests that INTENTIONALLY exercise the voice overlay / launcher
// channels. Run5 produced 8 such false-positive critical tickets.
// Setting `allow_foreground_leave: true` on those test cases now
// makes ensureAppForeground a no-op for them, while every other
// test continues to enforce the guard.

func TestTestCase_AllowForegroundLeave_DefaultsFalse(t *testing.T) {
	tc := testbank.TestCase{ID: "demo"}
	if tc.AllowForegroundLeave {
		t.Fatalf("AllowForegroundLeave should default to false, got true")
	}
}

func TestTestCase_AllowForegroundLeave_FieldRoundTrips(t *testing.T) {
	tc := testbank.TestCase{ID: "demo", AllowForegroundLeave: true}
	if !tc.AllowForegroundLeave {
		t.Fatalf("AllowForegroundLeave should be true after explicit set")
	}
}
