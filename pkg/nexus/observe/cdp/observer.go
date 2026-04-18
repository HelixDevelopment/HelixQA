// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package cdp implements the OCU P4.5 Observer backend that taps Chrome
// DevTools Protocol events via github.com/chromedp/chromedp.
//
// Subscribed events:
//   - network.EventResponseReceived  (URL + HTTP status)
//   - runtime.EventConsoleAPICalled  (console.log/warn/error)
//
// Kill-switch: HELIXQA_OBSERVE_CDP_STUB=1 forces ErrNotWired regardless of
// the environment, useful for tests that must not launch a real browser.
//
// Fallback: if chromium / google-chrome is not on PATH, ErrNotWired is
// returned so the caller can degrade gracefully.
//
// Note: CDP access requires no root — the browser exposes a local WebSocket
// endpoint on a user-controlled port.
package cdp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"

	contracts "digital.vasic.helixqa/pkg/nexus/native/contracts"
	"digital.vasic.helixqa/pkg/nexus/observe"
)

// ErrNotWired is returned by Start when chromium/chrome is absent, the
// HELIXQA_OBSERVE_CDP_STUB kill-switch is active, or the browser fails to
// launch.
var ErrNotWired = errors.New("observe/cdp: chromium/chrome unavailable or stub active")

// BrowserCandidates is the ordered list of executable names searched via
// exec.LookPath to decide whether a real browser is available.
// Exported so tests can override it without races (replace whole slice via
// the test's cleanup, always under t.Setenv-style serialisation).
var BrowserCandidates = []string{
	"chromium",
	"chromium-browser",
	"google-chrome",
	"google-chrome-stable",
	"chrome",
}

// resolveBrowser returns the first browser executable found on PATH, or an
// error when none is available.
func resolveBrowser() (string, error) {
	// Read BrowserCandidates once; tests that override it do so before
	// calling Start, so there is no concurrent mutation during the search.
	candidates := BrowserCandidates
	for _, name := range candidates {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("observe/cdp: no chromium/chrome binary found on PATH")
}

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

// productionProducer launches a real headless browser and subscribes to CDP.
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
	execPath, err := resolveBrowser()
	if err != nil {
		return ErrNotWired
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx,
		append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ExecPath(execPath),
			chromedp.Flag("headless", true),
			chromedp.Flag("no-sandbox", true),
			chromedp.Flag("disable-gpu", true),
		)...,
	)
	defer cancelAlloc()

	cdpCtx, cancelCDP := chromedp.NewContext(allocCtx)
	defer cancelCDP()

	// Subscribe to network + runtime events before navigating.
	chromedp.ListenTarget(cdpCtx, func(ev any) {
		var event contracts.Event
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			event = networkResponseToEvent(e)
		case *runtime.EventConsoleAPICalled:
			event = consoleAPICalledToEvent(e)
		default:
			return
		}
		select {
		case out <- event:
		case <-stopCh:
		case <-cdpCtx.Done():
		}
	})

	// Run the browser to establish the CDP connection; we don't navigate
	// anywhere — callers drive navigation via separate chromedp.Run calls.
	if err := chromedp.Run(cdpCtx); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("observe/cdp: browser run error: %w", err)
	}

	// Wait until external stop or context cancellation.
	select {
	case <-stopCh:
	case <-cdpCtx.Done():
	case <-ctx.Done():
	}
	return nil
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
// Returns ErrNotWired when the kill-switch is active or no browser is found.
func (o *Observer) Start(ctx context.Context, target contracts.Target) error {
	if isProduction(o.prod) {
		if stubActive() {
			return ErrNotWired
		}
		if _, err := resolveBrowser(); err != nil {
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
// pure translation helpers (no I/O — unit-testable)
// ---------------------------------------------------------------------------

func stubActive() bool {
	return os.Getenv("HELIXQA_OBSERVE_CDP_STUB") == "1"
}

// networkResponseToEvent converts a CDP Network.responseReceived event into a
// contracts.Event.
func networkResponseToEvent(e *network.EventResponseReceived) contracts.Event {
	payload := map[string]any{
		"url":    e.Response.URL,
		"status": e.Response.Status,
	}
	return contracts.Event{
		Kind:      contracts.EventKindCDP,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}

// consoleAPICalledToEvent converts a CDP Runtime.consoleAPICalled event into a
// contracts.Event.
func consoleAPICalledToEvent(e *runtime.EventConsoleAPICalled) contracts.Event {
	payload := map[string]any{
		"type": string(e.Type),
	}
	if len(e.Args) > 0 && e.Args[0] != nil {
		payload["value"] = e.Args[0].Value
	}
	return contracts.Event{
		Kind:      contracts.EventKindCDP,
		Timestamp: time.Now(),
		Payload:   payload,
	}
}
