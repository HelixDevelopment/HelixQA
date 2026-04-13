// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"context"
	"fmt"
	"image"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubProvider is a minimal VisionProvider implementation used only in tests.
type stubProvider struct {
	name string
}

func (s *stubProvider) Name() string { return s.name }

func (s *stubProvider) Analyze(
	_ context.Context,
	_ image.Image,
	_ string,
) (*VisionResult, error) {
	return &VisionResult{Provider: s.name}, nil
}

func (s *stubProvider) HealthCheck(_ context.Context) error { return nil }

func (s *stubProvider) GetCapabilities() ProviderCapabilities {
	return ProviderCapabilities{}
}

func (s *stubProvider) GetCostEstimate(_, _ int) float64 { return 0 }

// stubFactory returns a ProviderFactory that creates a stubProvider whose
// Name() returns name.
func stubFactory(name string) ProviderFactory {
	return func(_ map[string]interface{}) (VisionProvider, error) {
		return &stubProvider{name: name}, nil
	}
}

// TestRegistry_RegisterAndCreate verifies that a registered factory can be
// used to create a VisionProvider via Create.
func TestRegistry_RegisterAndCreate(t *testing.T) {
	r := NewRegistry()
	r.Register("stub", stubFactory("stub"))

	provider, err := r.Create("stub", nil)
	require.NoError(t, err)
	require.NotNil(t, provider)
	assert.Equal(t, "stub", provider.Name())
}

// TestRegistry_CreateUnknown verifies that Create returns an error for a name
// that has not been registered.
func TestRegistry_CreateUnknown(t *testing.T) {
	r := NewRegistry()

	provider, err := r.Create("nonexistent", nil)
	assert.Error(t, err)
	assert.Nil(t, provider)
}

// TestRegistry_List verifies that List returns all registered names.
func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register("alpha", stubFactory("alpha"))
	r.Register("beta", stubFactory("beta"))
	r.Register("gamma", stubFactory("gamma"))

	names := r.List()
	assert.ElementsMatch(t, []string{"alpha", "beta", "gamma"}, names)
}

// TestRegistry_IsRegistered verifies the IsRegistered helper.
func TestRegistry_IsRegistered(t *testing.T) {
	r := NewRegistry()
	r.Register("exists", stubFactory("exists"))

	assert.True(t, r.IsRegistered("exists"))
	assert.False(t, r.IsRegistered("missing"))
}

// TestRegistry_Unregister verifies that Unregister removes an entry so that
// Create and IsRegistered no longer see it.
func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	r.Register("temp", stubFactory("temp"))
	require.True(t, r.IsRegistered("temp"))

	r.Unregister("temp")

	assert.False(t, r.IsRegistered("temp"))

	provider, err := r.Create("temp", nil)
	assert.Error(t, err)
	assert.Nil(t, provider)
}

// TestRegistry_ConcurrentAccess exercises the registry under concurrent reads
// and writes to catch data races when run with -race.
func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	// Pre-register a provider that goroutines will read.
	r.Register("shared", stubFactory("shared"))

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := range goroutines {
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("concurrent-%d", i)
			r.Register(name, stubFactory(name))
			_ = r.IsRegistered(name)
			_ = r.List()
			_, _ = r.Create(name, nil)
			r.Unregister(name)
		}(i)
	}

	wg.Wait()

	// Only the pre-registered "shared" provider should remain.
	assert.True(t, r.IsRegistered("shared"))
}

// TestRegistry_DuplicateRegisterPanics verifies that registering the same name
// twice causes a panic.
func TestRegistry_DuplicateRegisterPanics(t *testing.T) {
	r := NewRegistry()
	r.Register("dup", stubFactory("dup"))

	assert.Panics(t, func() {
		r.Register("dup", stubFactory("dup"))
	})
}

// TestRegistry_NilFactoryPanics verifies that passing a nil factory to Register
// causes a panic.
func TestRegistry_NilFactoryPanics(t *testing.T) {
	r := NewRegistry()

	assert.Panics(t, func() {
		r.Register("nil-factory", nil)
	})
}
