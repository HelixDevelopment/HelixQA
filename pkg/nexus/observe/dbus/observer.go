// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package dbus implements the OCU P4 Observer backend that subscribes to
// D-Bus signals on the session or system bus. P4 scope ships the plumbing
// (Observer struct, injectable producer, factory registration). Real gdbus
// subscription wiring arrives in P4.5.
// Note: D-Bus session bus access requires no root — it is available to
// every logged-in user via the DBUS_SESSION_BUS_ADDRESS environment variable.
package dbus

import (
	"context"
	"errors"
	"reflect"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/observe"
)

// ErrNotWired is returned by Start while the real D-Bus subscription
// wiring is still pending (P4.5 scope).
var ErrNotWired = errors.New("observe/dbus: production D-Bus producer not wired yet (P4.5)")

type producer interface {
	Produce(
		ctx context.Context,
		target contracts.Target,
		out chan<- contracts.Event,
		stopCh <-chan struct{},
	) error
}

type productionProducer struct{}

func (productionProducer) Produce(
	_ context.Context,
	_ contracts.Target,
	_ chan<- contracts.Event,
	_ <-chan struct{},
) error {
	return ErrNotWired
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
func (o *Observer) Start(ctx context.Context, target contracts.Target) error {
	if isProduction(o.prod) {
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
