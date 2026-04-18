// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ax_tree

import (
	"context"
	"testing"
	"time"

	dbus "github.com/godbus/dbus/v5"
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
		{Kind: contracts.EventKindAXTree, Timestamp: now},
		{Kind: contracts.EventKindAXTree, Timestamp: now.Add(time.Millisecond)},
	}}
	defer withMock(t, mock)()

	obs, err := Open(context.Background(), observe.Config{BufferSize: 16})
	require.NoError(t, err)
	require.NoError(t, obs.Start(context.Background(), contracts.Target{ProcessName: "test-app"}))

	var got []contracts.Event
	for e := range obs.Events() {
		got = append(got, e)
	}
	require.Len(t, got, 2)
	require.Equal(t, contracts.EventKindAXTree, got[0].Kind)
	require.NoError(t, obs.Stop())
}

func TestObserver_FactoryRegisteredInInit(t *testing.T) {
	kinds := observe.Kinds()
	found := false
	for _, k := range kinds {
		if k == "ax_tree" {
			found = true
			break
		}
	}
	require.True(t, found, "ax_tree kind must be registered via init()")
}

func TestObserver_ProductionReturnsErrNotWired(t *testing.T) {
	// Force the stub so this test is deterministic regardless of whether a
	// real AT-SPI2 accessibility bus is reachable in the test environment.
	t.Setenv("HELIXQA_OBSERVE_AX_STUB", "1")

	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "app"})
	require.ErrorIs(t, err, ErrNotWired)
}

// ---------------------------------------------------------------------------
// P4.5 new tests
// ---------------------------------------------------------------------------

// TestResolveA11yAddress_MissingBus_ReturnsErrNotWired — when
// DBUS_SESSION_BUS_ADDRESS is absent, Start must return ErrNotWired without
// panicking.
func TestResolveA11yAddress_MissingBus_ReturnsErrNotWired(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("HELIXQA_OBSERVE_AX_STUB", "")

	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "app"})
	require.ErrorIs(t, err, ErrNotWired)
}

// TestStubEnv_ForcesErrNotWired — HELIXQA_OBSERVE_AX_STUB=1 must force
// ErrNotWired regardless of whether an accessibility bus is available.
func TestStubEnv_ForcesErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_OBSERVE_AX_STUB", "1")

	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "app"})
	require.ErrorIs(t, err, ErrNotWired)
}

// TestSignalToAXEvent_Mapping — signalToAXEvent must translate a synthetic
// dbus.Signal into a contracts.Event with Kind=EventKindAXTree and Payload
// containing the member (signal name) and path strings. No real bus
// connection is needed.
func TestSignalToAXEvent_Mapping(t *testing.T) {
	sig := &dbus.Signal{
		Sender: ":1.77",
		Path:   dbus.ObjectPath("/org/a11y/atspi/accessible/0"),
		Name:   "org.a11y.atspi.Event.Object.StateChanged",
		Body:   []any{"focused", int32(1), int32(0)},
	}

	ev := signalToAXEvent(sig)

	require.Equal(t, contracts.EventKindAXTree, ev.Kind)
	require.False(t, ev.Timestamp.IsZero(), "Timestamp must be set")
	require.Equal(t, ":1.77", ev.Payload["sender"])
	require.Equal(t, "/org/a11y/atspi/accessible/0", ev.Payload["path"])
	require.Equal(t, "org.a11y.atspi.Event.Object.StateChanged", ev.Payload["member"])
	body, ok := ev.Payload["body"]
	require.True(t, ok, "body key must be present when Signal.Body is non-empty")
	require.Equal(t, []any{"focused", int32(1), int32(0)}, body)
}
