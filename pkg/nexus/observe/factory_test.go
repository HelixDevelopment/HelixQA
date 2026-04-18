// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package observe

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// fakeObserver is a minimal no-op Observer used by factory tests.
type fakeObserver struct{ kind string }

func (f *fakeObserver) Start(_ context.Context, _ contracts.Target) error { return nil }
func (f *fakeObserver) Events() <-chan contracts.Event {
	ch := make(chan contracts.Event)
	close(ch)
	return ch
}
func (f *fakeObserver) Snapshot(_ time.Time, _ time.Duration) ([]contracts.Event, error) {
	return nil, nil
}
func (f *fakeObserver) Stop() error { return nil }

func TestFactory_RegisterAndOpen(t *testing.T) {
	Register("test-observe-kind", func(_ context.Context, _ Config) (contracts.Observer, error) {
		return &fakeObserver{kind: "test-observe-kind"}, nil
	})
	got, err := Open(context.Background(), "test-observe-kind", Config{})
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestFactory_UnknownKind(t *testing.T) {
	_, err := Open(context.Background(), "observe-does-not-exist", Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "observe-does-not-exist")
}

func TestFactory_Kinds(t *testing.T) {
	Register("kinds-probe-observe", func(_ context.Context, _ Config) (contracts.Observer, error) {
		return &fakeObserver{kind: "kinds-probe-observe"}, nil
	})
	kinds := Kinds()
	found := false
	for _, k := range kinds {
		if k == "kinds-probe-observe" {
			found = true
			break
		}
	}
	require.True(t, found, "Kinds() should include registered kind")
}

func TestFactory_DefaultBufferSize(t *testing.T) {
	var capturedCfg Config
	Register("buf-size-probe", func(_ context.Context, cfg Config) (contracts.Observer, error) {
		capturedCfg = cfg
		return &fakeObserver{}, nil
	})
	_, err := Open(context.Background(), "buf-size-probe", Config{BufferSize: 0})
	require.NoError(t, err)
	require.Equal(t, 1024, capturedCfg.BufferSize, "zero BufferSize must default to 1024")
}

func TestFactory_ConcurrentRegister(t *testing.T) {
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			Register("race-kind-observe", func(_ context.Context, _ Config) (contracts.Observer, error) {
				return &fakeObserver{kind: "race-kind-observe"}, nil
			})
			_, _ = Open(context.Background(), "race-kind-observe", Config{})
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
