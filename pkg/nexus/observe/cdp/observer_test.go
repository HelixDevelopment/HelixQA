// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cdp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/observe"
)

// ---------------------------------------------------------------------------
// Mock producer (keeps existing mock-producer tests green)
// ---------------------------------------------------------------------------

type mockProducer struct {
	events []contracts.Event
}

func (m *mockProducer) Produce(
	_ context.Context,
	_ contracts.Target,
	out chan<- contracts.Event,
	stopCh <-chan struct{},
) error {
	for _, e := range m.events {
		select {
		case out <- e:
		case <-stopCh:
			return nil
		}
	}
	return nil
}

func withMock(t *testing.T, mock producer) func() {
	t.Helper()
	orig := newProducer
	newProducer = mock
	return func() { newProducer = orig }
}

// ---------------------------------------------------------------------------
// Existing tests (must stay green)
// ---------------------------------------------------------------------------

func TestObserver_MockProducesEvents(t *testing.T) {
	now := time.Now()
	mock := &mockProducer{events: []contracts.Event{
		{Kind: contracts.EventKindCDP, Timestamp: now},
	}}
	defer withMock(t, mock)()

	obs, err := Open(context.Background(), observe.Config{BufferSize: 16})
	require.NoError(t, err)
	require.NoError(t, obs.Start(context.Background(), contracts.Target{ProcessName: "chromium"}))

	var got []contracts.Event
	for e := range obs.Events() {
		got = append(got, e)
	}
	require.Len(t, got, 1)
	require.Equal(t, contracts.EventKindCDP, got[0].Kind)
	require.NoError(t, obs.Stop())
}

func TestObserver_FactoryRegisteredInInit(t *testing.T) {
	kinds := observe.Kinds()
	found := false
	for _, k := range kinds {
		if k == "cdp" {
			found = true
			break
		}
	}
	require.True(t, found, "cdp kind must be registered via init()")
}

func TestObserver_ProductionReturnsErrNotWired(t *testing.T) {
	// Force the stub so this test is deterministic regardless of whether
	// chromium/chrome is installed in the test environment.
	t.Setenv("HELIXQA_OBSERVE_CDP_STUB", "1")

	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "chromium"})
	require.ErrorIs(t, err, ErrNotWired)
}

// ---------------------------------------------------------------------------
// P4.5 new tests
// ---------------------------------------------------------------------------

// TestResolveBrowser_MissingPath_Errors — when no browser candidate is on
// PATH, resolveBrowser must return a non-nil error (no panic).
func TestResolveBrowser_MissingPath_Errors(t *testing.T) {
	// Override BrowserCandidates to a name that will never exist on PATH.
	// Set before any goroutine reads it (no concurrent access here).
	orig := BrowserCandidates
	BrowserCandidates = []string{"__no_browser_here__"}
	t.Cleanup(func() { BrowserCandidates = orig })

	_, err := resolveBrowser()
	require.Error(t, err)
}

// TestStubEnv_ForcesErrNotWired — HELIXQA_OBSERVE_CDP_STUB=1 must force
// ErrNotWired regardless of whether a browser is installed.
func TestStubEnv_ForcesErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_OBSERVE_CDP_STUB", "1")

	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "chromium"})
	require.ErrorIs(t, err, ErrNotWired)
}

// TestCDPEventToEvent_NetworkResponseReceived — networkResponseToEvent must
// translate a synthetic *network.EventResponseReceived into a contracts.Event
// with the correct Kind, URL, and status. No real browser is needed.
func TestCDPEventToEvent_NetworkResponseReceived(t *testing.T) {
	resp := &network.Response{
		URL:    "https://example.com/api/v1/test",
		Status: 200,
	}
	ev := &network.EventResponseReceived{
		Response: resp,
	}

	got := networkResponseToEvent(ev)

	require.Equal(t, contracts.EventKindCDP, got.Kind)
	require.False(t, got.Timestamp.IsZero(), "Timestamp must be set")
	require.Equal(t, "https://example.com/api/v1/test", got.Payload["url"])

	// Status is stored as int64 by cdproto — accept both int64 and json.Number.
	switch v := got.Payload["status"].(type) {
	case int64:
		require.Equal(t, int64(200), v)
	case json.Number:
		n, err := v.Int64()
		require.NoError(t, err)
		require.Equal(t, int64(200), n)
	default:
		t.Fatalf("unexpected status type %T", got.Payload["status"])
	}
}
