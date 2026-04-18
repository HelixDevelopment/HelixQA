// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package cdp implements the OCU P4 Observer backend that taps Chrome
// DevTools Protocol events (network, DOM, JS). P4 scope ships the
// plumbing (Observer struct, injectable producer, factory registration).
// Real CDP WebSocket subscription wiring arrives in P4.5.
// Note: CDP access requires no root — the browser exposes a local
// WebSocket endpoint on a user-controlled port.
package cdp

import (
	"context"
	"errors"
	"reflect"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/observe"
)

// ErrNotWired is returned by Start while the real CDP subscription
// wiring is still pending (P4.5 scope).
var ErrNotWired = errors.New("observe/cdp: production CDP producer not wired yet (P4.5)")

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
	observe.Register("cdp", Open)
}

// Open constructs an Observer. ErrNotWired surfaces at Start time, not Open time.
func Open(_ context.Context, cfg observe.Config) (contracts.Observer, error) {
	return &Observer{
		BaseObserver: observe.NewBase(cfg),
		prod:         newProducer,
	}, nil
}

// Observer is the Chrome DevTools Protocol event tap observer.
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
