// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package encoder defines the Encoder interface that every codec backend in
// the OCU P5 recording layer must satisfy, together with the factory registry
// that maps codec kind strings to constructors.
//
// Production implementations (x264, nvenc, vaapi) are stubs in P5 — they
// all return ErrNotWired from Encode(). Real FFmpeg/NVENC CGO bindings land
// in P5.5. Tests inject a mock Encoder to exercise the Recorder without
// requiring a real codec.
package encoder

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// ErrNotWired is returned by production encoder stubs whose real CGO
// binding has not yet landed (FFmpeg/NVENC arrive in P5.5).
var ErrNotWired = errors.New("record/encoder: production codec not wired yet (P5.5)")

// Encoder is the interface every codec backend must satisfy.
type Encoder interface {
	// Encode accepts a single captured frame and encodes it into the
	// backend's output stream. Returns ErrNotWired when the production
	// binding is absent.
	Encode(frame contracts.Frame) error

	// Close flushes any pending output and releases all codec resources.
	// Must be called exactly once when recording ends.
	Close() error
}

// Factory constructs an Encoder for the given kind. Each sub-package
// registers its factory via Register() inside its init().
type Factory func() Encoder

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

// New constructs an Encoder of the requested kind. Returns an error if the
// kind has not been registered.
func New(kind string) (Encoder, error) {
	mu.RLock()
	f, ok := registry[kind]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("record/encoder: unknown kind %q", kind)
	}
	return f(), nil
}

// Kinds returns the sorted list of registered encoder kinds.
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
