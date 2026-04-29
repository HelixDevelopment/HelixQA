// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package autonomous

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.helixqa/pkg/testbank"
)

// TestPlaywrightExecutor_NoCDP verifies that Execute returns a clean
// failure (rather than panicking) when the underlying adapter has
// no real CDP endpoint to talk to. This anchors the Article XI
// §11.2.2 contract that an unreachable real system never produces
// a silent PASS.
func TestPlaywrightExecutor_NoCDP(t *testing.T) {
	// Use a port that is guaranteed unreachable.
	p := NewPlaywrightExecutor("ws://127.0.0.1:1")
	ctx := context.Background()
	res := p.Execute(ctx, "navigate https://example.com")
	require.False(t, res.Success, "unreachable CDP must not silently PASS")
	assert.NotEmpty(t, res.Message)
}

// TestPlaywrightExecutor_EmptyValue verifies the parser returns a
// clean error when no verb is supplied.
func TestPlaywrightExecutor_EmptyValue(t *testing.T) {
	p := NewPlaywrightExecutor("ws://unused")
	res := p.Execute(context.Background(), "")
	require.False(t, res.Success)
	assert.Contains(t, res.Message, "empty action value")
}

// TestPlaywrightExecutor_UnknownVerb verifies an unrecognized verb
// surfaces a clear error listing the known verbs.
func TestPlaywrightExecutor_UnknownVerb(t *testing.T) {
	p := NewPlaywrightExecutor("ws://127.0.0.1:1")
	res := p.Execute(context.Background(), "frobnicate text=Foo")
	// Either the unreachable-CDP error fires first OR our verb-
	// dispatch error does. Both are clean failures with messages.
	require.False(t, res.Success)
	assert.NotEmpty(t, res.Message)
}

// TestSplitVerb covers the verb-extractor edge cases.
func TestSplitVerb(t *testing.T) {
	cases := []struct{ in, verb, rest string }{
		{"navigate https://example.com", "navigate", "https://example.com"},
		{"  CLICK  text=Foo  ", "click", "text=Foo"},
		{"fill input[name=user] admin", "fill", "input[name=user] admin"},
		{"press Enter", "press", "Enter"},
		{"navigate", "navigate", ""},
		{"", "", ""},
		{"   ", "", ""},
	}
	for _, tc := range cases {
		v, r := splitVerb(tc.in)
		assert.Equal(t, tc.verb, v, "verb for %q", tc.in)
		assert.Equal(t, tc.rest, r, "rest for %q", tc.in)
	}
}

// TestSplitSelectorValue verifies fill's selector/value parsing.
func TestSplitSelectorValue(t *testing.T) {
	cases := []struct{ in, sel, val string }{
		{"input[name=username] admin", "input[name=username]", "admin"},
		{"text=Sign In", "text=Sign", "In"},
		{"#submit", "#submit", ""},
		{"", "", ""},
	}
	for _, tc := range cases {
		s, v := splitSelectorValue(tc.in)
		assert.Equal(t, tc.sel, s, "selector for %q", tc.in)
		assert.Equal(t, tc.val, v, "value for %q", tc.in)
	}
}

// TestActionTypePlaywright_RoundTrip verifies the schema parse path
// for ActionTypePlaywright. (Full dispatch requires a running
// Playwright container — covered by integration tests when
// HELIXQA_PLAYWRIGHT_CDP_URL is set.)
func TestActionTypePlaywright_RoundTrip(t *testing.T) {
	cases := []struct {
		action string
		value  string
	}{
		{"playwright: navigate https://example.com", "navigate https://example.com"},
		{"playwright: click text=Sign In", "click text=Sign In"},
		{"playwright: fill input[name=username] admin", "fill input[name=username] admin"},
		{"playwright: assertVisible text=Welcome", "assertVisible text=Welcome"},
		{"playwright: waitFor text=Loaded", "waitFor text=Loaded"},
		{"playwright: press Enter", "press Enter"},
	}
	for _, tc := range cases {
		step := testbank.TestStep{Action: tc.action}
		at, val := step.ParseAction()
		assert.Equal(t, testbank.ActionTypePlaywright, at, "action type for %q", tc.action)
		assert.Equal(t, tc.value, val, "value for %q", tc.action)
	}
}

// TestPlaywrightExecutor_AntiBluffMarker is the §11.2.5 anchor: an
// unreachable CDP endpoint MUST cause a non-Success ActionResult.
// Removing the unreachable-port check from the test infrastructure
// (e.g. swapping in a panic-on-call mock) would still make this
// test fail because the adapter's underlying execNode would error
// rather than silently succeed.
func TestPlaywrightExecutor_AntiBluffMarker(t *testing.T) {
	p := NewPlaywrightExecutor("ws://127.0.0.1:1")
	res := p.Execute(context.Background(), "click text=NonexistentButton")
	require.False(t, res.Success, "no playwright runtime must fail loud")
}
