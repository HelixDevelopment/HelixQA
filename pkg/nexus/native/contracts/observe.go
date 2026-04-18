// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package contracts

import (
	"context"
	"time"
)

// EventKind identifies the origin / transport of an observed event.
type EventKind string

const (
	// EventKindSyscall is a kernel-level syscall event (e.g. via eBPF / strace).
	EventKindSyscall EventKind = "syscall"

	// EventKindDBus is a D-Bus signal or method call captured on the session
	// or system bus.
	EventKindDBus EventKind = "dbus"

	// EventKindCDP is a Chrome DevTools Protocol event (network, DOM, JS).
	EventKindCDP EventKind = "cdp"

	// EventKindAXTree is an accessibility-tree mutation event.
	EventKindAXTree EventKind = "ax_tree"

	// EventKindHook is an event injected by a user-space hook or LD_PRELOAD.
	EventKindHook EventKind = "hook"
)

// Target identifies the process or resource to observe.
type Target struct {
	ProcessName string
	PID         int
	Labels      map[string]string
}

// Event is a single observation emitted by an Observer.
type Event struct {
	Kind      EventKind
	Timestamp time.Time
	Payload   map[string]interface{}
	Raw       []byte
}

// Observer watches a running process or system resource and emits Events.
type Observer interface {
	// Start begins observation of the given target.
	// The context controls the lifetime of the observation session.
	Start(ctx context.Context, target Target) error

	// Events returns a read-only channel of observed events.
	// The channel is closed when the observer stops.
	Events() <-chan Event

	// Snapshot returns all events that occurred within [at-window, at].
	// at is treated as the end of the window; window is its duration.
	Snapshot(at time.Time, window time.Duration) ([]Event, error)

	// Stop halts observation and flushes any buffered events.
	Stop() error
}
