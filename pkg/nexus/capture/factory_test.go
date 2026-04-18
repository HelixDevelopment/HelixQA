// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package capture

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

type fakeSource struct {
	name   string
	frames chan contracts.Frame
}

func (f *fakeSource) Name() string                                         { return f.name }
func (f *fakeSource) Start(context.Context, contracts.CaptureConfig) error { return nil }
func (f *fakeSource) Stop() error                                          { return nil }
func (f *fakeSource) Frames() <-chan contracts.Frame                       { return f.frames }
func (f *fakeSource) Stats() contracts.CaptureStats                        { return contracts.CaptureStats{} }
func (f *fakeSource) Close() error                                         { close(f.frames); return nil }

func TestFactory_RegisterAndOpen(t *testing.T) {
	// Scope with a local registry — factory uses a package-level
	// registry so guard against test interference with the kind name.
	Register("test-factory-kind", func(ctx context.Context, cfg contracts.CaptureConfig) (contracts.CaptureSource, error) {
		return &fakeSource{name: "test-factory-kind", frames: make(chan contracts.Frame, 1)}, nil
	})
	src, err := Open(context.Background(), "test-factory-kind", contracts.CaptureConfig{})
	require.NoError(t, err)
	require.NotNil(t, src)
	require.Equal(t, "test-factory-kind", src.Name())
	require.NotNil(t, src.Frames())
	require.NoError(t, src.Close())
}

func TestFactory_UnknownKind(t *testing.T) {
	_, err := Open(context.Background(), "does-not-exist", contracts.CaptureConfig{})
	require.Error(t, err)
}

func TestFactory_Kinds(t *testing.T) {
	Register("kinds-probe", func(ctx context.Context, cfg contracts.CaptureConfig) (contracts.CaptureSource, error) {
		return &fakeSource{name: "kinds-probe", frames: make(chan contracts.Frame, 1)}, nil
	})
	kinds := Kinds()
	found := false
	for _, k := range kinds {
		if k == "kinds-probe" {
			found = true
			break
		}
	}
	require.True(t, found, "Kinds() should include registered kind")
}

func TestFactory_ConcurrentRegister(t *testing.T) {
	// Sanity: Register + Open from multiple goroutines don't race under -race.
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func(i int) {
			defer func() { done <- struct{}{} }()
			kind := "race-kind"
			Register(kind, func(ctx context.Context, cfg contracts.CaptureConfig) (contracts.CaptureSource, error) {
				return &fakeSource{name: kind, frames: make(chan contracts.Frame, 1)}, nil
			})
			_, _ = Open(context.Background(), kind, contracts.CaptureConfig{})
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
