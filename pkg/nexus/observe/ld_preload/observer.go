// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package ld_preload implements the OCU P4.5 Observer backend that hooks a
// target process via LD_PRELOAD. The shim is a shared library (.so) compiled
// by the operator from the C template at docs/hooks/ld-preload-shim.c. It
// writes newline-delimited JSON records to a named pipe (FIFO) whose path is
// passed via HELIXQA_LD_SHIM_FIFO; the Observer reads that FIFO and emits
// contracts.Event values with Kind == EventKindHook.
//
// Shim path resolution (first non-empty wins):
//  1. target.Labels["shim_path"]
//  2. HELIXQA_LD_SHIM environment variable
//
// If the resolved shim file does not exist on disk, Start returns
// ErrNotWired so the caller can degrade gracefully.
//
// Kill-switch: HELIXQA_OBSERVE_LDPRELOAD_STUB=1 forces ErrNotWired before
// any file-system or process access, useful for tests that must not spawn
// real processes.
//
// Note: LD_PRELOAD requires no root — the shim .so is placed in a
// user-writable directory and injected via the child process environment.
package ld_preload

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"time"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/observe"
)

// ErrNotWired is returned by Start when the shim is absent, the kill-switch
// is active, or the target binary cannot be launched.
var ErrNotWired = errors.New("observe/ld_preload: shim not found or stub active (P4.5)")

// shimLine is the JSON structure written by the C shim to the FIFO.
// See docs/hooks/ld-preload-shim.c for the matching C fprintf format.
type shimLine struct {
	TsNs int64  `json:"ts_ns"`
	Fn   string `json:"fn"`
	Arg  string `json:"arg"`
}

// ---------------------------------------------------------------------------
// producer interface (injectable for tests)
// ---------------------------------------------------------------------------

type producer interface {
	Produce(
		ctx context.Context,
		target contracts.Target,
		out chan<- contracts.Event,
		stopCh <-chan struct{},
	) error
}

// ---------------------------------------------------------------------------
// productionProducer
// ---------------------------------------------------------------------------

// productionProducer launches the target binary with LD_PRELOAD set and
// reads JSON Lines from the FIFO written by the shim.
type productionProducer struct{}

func (productionProducer) Produce(
	ctx context.Context,
	target contracts.Target,
	out chan<- contracts.Event,
	stopCh <-chan struct{},
) error {
	shimPath := resolveShimPath(target)
	if shimPath == "" {
		return ErrNotWired
	}
	if _, err := os.Stat(shimPath); err != nil {
		return ErrNotWired
	}

	// Create a temporary FIFO for shim→observer communication.
	fifoPath, err := makeFIFO()
	if err != nil {
		return fmt.Errorf("ld_preload: create FIFO: %w", err)
	}
	defer os.Remove(fifoPath) //nolint:errcheck

	// Resolve the target executable.
	execPath := target.Labels["exec_path"]
	if execPath == "" {
		execPath = target.ProcessName
	}
	if execPath == "" {
		return fmt.Errorf("ld_preload: target has no exec_path or ProcessName")
	}

	cmd := exec.CommandContext(ctx, execPath) //nolint:gosec
	cmd.Env = append(os.Environ(),
		"LD_PRELOAD="+shimPath,
		"HELIXQA_LD_SHIM_FIFO="+fifoPath,
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("ld_preload: start target %q: %w", execPath, err)
	}
	defer cmd.Wait() //nolint:errcheck

	// Open the FIFO for reading. This blocks until the shim opens it for
	// writing (which happens in the shim's ensure_out() on first emit).
	// A context-aware open is achieved by running it in a goroutine.
	type fifoResult struct {
		f   *os.File
		err error
	}
	fifoCh := make(chan fifoResult, 1)
	go func() {
		f, err := os.Open(fifoPath) //nolint:gosec
		fifoCh <- fifoResult{f, err}
	}()

	var fifo *os.File
	select {
	case res := <-fifoCh:
		if res.err != nil {
			return fmt.Errorf("ld_preload: open FIFO %q: %w", fifoPath, res.err)
		}
		fifo = res.f
	case <-ctx.Done():
		return nil
	case <-stopCh:
		return nil
	}
	defer fifo.Close() //nolint:errcheck

	scanner := bufio.NewScanner(fifo)
	for scanner.Scan() {
		line := scanner.Bytes()
		ev, err := parseShimLine(line)
		if err != nil {
			// Malformed line — skip and continue; the shim may emit a
			// partial line during startup.
			continue
		}
		select {
		case out <- ev:
		case <-stopCh:
			return nil
		case <-ctx.Done():
			return nil
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// resolveShimPath returns the shim .so path from Labels or environment.
func resolveShimPath(target contracts.Target) string {
	if p, ok := target.Labels["shim_path"]; ok && p != "" {
		return p
	}
	return os.Getenv("HELIXQA_LD_SHIM")
}

// makeFIFO creates a named pipe in os.TempDir and returns its path.
// The caller is responsible for removing it.
func makeFIFO() (string, error) {
	// Use a unique name derived from a temp file; remove the file first so
	// mkfifo can create the pipe node at the same path.
	f, err := os.CreateTemp("", "helix-ldshim-*.fifo")
	if err != nil {
		return "", err
	}
	path := f.Name()
	f.Close()
	os.Remove(path) //nolint:errcheck

	if err := mkfifo(path); err != nil {
		return "", fmt.Errorf("mkfifo %q: %w", path, err)
	}
	return path, nil
}

// parseShimLine decodes one JSON line written by the C shim into a
// contracts.Event. Exported for unit testing.
func parseShimLine(data []byte) (contracts.Event, error) {
	var sl shimLine
	if err := json.Unmarshal(data, &sl); err != nil {
		return contracts.Event{}, fmt.Errorf("ld_preload: parse shim line: %w", err)
	}
	return contracts.Event{
		Kind:      contracts.EventKindHook,
		Timestamp: time.Unix(0, sl.TsNs),
		Payload: map[string]any{
			"fn":  sl.Fn,
			"arg": sl.Arg,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Observer wiring
// ---------------------------------------------------------------------------

var productionProducerType = reflect.TypeOf(productionProducer{})

func isProduction(p producer) bool {
	return reflect.TypeOf(p) == productionProducerType
}

// newProducer is the package-level injectable; tests replace it.
var newProducer producer = productionProducer{}

func init() {
	observe.Register("ld_preload", Open)
}

// Open constructs an Observer. ErrNotWired surfaces at Start time, not Open time.
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
// Returns ErrNotWired when:
//   - HELIXQA_OBSERVE_LDPRELOAD_STUB=1 is set.
//   - The shim path cannot be resolved or the shim file does not exist.
func (o *Observer) Start(ctx context.Context, target contracts.Target) error {
	if stubActive() {
		return ErrNotWired
	}
	if isProduction(o.prod) {
		shimPath := resolveShimPath(target)
		if shimPath == "" {
			return ErrNotWired
		}
		if _, err := os.Stat(shimPath); err != nil {
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
// kill-switch
// ---------------------------------------------------------------------------

func stubActive() bool {
	return os.Getenv("HELIXQA_OBSERVE_LDPRELOAD_STUB") == "1"
}
