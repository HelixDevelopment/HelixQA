// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package ld_preload implements the OCU P4 Observer backend that hooks a
// target process via LD_PRELOAD. P4 scope ships the plumbing (Observer
// struct, injectable producer, factory registration). Real .so shim
// compilation and install into a user-owned path arrive in P4.5.
// Note: LD_PRELOAD requires no root — the shim .so is placed in a
// user-writable directory and injected via the process environment.
package ld_preload

import (
	"context"
	"errors"
	"reflect"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/observe"
)

// ErrNotWired is returned by Start while the real LD_PRELOAD shim wiring
// is still pending (P4.5 scope).
var ErrNotWired = errors.New("observe/ld_preload: production shim producer not wired yet (P4.5)")

// producer is the injectable backend. Tests swap newProducer for a fake;
// production keeps productionProducer.
type producer interface {
	Produce(
		ctx context.Context,
		target contracts.Target,
		out chan<- contracts.Event,
		stopCh <-chan struct{},
	) error
}

// productionProducer is the not-yet-wired stub used in production.
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

// newProducer is the package-level injectable; tests replace it.
var newProducer producer = productionProducer{}

func init() {
	observe.Register("ld_preload", Open)
}

// Open constructs an Observer. The production path always succeeds at
// Open time; ErrNotWired surfaces when Start is called.
func Open(_ context.Context, cfg observe.Config) (contracts.Observer, error) {
	return &Observer{
		BaseObserver: observe.NewBase(cfg),
		prod:         newProducer,
	}, nil
}

// Observer is the LD_PRELOAD hook-based event observer.
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
