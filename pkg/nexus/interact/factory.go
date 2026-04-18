// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package interact hosts the OCU P3 interaction engine: pluggable
// Interactor backends (linux/web/android/androidtv) behind a single
// factory. Every backend satisfies contracts.Interactor; selection
// happens by string kind.
// Contracts are frozen in P0 — P3 only consumes them.
package interact

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// Config carries parameters shared across all Interactor backends.
type Config struct {
	// DelayBetweenActions is an optional pause inserted after every
	// mutating action. Zero means no delay.
	DelayBetweenActions time.Duration

	// VerifyAfterAction, when true, signals the caller intends to wrap
	// the returned Interactor with verify.Wrap. The factory itself does
	// not perform verification — this field is informational for P3 scope.
	VerifyAfterAction bool
}

// Factory constructs an Interactor for a given config. Each backend
// registers its factory via Register() in its own init().
type Factory func(ctx context.Context, cfg Config) (contracts.Interactor, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register installs a Factory under the given kind. Later calls with
// the same kind replace the previous factory (useful for tests). Safe
// for concurrent use.
func Register(kind string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[kind] = f
}

// Open constructs an Interactor of the requested kind. Returns an error
// if the kind has not been Register()-ed.
func Open(ctx context.Context, kind string, cfg Config) (contracts.Interactor, error) {
	mu.RLock()
	f, ok := registry[kind]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("interact: unknown kind %q", kind)
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
