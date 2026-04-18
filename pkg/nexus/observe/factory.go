// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package observe hosts the OCU P4 observation engine: pluggable Observer
// backends (ld_preload, plthook, dbus, cdp, ax_tree) behind a single
// factory. Every backend satisfies contracts.Observer; selection happens
// by string kind.
// Contracts are frozen in P0 — P4 only consumes them.
package observe

import (
	"context"
	"fmt"
	"sort"
	"sync"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// Config carries parameters shared across all Observer backends.
type Config struct {
	// BufferSize is the capacity of the internal ring buffer that stores
	// observed events. Zero defaults to 1024.
	BufferSize int
}

// Factory constructs an Observer for a given config. Each backend registers
// its factory via Register() in its own init().
type Factory func(ctx context.Context, cfg Config) (contracts.Observer, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register installs a Factory under the given kind. Later calls with the
// same kind replace the previous factory (useful for tests). Safe for
// concurrent use.
func Register(kind string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[kind] = f
}

// Open constructs an Observer of the requested kind. Returns an error if
// the kind has not been Register()-ed.
func Open(ctx context.Context, kind string, cfg Config) (contracts.Observer, error) {
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 1024
	}
	mu.RLock()
	f, ok := registry[kind]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("observe: unknown kind %q", kind)
	}
	return f(ctx, cfg)
}

// Kinds returns the sorted list of registered kinds.
func Kinds() []string {
	mu.RLock()
	defer mu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
