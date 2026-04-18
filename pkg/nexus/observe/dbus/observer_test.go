// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package dbus

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
		{Kind: contracts.EventKindDBus, Timestamp: now},
		{Kind: contracts.EventKindDBus, Timestamp: now.Add(time.Millisecond)},
	}}
	defer withMock(t, mock)()

	obs, err := Open(context.Background(), observe.Config{BufferSize: 16})
	require.NoError(t, err)
	require.NoError(t, obs.Start(context.Background(), contracts.Target{ProcessName: "test"}))

	var got []contracts.Event
	for e := range obs.Events() {
		got = append(got, e)
	}
	require.Len(t, got, 2)
	require.Equal(t, contracts.EventKindDBus, got[0].Kind)
	require.NoError(t, obs.Stop())
}

func TestObserver_FactoryRegisteredInInit(t *testing.T) {
	kinds := observe.Kinds()
	found := false
	for _, k := range kinds {
		if k == "dbus" {
			found = true
			break
		}
	}
	require.True(t, found, "dbus kind must be registered via init()")
}

func TestObserver_ProductionReturnsErrNotWired(t *testing.T) {
	// Force the stub so this test is deterministic regardless of whether a
	// real D-Bus session bus is reachable in the test environment.
	t.Setenv("HELIXQA_OBSERVE_DBUS_STUB", "1")

	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "proc"})
	require.ErrorIs(t, err, ErrNotWired)
}

// ---------------------------------------------------------------------------
// P4.5 new tests
// ---------------------------------------------------------------------------

// TestConnect_MissingEnv_ReturnsErrNotWired — when DBUS_SESSION_BUS_ADDRESS
// is absent, Start must return ErrNotWired without panicking.
func TestConnect_MissingEnv_ReturnsErrNotWired(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "")
	t.Setenv("HELIXQA_OBSERVE_DBUS_STUB", "")

	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "proc"})
	require.ErrorIs(t, err, ErrNotWired)
}

// TestStubEnv_ForcesErrNotWired — HELIXQA_OBSERVE_DBUS_STUB=1 must force
// ErrNotWired regardless of whether a session bus is available.
func TestStubEnv_ForcesErrNotWired(t *testing.T) {
	t.Setenv("HELIXQA_OBSERVE_DBUS_STUB", "1")

	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "proc"})
	require.ErrorIs(t, err, ErrNotWired)
}

// TestSignalToEvent_Mapping — signalToEvent must translate a synthetic
// dbus.Signal into a contracts.Event with the correct Kind and Payload
// fields. No real bus connection is needed.
func TestSignalToEvent_Mapping(t *testing.T) {
	sig := &dbus.Signal{
		Sender: ":1.42",
		Path:   dbus.ObjectPath("/org/example/Foo"),
		Name:   "org.example.Foo.Bar",
		Body:   []any{"hello", int32(99)},
	}

	ev := signalToEvent(sig)

	require.Equal(t, contracts.EventKindDBus, ev.Kind)
	require.False(t, ev.Timestamp.IsZero(), "Timestamp must be set")
	require.Equal(t, ":1.42", ev.Payload["sender"])
	require.Equal(t, "/org/example/Foo", ev.Payload["path"])
	require.Equal(t, "org.example.Foo.Bar", ev.Payload["name"])
	body, ok := ev.Payload["body"]
	require.True(t, ok, "body key must be present when Signal.Body is non-empty")
	require.Equal(t, []any{"hello", int32(99)}, body)
}
