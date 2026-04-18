// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package ax_tree

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/observe"
)

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
	obs, err := Open(context.Background(), observe.Config{})
	require.NoError(t, err)
	err = obs.Start(context.Background(), contracts.Target{ProcessName: "app"})
	require.ErrorIs(t, err, ErrNotWired)
}
