// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package dbus implements the OCU P4.5 Observer backend that subscribes to
// D-Bus signals on the session bus via github.com/godbus/dbus/v5.
//
// Connection requires DBUS_SESSION_BUS_ADDRESS to be set in the environment
// (automatically present for every logged-in desktop session). When the
// variable is unset or the connection fails, ErrNotWired is returned so the
// caller can degrade gracefully.
//
// Kill-switch: HELIXQA_OBSERVE_DBUS_STUB=1 forces ErrNotWired regardless of
// the environment, useful for tests that must not touch a real bus.
//
// Note: D-Bus session-bus access requires no root — it is a per-user socket
// managed by the desktop session.
package dbus

import (
	"context"
	"errors"
	"os"
	"reflect"
	"time"

	dbus "github.com/godbus/dbus/v5"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/observe"
)

// ErrNotWired is returned by Start when the session bus is unavailable or
// the HELIXQA_OBSERVE_DBUS_STUB kill-switch is active.
var ErrNotWired = errors.New("observe/dbus: D-Bus session bus unavailable or stub active")

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

// productionProducer connects to the real session bus.
type productionProducer struct{}

func (productionProducer) Produce(
	ctx context.Context,
	target contracts.Target,
	out chan<- contracts.Event,
	stopCh <-chan struct{},
) error {
	if stubActive() {
		return ErrNotWired
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return ErrNotWired
	}
	defer conn.Close() //nolint:errcheck

	// Build match options. If target.Labels["interface"] is set, subscribe
	// only to that interface; otherwise receive all signals.
	var opts []dbus.MatchOption
	if iface, ok := target.Labels["interface"]; ok && iface != "" {
		opts = append(opts, dbus.WithMatchInterface(iface))
	}
	if err := conn.AddMatchSignal(opts...); err != nil {
		return ErrNotWired
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
			ev := signalToEvent(sig)
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
	observe.Register("dbus", Open)
}

// Open constructs an Observer. ErrNotWired surfaces at Start time, not Open time.
func Open(_ context.Context, cfg observe.Config) (contracts.Observer, error) {
	return &Observer{
		BaseObserver: observe.NewBase(cfg),
		prod:         newProducer,
	}, nil
}

// Observer is the D-Bus signal subscriber event observer.
type Observer struct {
	*observe.BaseObserver
	prod producer
}

// Start implements contracts.Observer.
// Returns ErrNotWired when the kill-switch is active, DBUS_SESSION_BUS_ADDRESS
// is unset, or the bus connection otherwise fails.
func (o *Observer) Start(ctx context.Context, target contracts.Target) error {
	if isProduction(o.prod) && (stubActive() || os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "") {
		return ErrNotWired
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
	return os.Getenv("HELIXQA_OBSERVE_DBUS_STUB") == "1"
}

// signalToEvent converts a godbus Signal into a contracts.Event.
// This is a pure translation with no I/O — exported for unit testing.
func signalToEvent(sig *dbus.Signal) contracts.Event {
	payload := map[string]any{
		"sender": sig.Sender,
		"path":   string(sig.Path),
		"name":   sig.Name,
	}
	if len(sig.Body) > 0 {
		payload["body"] = sig.Body
	}
	return contracts.Event{
		Kind:      contracts.EventKindDBus,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}
