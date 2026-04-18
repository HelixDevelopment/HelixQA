// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ld_preload

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/observe"
)

// ---------------------------------------------------------------------------
// Mock producer (keeps existing mock-producer tests green)
// ---------------------------------------------------------------------------

// mockProducer emits a configurable number of events then stops.
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
		{Kind: contracts.EventKindHook, Timestamp: now},
		{Kind: contracts.EventKindHook, Timestamp: now.Add(time.Millisecond)},
	}}
	defer withMock(t, mock)()

	obs, err := Open(context.Background(), observe.Config{BufferSize: 16})
	require.NoError(t, err)

	require.NoError(t, obs.Start(context.Background(), contracts.Target{ProcessName: "test"}))
	// Drain events.
	var got []contracts.Event
	for e := range obs.Events() {
		got = append(got, e)
	}
	require.Len(t, got, 2)
	require.Equal(t, contracts.EventKindHook, got[0].Kind)
	require.NoError(t, obs.Stop())
}

func TestObserver_FactoryRegisteredInInit(t *testing.T) {
	kinds := observe.Kinds()
	found := false
	for _, k := range kinds {
		if k == "ld_preload" {
			found = true
			break
		}
	}
	require.True(t, found, "ld_preload kind must be registered via init()")
}

// TestObserver_ProductionReturnsErrNotWired — with the kill-switch active and
// no mock injected, Start must return ErrNotWired.
func TestObserver_ProductionReturnsErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_OBSERVE_LDPRELOAD_STUB", "1")

	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "proc"})
	require.ErrorIs(t, err, ErrNotWired)
}

// ---------------------------------------------------------------------------
// P4.5 new tests
// ---------------------------------------------------------------------------

// TestProduction_MissingShim_ReturnsErrNotWired — when neither
// target.Labels["shim_path"] nor HELIXQA_LD_SHIM is set the production
// observer must return ErrNotWired without panicking.
func TestProduction_MissingShim_ReturnsErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_OBSERVE_LDPRELOAD_STUB", "")
	t.Setenv("HELIXQA_LD_SHIM", "")

	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)

	// No shim_path label, no env — must degrade to ErrNotWired.
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "proc"})
	require.ErrorIs(t, err, ErrNotWired)
}

// TestStubEnv_ForcesErrNotWired — HELIXQA_OBSERVE_LDPRELOAD_STUB=1 must
// force ErrNotWired regardless of whether a valid shim file exists.
func TestStubEnv_ForcesErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_OBSERVE_LDPRELOAD_STUB", "1")
	// Even with a shim path set, the kill-switch must win.
	t.Setenv("HELIXQA_LD_SHIM", "/usr/lib/libdl.so.2")

	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "proc"})
	require.ErrorIs(t, err, ErrNotWired)
}

// TestParseShimLine_JSON — parseShimLine must decode a valid JSON record into
// a contracts.Event with EventKindHook, the correct Timestamp, and Payload
// fields "fn" and "arg".
func TestParseShimLine_JSON(t *testing.T) {
	// ts_ns = 1 second past the Unix epoch.
	line := []byte(`{"ts_ns":1000000000,"fn":"open","arg":"/etc/passwd"}`)

	ev, err := parseShimLine(line)
	require.NoError(t, err)

	require.Equal(t, contracts.EventKindHook, ev.Kind)

	wantTS := time.Unix(1, 0)
	require.Equal(t, wantTS, ev.Timestamp, "Timestamp must equal ts_ns converted to time.Time")

	require.Equal(t, "open", ev.Payload["fn"])
	require.Equal(t, "/etc/passwd", ev.Payload["arg"])
}

// TestParseShimLine_MalformedJSON — malformed input must return an error,
// not panic.
func TestParseShimLine_MalformedJSON(t *testing.T) {
	_, err := parseShimLine([]byte(`not json`))
	require.Error(t, err)
}
