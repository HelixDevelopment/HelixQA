// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package remote is the HelixQA-side adapter that maps
// contracts.Capability requests to a local or remote Worker via
// Containers/pkg/scheduler + /pkg/distribution. It deliberately
// stays thin: host discovery, GPU probing, and scoring all live in
// Containers.
package remote

import (
	"context"
	"fmt"

	cremote "digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/scheduler"
	"google.golang.org/protobuf/proto"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
)

// HostManager is the narrow subset of Containers.HostManager we
// need. Having our own interface lets tests inject a fake.
type HostManager interface {
	ProbeAll(ctx context.Context) (map[string]*cremote.HostResources, error)
}

// Dispatcher resolves a Capability to a Worker. P0 ships a minimal
// local fallback Worker + a stub remote Worker (real gRPC arrives
// in P2).
type Dispatcher struct {
	hm     HostManager
	opts   scheduler.Options
	scorer *scheduler.ResourceScorer
}

// NewDispatcher wires a host manager and scorer options.
func NewDispatcher(hm HostManager, opts scheduler.Options) *Dispatcher {
	return &Dispatcher{
		hm:     hm,
		opts:   opts,
		scorer: scheduler.NewResourceScorer(opts),
	}
}

// Resolve implements contracts.Dispatcher.
func (d *Dispatcher) Resolve(ctx context.Context, need contracts.Capability) (contracts.Worker, error) {
	if need.PreferLocal {
		return &localWorker{}, nil
	}

	req := capabilityToRequirement(need)
	hosts, err := d.hm.ProbeAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("dispatcher: probe hosts: %w", err)
	}

	bestHost := ""
	bestScore := 0.0
	for name, res := range hosts {
		if !d.scorer.CanFit(res, req) {
			continue
		}
		if !res.HasGPU() {
			continue
		}
		sc := d.scorer.Score(res, req)
		if sc > bestScore {
			bestScore = sc
			bestHost = name
		}
	}
	if bestHost == "" {
		return nil, fmt.Errorf("dispatcher: no host satisfies %s", need.Kind)
	}
	return &remoteWorker{host: bestHost}, nil
}

func capabilityToRequirement(need contracts.Capability) scheduler.ContainerRequirements {
	caps := []string{}
	switch need.Kind {
	case contracts.KindCUDAOpenCV, contracts.KindTensorRTOCR:
		caps = []string{"cuda"}
	case contracts.KindNVENC:
		caps = []string{"nvenc"}
	}
	return scheduler.ContainerRequirements{
		Name: "ocu-" + string(need.Kind),
		GPU: &scheduler.GPURequirement{
			Count:        1,
			MinVRAMMB:    need.MinVRAM,
			Vendor:       "nvidia",
			Capabilities: caps,
		},
	}
}

type localWorker struct{}

func (l *localWorker) Call(context.Context, proto.Message, proto.Message) error {
	return fmt.Errorf("localWorker: real local impl arrives in P2/P5")
}
func (l *localWorker) Close() error { return nil }

type remoteWorker struct {
	host string
}

func (r *remoteWorker) Call(context.Context, proto.Message, proto.Message) error {
	return fmt.Errorf("remoteWorker: gRPC transport arrives in P2")
}
func (r *remoteWorker) Close() error { return nil }
