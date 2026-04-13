// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package cheaper

import (
	"fmt"
	"sort"
	"sync"
)

// Registry holds a collection of named ProviderFactory functions. Each caller
// creates its own Registry via NewRegistry — there is no package-level
// singleton. All methods are safe for concurrent use.
type Registry struct {
	factories map[string]ProviderFactory
	mu        sync.RWMutex
}

// NewRegistry returns an empty, ready-to-use Registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]ProviderFactory),
	}
}

// Register adds factory under name. It panics when factory is nil or when name
// has already been registered. Use Unregister first if you need to replace an
// existing entry (e.g. in tests).
func (r *Registry) Register(name string, factory ProviderFactory) {
	if factory == nil {
		panic(fmt.Sprintf("cheaper: Register called with nil factory for %q", name))
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.factories[name]; exists {
		panic(fmt.Sprintf("cheaper: provider %q is already registered", name))
	}

	r.factories[name] = factory
}

// Create looks up the factory registered under name, invokes it with config,
// and returns the resulting VisionProvider. It returns an error when name is
// not registered or when the factory itself returns an error.
func (r *Registry) Create(name string, config map[string]interface{}) (VisionProvider, error) {
	r.mu.RLock()
	factory, ok := r.factories[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("cheaper: no provider registered with name %q", name)
	}

	provider, err := factory(config)
	if err != nil {
		return nil, fmt.Errorf("cheaper: factory for %q failed: %w", name, err)
	}

	return provider, nil
}

// List returns a sorted slice of all registered provider names. The returned
// slice is a copy — callers may modify it freely.
func (r *Registry) List() []string {
	r.mu.RLock()
	names := make([]string, 0, len(r.factories))
	for name := range r.factories {
		names = append(names, name)
	}
	r.mu.RUnlock()

	sort.Strings(names)
	return names
}

// IsRegistered reports whether a provider with the given name has been
// registered.
func (r *Registry) IsRegistered(name string) bool {
	r.mu.RLock()
	_, ok := r.factories[name]
	r.mu.RUnlock()
	return ok
}

// Unregister removes the factory registered under name. It is a no-op when
// name is not present. Primarily intended for test teardown.
func (r *Registry) Unregister(name string) {
	r.mu.Lock()
	delete(r.factories, name)
	r.mu.Unlock()
}
