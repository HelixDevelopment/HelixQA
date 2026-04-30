// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package interact

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// fakeInteractor is a minimal no-op Interactor used by factory tests.
type fakeInteractor struct{ kind string }

func (f *fakeInteractor) Click(_ context.Context, _ contracts.Point, _ contracts.ClickOptions) error {
	return nil
}
func (f *fakeInteractor) Type(_ context.Context, _ string, _ contracts.TypeOptions) error {
	return nil
}
func (f *fakeInteractor) Scroll(_ context.Context, _ contracts.Point, _, _ float64) error {
	return nil
}
func (f *fakeInteractor) Key(_ context.Context, _ contracts.KeyCode, _ contracts.KeyOptions) error {
	return nil
}
func (f *fakeInteractor) Drag(_ context.Context, _, _ contracts.Point, _ contracts.DragOptions) error {
	return nil
}

func TestFactory_RegisterAndOpen(t *testing.T) {
	Register("test-interact-kind", func(_ context.Context, _ Config) (contracts.Interactor, error) {
		return &fakeInteractor{kind: "test-interact-kind"}, nil
	})
	got, err := Open(context.Background(), "test-interact-kind", Config{})
	require.NoError(t, err)
	require.NotNil(t, got)
}

func TestFactory_UnknownKind(t *testing.T) {
	_, err := Open(context.Background(), "interact-does-not-exist", Config{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "interact-does-not-exist")
}

func TestFactory_Kinds(t *testing.T) {
	Register("kinds-probe-interact", func(_ context.Context, _ Config) (contracts.Interactor, error) {
		return &fakeInteractor{kind: "kinds-probe-interact"}, nil
	})
	kinds := Kinds()
	found := false
	for _, k := range kinds {
		if k == "kinds-probe-interact" {
			found = true
			break
		}
	}
	require.True(t, found, "Kinds() should include registered kind")
}

func TestFactory_ConcurrentRegister(t *testing.T) {
	// bluff-scan: no-assert-ok (concurrency test — go test -race catches data races; absence of panic == correctness)
	done := make(chan struct{})
	for i := 0; i < 50; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			Register("race-kind-interact", func(_ context.Context, _ Config) (contracts.Interactor, error) {
				return &fakeInteractor{kind: "race-kind-interact"}, nil
			})
			_, _ = Open(context.Background(), "race-kind-interact", Config{})
		}()
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
