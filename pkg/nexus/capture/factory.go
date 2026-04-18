// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package capture hosts the OCU P1 capture engine: pluggable
// CaptureSource backends (web/linux/android/androidtv) behind a
// single factory. Every source satisfies
// contracts.CaptureSource; selection happens by string kind.
// Contracts are frozen in P0 — P1 only consumes them.
package capture

import (
	"context"
	"fmt"
	"sort"
	"sync"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// Factory constructs a CaptureSource for a given config. Each
// backend registers its factory via Register() in its own init().
type Factory func(ctx context.Context, cfg contracts.CaptureConfig) (contracts.CaptureSource, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register installs a Factory under the given kind. Later calls
// with the same kind replace the previous factory (useful for
// tests). Safe for concurrent use.
func Register(kind string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[kind] = f
}

// Open constructs a CaptureSource of the requested kind. Returns
// an error if the kind has not been Register()-ed.
func Open(ctx context.Context, kind string, cfg contracts.CaptureConfig) (contracts.CaptureSource, error) {
	mu.RLock()
	f, ok := registry[kind]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("capture: unknown kind %q", kind)
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
