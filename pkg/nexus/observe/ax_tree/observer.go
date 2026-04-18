// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package ax_tree implements the OCU P4.5 Observer backend that subscribes to
// the Linux AT-SPI2 accessibility tree via D-Bus.
//
// Architecture:
//  1. Query the session bus for the AT-SPI2 bus address via
//     org.a11y.Bus.GetAddress on the /org/a11y/bus object path.
//  2. Dial that address directly with dbus.Dial.
//  3. Subscribe to signals on org.a11y.atspi.Event.Object (focus,
//     state-changed, children-changed) and org.a11y.atspi.Event.Window.
//  4. Translate each signal into a contracts.Event{Kind: EventKindAXTree}.
//
// Kill-switch: HELIXQA_OBSERVE_AX_STUB=1 forces ErrNotWired regardless of
// the environment, useful for tests that must not touch a real bus.
//
// Fallback: if DBUS_SESSION_BUS_ADDRESS is unset, the session-bus call fails,
// the accessibility bus address is empty, or the AT-SPI2 bus dial fails,
// ErrNotWired is returned so the caller can degrade gracefully.
//
// Note: AT-SPI2 accessibility bus access requires no root — it is available
// to every logged-in desktop session user.
package ax_tree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"reflect"
	"time"

	dbus "github.com/godbus/dbus/v5"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/observe"
)

// ErrNotWired is returned by Start when the AT-SPI2 bus is unavailable or
// the HELIXQA_OBSERVE_AX_STUB kill-switch is active.
var ErrNotWired = errors.New("observe/ax_tree: AT-SPI2 accessibility bus unavailable or stub active")

// a11yBusInterface is the well-known D-Bus interface for the AT-SPI2 bus broker.
const (
	a11yBusName      = "org.a11y.Bus"
	a11yBusPath      = "/org/a11y/bus"
	a11yBusInterface = "org.a11y.Bus"
	a11yGetAddress   = "org.a11y.Bus.GetAddress"

	atspiEventObject = "org.a11y.atspi.Event.Object"
	atspiEventWindow = "org.a11y.atspi.Event.Window"
)

// ---------------------------------------------------------------------------
// internal producer interface (injectable for tests)
// ---------------------------------------------------------------------------

type producer interface {
	Produce(
		ctx context.Context,
		target contracts.Target,
		out chan<- contracts.Event,
		stopCh <-chan struct{},
	) error
}

// productionProducer connects to the real AT-SPI2 accessibility bus.
type productionProducer struct{}

func (productionProducer) Produce(
	ctx context.Context,
	_ contracts.Target,
	out chan<- contracts.Event,
	stopCh <-chan struct{},
) error {
	if stubActive() {
		return ErrNotWired
	}

	addr, err := resolveA11yAddress()
	if err != nil {
		return ErrNotWired
	}

	conn, err := dbus.Dial(addr)
	if err != nil {
		return ErrNotWired
	}
	defer conn.Close() //nolint:errcheck

	if err := conn.Auth(nil); err != nil {
		return ErrNotWired
	}
	if err := conn.Hello(); err != nil {
		return ErrNotWired
	}

	// Subscribe to Object and Window AT-SPI signals.
	for _, iface := range []string{atspiEventObject, atspiEventWindow} {
		if err := conn.AddMatchSignal(dbus.WithMatchInterface(iface)); err != nil {
			return fmt.Errorf("observe/ax_tree: AddMatchSignal(%s): %w", iface, err)
		}
	}

	sigCh := make(chan *dbus.Signal, 64)
	conn.Signal(sigCh)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-stopCh:
			return nil
		case sig, ok := <-sigCh:
			if !ok {
				return nil
			}
			ev := signalToAXEvent(sig)
			select {
			case out <- ev:
			case <-stopCh:
				return nil
			case <-ctx.Done():
				return nil
			}
		}
	}
}

var productionProducerType = reflect.TypeOf(productionProducer{})

func isProduction(p producer) bool {
	return reflect.TypeOf(p) == productionProducerType
}

var newProducer producer = productionProducer{}

func init() {
	observe.Register("ax_tree", Open)
}

// Open constructs an Observer. ErrNotWired surfaces at Start time, not Open time.
func Open(_ context.Context, cfg observe.Config) (contracts.Observer, error) {
	return &Observer{
		BaseObserver: observe.NewBase(cfg),
		prod:         newProducer,
	}, nil
}

// Observer is the AT-SPI2 accessibility-tree event observer.
type Observer struct {
	*observe.BaseObserver
	prod producer
}

// Start implements contracts.Observer.
// Returns ErrNotWired when the kill-switch is active, DBUS_SESSION_BUS_ADDRESS
// is unset, the a11y bus lookup fails, or the AT-SPI2 bus connection fails.
func (o *Observer) Start(ctx context.Context, target contracts.Target) error {
	if isProduction(o.prod) {
		if stubActive() || os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
			return ErrNotWired
		}
	}
	o.StartLoop(ctx, target, func(
		ctx context.Context,
		target contracts.Target,
		out chan<- contracts.Event,
		stopCh <-chan struct{},
	) error {
		return o.prod.Produce(ctx, target, out, stopCh)
	})
	return nil
}

// Stop implements contracts.Observer.
func (o *Observer) Stop() error {
	return o.BaseStop()
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func stubActive() bool {
	return os.Getenv("HELIXQA_OBSERVE_AX_STUB") == "1"
}

// resolveA11yAddress queries the session bus for the AT-SPI2 accessibility
// bus socket address via org.a11y.Bus.GetAddress.
// Returns an error when the session bus is unavailable or returns an empty
// address.
func resolveA11yAddress() (string, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return "", fmt.Errorf("observe/ax_tree: session bus unavailable: %w", err)
	}
	defer conn.Close() //nolint:errcheck

	obj := conn.Object(a11yBusName, dbus.ObjectPath(a11yBusPath))
	var addr string
	if err := obj.Call(a11yGetAddress, 0).Store(&addr); err != nil {
		return "", fmt.Errorf("observe/ax_tree: GetAddress call failed: %w", err)
	}
	if addr == "" {
		return "", fmt.Errorf("observe/ax_tree: accessibility bus address is empty")
	}
	return addr, nil
}

// signalToAXEvent converts a godbus Signal from the AT-SPI2 bus into a
// contracts.Event. This is a pure translation with no I/O — exported for
// unit testing.
func signalToAXEvent(sig *dbus.Signal) contracts.Event {
	payload := map[string]any{
		"sender": sig.Sender,
		"path":   string(sig.Path),
		"member": sig.Name,
	}
	if len(sig.Body) > 0 {
		payload["body"] = sig.Body
	}
	return contracts.Event{
		Kind:      contracts.EventKindAXTree,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}
